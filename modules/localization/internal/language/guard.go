package language

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/localization/internal/gen"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

const (
	putLanguageFunctionID        = "localization.PutLanguage"
	setLanguageEnabledFunctionID = "localization.SetLanguageEnabled"
	languageChangedEvent         = "localization.language.changed"
	// unsignedServiceSite 是公共入口在没有已核站点时写入的占位值。
	// 本域不把它当作真实站点，也不自行补签。
	unsignedServiceSite = "system"
	receiptTTL          = 7 * 24 * time.Hour
	maxLanguages        = 256
)


func requireActor(op OperationContext) error {
	if !platform.ValidIdentifier(op.Actor.SubjectID) {
		return platform.NewError(platform.CodeUnauthenticated, false, "操作主体缺失")
	}
	if op.Actor.SiteID == "" || op.Actor.SiteID == unsignedServiceSite || !platform.ValidIdentifier(op.Actor.SiteID) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "缺少已核本站")
	}
	return nil
}

func requireIdempotencyKey(key string) error {
	if !platform.ValidIdentifier(key) {
		return platform.NewError(platform.CodeValidation, false, "幂等键不合法")
	}
	return nil
}

// acceptStoredTag 只落实合同已经点名的长度和书写方向。
// 第一个连字符前的原文与 ar/he/fa/ur 逐字相同才是 rtl，其余标签是 ltr。
// 不折叠大小写，不删除脚本，不解释私用段，也不调用 NormalizeLanguageTag 冒充规范标签。
func acceptStoredTag(tag string,direction gen.LanguageDirection)error{
 normalized,err:=NormalizeLanguageTag(context.Background(),OperationContext{},NormalizeLanguageTagInput{Tag:tag});if err!=nil{return err}
 if string(direction)!=normalized.Direction{return platform.NewError(platform.CodeValidation,false,"语言方向与规范语言主子标签不一致")};return nil
}

func openLanguage(ctx context.Context, op OperationContext) (*sql.Tx, error) {
	if err := requireActor(op); err != nil {
		return nil, err
	}
	if !platform.ValidIdentifier(op.DataEpoch) {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "操作数据代次缺失")
	}
	store, runtimeClient := localizationruntime.LanguageGateway()
	if store == nil || store.DB() == nil || runtimeClient == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "语言配置存储或运行权威尚未装配")
	}
	view, err := runtimeClient.ReadRuntimeState(ctx)
	if err != nil {
		return nil, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "当前运行权威不可读")
	}
	if view.DataEpoch == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "当前数据代次未知")
	}
	if view.DataEpoch != op.DataEpoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "数据代次与当前站点不一致")
	}
	if view.Maintenance || view.Phase != platform.PhaseRunning {
		return nil, platform.NewError(platform.CodeLocked, false, "当前维护阶段不接受语言配置")
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, platform.NormalizeIOError(err, platform.CodeInternal, true, "开始语言配置事务失败")
	}
	var epoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton = 1`).Scan(&epoch); err != nil {
		_ = tx.Rollback()
		return nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "本域代次水位不可读")
	}
	if epoch != string(op.DataEpoch) {
		_ = tx.Rollback()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "本域代次水位与操作不一致")
	}
	return tx, nil
}

func rejectHeldBarrier(ctx context.Context, tx *sql.Tx) error {
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM local_barrier WHERE singleton = 1`).Scan(&state); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "本地维护屏障不可读")
	}
	if state != "released" {
		return platform.NewError(platform.CodeLocked, false, "本地维护屏障尚未释放")
	}
	return nil
}

func digestInput(value any) (platform.Hash, error) {
	raw, err := platform.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return platform.HashBytes(raw), nil
}

func readReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, functionID, key string, inputHash platform.Hash) (json.RawMessage, bool, error) {
	var storedHash, createdAt string
	var result []byte
	err := tx.QueryRowContext(ctx, `SELECT input_hash, result_json, created_at FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, functionID, key).Scan(&storedHash, &result, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言配置原回执不可读")
	}
	created, err := platform.ParseTimestamp(platform.Timestamp(createdAt))
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return nil, false, platform.NewError(platform.CodeInternal, false, "语言配置原回执时间损坏")
	}
	if now.Sub(created) > receiptTTL {
		return nil, false, nil
	}
	if storedHash != string(inputHash) {
		return nil, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同语言配置或代次")
	}
	if !json.Valid(result) {
		return nil, false, platform.NewError(platform.CodeInternal, false, "语言配置原回执损坏")
	}
	return result, true, nil
}

func writeReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, functionID, key string, inputHash platform.Hash, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "语言配置原回执编码失败")
	}
	now := string(platform.Now())
	if _, err := tx.ExecContext(ctx, `DELETE FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, functionID, key); err != nil {
		return mapWriteErr(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipts
		(site_id, actor_id, function_id, idempotency_key, data_epoch, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		op.Actor.SiteID, op.Actor.SubjectID, functionID, key, string(op.DataEpoch), string(inputHash), raw, now)
	return mapWriteErr(err)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLanguage(row rowScanner) (gen.Language, string, error) {
	var item gen.Language
	var epoch, direction string
	var enabled, isDefault, automatic, reflection int
	var profile sql.NullString
	err := row.Scan(&item.Tag, &item.Version, &enabled, &isDefault, &profile, &automatic, &direction,
		&reflection, &item.BatchOutputTokens, &item.TranslationMemoryMinimumCharacters, &epoch)
	if err != nil {
		return gen.Language{}, "", err
	}
	if (enabled != 0 && enabled != 1) || (isDefault != 0 && isDefault != 1) || (automatic != 0 && automatic != 1) || (reflection != 0 && reflection != 1) {
		return gen.Language{}, "", platform.NewError(platform.CodeInternal, false, "语言配置记录损坏")
	}
	if direction != string(gen.Ltr) && direction != string(gen.Rtl) {
		return gen.Language{}, "", platform.NewError(platform.CodeInternal, false, "语言方向记录损坏")
	}
	item.Enabled = enabled == 1
	item.IsDefault = isDefault == 1
	item.AutomaticTranslation = automatic == 1
	item.ReflectionEnabled = reflection == 1
	item.Direction = gen.LanguageDirection(direction)
	if profile.Valid {
		id := profile.String
		item.ProfileId = &id
	}
	return item, epoch, nil
}

func loadLanguage(ctx context.Context, tx *sql.Tx, tag string) (gen.Language, string, bool, error) {
	item, epoch, err := scanLanguage(tx.QueryRowContext(ctx, `SELECT language_tag, version, enabled, default_language,
		translation_profile, automatic_translation, direction, reflection_enabled, batch_output_tokens,
		translation_memory_minimum_characters, data_epoch
		FROM languages WHERE language_tag = ?`, tag))
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Language{}, "", false, nil
	}
	if err != nil {
		return gen.Language{}, "", false, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言配置不可读")
	}
	return item, epoch, true, nil
}

func profileValue(id *gen.Identifier) any {
	if id == nil {
		return nil
	}
	return string(*id)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func enqueueLanguageChanged(tx *sql.Tx, tag string, version int, enabled bool, epoch platform.Identifier) error {
	if !platform.ValidIdentifier(tag) {
		return platform.NewError(platform.CodeValidation, false, "语言标签不能作为事件主体")
	}
	payload, err := json.Marshal(struct {
		Tag       string `json:"tag"`
		Version   int    `json:"version"`
		Enabled   bool   `json:"enabled"`
		DataEpoch string `json:"dataEpoch"`
	}{tag, version, enabled, string(epoch)})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "语言变更事件编码失败")
	}
	return outbox.EnqueueEvent(tx, platform.EventEnvelope{
		EventID:      platform.NewIdentifier(),
		EventType:    languageChangedEvent,
		EventVersion: 1,
		Producer:     "localization",
		SubjectID:    tag,
		Revision:     platform.Revision(version),
		OccurredAt:   platform.Now(),
		DataEpoch:    epoch,
		Payload:      payload,
	})
}

func mapWriteErr(err error) error {
	if err == nil {
		return nil
	}
	var public *platform.Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, false, "语言配置操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "语言配置操作已到截止")
	}
	if strings.Contains(err.Error(), "constraint") {
		return platform.NewError(platform.CodeConflict, false, "语言配置与已有默认语言或主键冲突")
	}
	return platform.NewError(platform.CodeInternal, false, "语言配置写入失败")
}

func finishTx(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "提交语言配置失败")
	}
	return nil
}
