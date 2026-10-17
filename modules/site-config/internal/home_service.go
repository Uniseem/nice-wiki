package siteconfig

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/site-config/internal/gen"
)

type HomeMediaClient interface {
	ReadAsset(context.Context, string, ...url.Values) (json.RawMessage, error)
	AcquireResourceReferences(context.Context, any) (json.RawMessage, error)
	ConfirmResourceReferences(context.Context, any) (json.RawMessage, error)
	ReleaseResourceReferences(context.Context, any) (json.RawMessage, error)
}

type HomePublicLinkClient interface {
	ResolvePublishedLink(context.Context, any) (json.RawMessage, error)
}
type HomeQACapabilityClient interface {
	ReadPublicCapability(context.Context) (json.RawMessage, error)
}
type HomeRuntime struct {
	Media       HomeMediaClient
	PublicLinks HomePublicLinkClient
	QA          HomeQACapabilityClient
	Runtime     func(context.Context) (platform.RuntimeView, error)
	WriteWindow func(context.Context) (time.Time, error)
}

func (s *ActiveStore) ConfigureHome(runtime HomeRuntime) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.homeRuntime = runtime
}

func (s *ActiveStore) homeDependencies() HomeRuntime {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.homeRuntime
}

type homeResourceReference struct {
	SourceService    string              `json:"sourceService"`
	SourceObjectID   string              `json:"sourceObjectId"`
	SourceRevisionID string              `json:"sourceRevisionId"`
	SourceVersion    int                 `json:"sourceVersion"`
	ReferenceID      string              `json:"referenceId"`
	Asset            gen.AssetVersionRef `json:"asset"`
	Kind             string              `json:"kind"`
	Confirmed        bool                `json:"confirmed"`
	ExpiresAt        *time.Time          `json:"expiresAt"`
}

type homeReferencePin struct {
	PinID         string                `json:"pinId"`
	Assets        []gen.AssetVersionRef `json:"assets"`
	ExpiresAt     time.Time             `json:"expiresAt"`
	DeletionFence int                   `json:"deletionFence"`
}

type homeReferenceIntent struct {
	OperationID      string
	RootOperationID  string
	ActorID          string
	RequestHash      string
	Kind             string
	SiteID           string
	DataEpoch        string
	SourceObjectID   string
	SourceRevisionID string
	SourceVersion    int
	ReferenceDigest  string
	References       []homeResourceReference
	Decision         string
	Phase            string
	Pin              *homeReferencePin
	FailureCode      string
}

type ReadHomeReferenceDecisionInput struct {
	SourceOperationID string `json:"sourceOperationId"`
}
type ReadHomeReferenceDecisionResult struct {
	State             string `json:"state"`
	SiteID            string `json:"siteId"`
	SourceOperationID string `json:"sourceOperationId"`
	SourceObjectID    string `json:"sourceObjectId"`
	SourceRevisionID  string `json:"sourceRevisionId"`
	SourceVersion     int    `json:"sourceVersion"`
	ReferenceDigest   string `json:"referenceDigest"`
	DataEpoch         string `json:"dataEpoch"`
}

type RecoverHomeReferencesInput struct{}
type RecoverHomeReferencesResult struct {
	CompletedCount int `json:"completedCount"`
	PendingCount   int `json:"pendingCount"`
}

func homeSourceObjectID(siteID string) string {
	return platform.HashBytes([]byte("site-config.home.v1|" + siteID))
}

// referenceSourceObjectID 复用首页 pin 协议，只按种类区分来源对象，避免确认时互相覆盖。
func referenceSourceObjectID(kind, siteID string) string {
	switch kind {
	case "home":
		return homeSourceObjectID(siteID)
	case "branding":
		return platform.HashBytes([]byte("site-config.branding.v1|" + siteID))
	default:
		return ""
	}
}

func referenceMaterial(kind string, config gen.SiteConfig, revisionID string) ([]homeResourceReference, string, error) {
	switch kind {
	case "home":
		return homeReferenceMaterial(config, revisionID)
	case "branding":
		return brandingReferenceMaterial(config, revisionID)
	default:
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "资源引用候选种类不合法")
	}
}

func homeReferenceMaterial(config gen.SiteConfig, revisionID string) ([]homeResourceReference, string, error) {
	_, uses, err := validateHomeStructure(config.Home)
	if err != nil {
		return nil, "", err
	}
	references := make([]homeResourceReference, 0, len(uses))
	for _, use := range uses {
		references = append(references, homeResourceReference{SourceService: "site-config", SourceObjectID: homeSourceObjectID(config.SiteId),
			SourceRevisionID: revisionID, SourceVersion: config.Revision, ReferenceID: use.ReferenceID, Asset: use.Asset, Kind: "pending_pin"})
	}
	sort.Slice(references, func(i, j int) bool {
		left, right := references[i], references[j]
		if left.ReferenceID != right.ReferenceID {
			return left.ReferenceID < right.ReferenceID
		}
		if left.Asset.AssetId != right.Asset.AssetId {
			return left.Asset.AssetId < right.Asset.AssetId
		}
		if left.Asset.Version != right.Asset.Version {
			return left.Asset.Version < right.Asset.Version
		}
		return left.Asset.Sha256 < right.Asset.Sha256
	})
	raw, err := platform.CanonicalJSON(references)
	if err != nil {
		return nil, "", err
	}
	return references, platform.HashBytes(raw), nil
}

// saveHomeReferenceIntent 与候选材料同事务，先持久acquire intent才允许远端请求。
func saveHomeReferenceIntent(ctx context.Context, op platform.OperationContext, tx *sql.Tx, config gen.SiteConfig, revisionID string) error {
	return saveReferenceIntent(ctx, op, tx, "home", config, revisionID)
}

// saveReferenceIntent 把首页或品牌候选写入同一张决定表；来源对象按种类分开。
func saveReferenceIntent(ctx context.Context, op platform.OperationContext, tx *sql.Tx, kind string, config gen.SiteConfig, revisionID string) error {
	references, digest, err := referenceMaterial(kind, config, revisionID)
	if err != nil {
		return err
	}
	sourceID := referenceSourceObjectID(kind, config.SiteId)
	if sourceID == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "资源引用候选种类不合法")
	}
	raw, err := platform.CanonicalJSON(references)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO home_reference_decisions(operation_id,root_operation_id,site_id,data_epoch,source_object_id,source_revision_id,source_version,reference_digest,references_json,decision,phase,created_at)
        VALUES(?,?,?,?,?,?,?,?,?,'pending','acquire_pending',?)`, op.OperationID, op.RootOperationID, config.SiteId, op.DataEpoch,
		sourceID, revisionID, config.Revision, digest, string(raw), config.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		detail := "保存首页引用获取决定失败"
		if kind == "branding" {
			detail = "保存品牌引用获取决定失败"
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, detail)
	}
	return nil
}

func (s *ActiveStore) readHomeReferenceIntent(ctx context.Context, q ReadTransaction, operationID string) (homeReferenceIntent, error) {
	intent := homeReferenceIntent{OperationID: operationID}
	var references sql.NullString
	var pin, pinHash, failure, kind sql.NullString
	err := q.QueryRowContext(ctx, `SELECT h.root_operation_id,o.actor_id,o.request_hash,h.site_id,h.data_epoch,h.source_object_id,h.source_revision_id,h.source_version,h.reference_digest,
        CASE WHEN length(CAST(h.references_json AS BLOB))<=? THEN h.references_json ELSE NULL END,h.decision,h.phase,
        CASE WHEN h.pin_json IS NULL OR length(CAST(h.pin_json AS BLOB))<=? THEN h.pin_json ELSE NULL END,h.pin_hash,h.failure_code,m.kind
        FROM home_reference_decisions h JOIN config_operations o ON o.operation_id=h.operation_id LEFT JOIN config_operation_materials m ON m.operation_id=h.operation_id WHERE h.operation_id=?`, maxConfigFileBytes, maxConfigFileBytes, operationID).
		Scan(&intent.RootOperationID, &intent.ActorID, &intent.RequestHash, &intent.SiteID, &intent.DataEpoch, &intent.SourceObjectID, &intent.SourceRevisionID, &intent.SourceVersion,
			&intent.ReferenceDigest, &references, &intent.Decision, &intent.Phase, &pin, &pinHash, &failure, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		var candidateKind string
		readErr := q.QueryRowContext(ctx, `SELECT kind FROM config_operation_materials WHERE operation_id=?`, operationID).Scan(&candidateKind)
		if readErr != nil && !errors.Is(readErr, sql.ErrNoRows) {
			return intent, platform.NewError(platform.CodeDependencyUnavailable, true, "配置候选来源不可读")
		}
		switch candidateKind {
		case "home":
			return intent, platform.NewError(platform.CodeIntegrityError, false, "首页候选缺少持久引用决定")
		case "branding":
			return intent, platform.NewError(platform.CodeIntegrityError, false, "品牌候选缺少持久引用决定")
		}
		return intent, platform.NewError(platform.CodeNotFound, false, "首页引用来源操作不存在")
	}
	if err != nil {
		return intent, platform.NewError(platform.CodeDependencyUnavailable, true, "首页引用来源决定不可读")
	}
	intent.Kind = kind.String
	if (intent.Kind != "home" && intent.Kind != "branding") || !references.Valid || decodeConfigJSON([]byte(references.String), &intent.References) != nil || intent.References == nil {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "资源引用材料缺失或损坏")
	}
	raw, err := platform.CanonicalJSON(intent.References)
	expectedSource := referenceSourceObjectID(intent.Kind, intent.SiteID)
	if err != nil || expectedSource == "" || platform.HashBytes(raw) != intent.ReferenceDigest || intent.SourceObjectID != expectedSource ||
		intent.SourceRevisionID != configRevisionID(intent.DataEpoch, intent.SiteID, operationID) {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "资源引用材料摘要或来源不一致")
	}
	for _, reference := range intent.References {
		if reference.SourceService != "site-config" || reference.SourceObjectID != intent.SourceObjectID || reference.SourceRevisionID != intent.SourceRevisionID ||
			reference.SourceVersion != intent.SourceVersion || reference.Kind != "pending_pin" || reference.Confirmed || reference.ExpiresAt != nil {
			return intent, platform.NewError(platform.CodeIntegrityError, false, "首页引用完整来源字段不一致")
		}
	}
	if pin.Valid != pinHash.Valid {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "首页pin材料不完整")
	}
	if pin.Valid {
		var value homeReferencePin
		if platform.HashBytes([]byte(pin.String)) != pinHash.String || decodeConfigJSON([]byte(pin.String), &value) != nil || validateHomePin(intent, value) != nil {
			return intent, platform.NewError(platform.CodeIntegrityError, false, "首页pin材料损坏")
		}
		intent.Pin = &value
	}
	if failure.Valid {
		intent.FailureCode = failure.String
	}
	if !platform.ValidIdentifier(intent.RootOperationID) || !platform.ValidIdentifier(intent.ActorID) || !platform.ValidHash(intent.RequestHash) || !platform.ValidIdentifier(intent.SiteID) || !platform.ValidIdentifier(intent.DataEpoch) || intent.SourceVersion < 1 {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "首页引用持久来源身份损坏")
	}
	validPhase := false
	switch intent.Decision {
	case "pending":
		validPhase = intent.Phase == "acquire_pending" || intent.Phase == "pin_acquired"
	case "committed":
		validPhase = intent.Phase == "confirm_pending" || intent.Phase == "confirmed"
	case "aborted", "rejected":
		validPhase = intent.Phase == "release_pending" || intent.Phase == "released"
		if _, ok := platform.ProblemHTTPStatus[intent.FailureCode]; !ok {
			return intent, platform.NewError(platform.CodeIntegrityError, false, "首页终止回执损坏")
		}
	}
	if !validPhase || ((intent.Phase == "pin_acquired" || intent.Decision == "committed") && intent.Pin == nil) {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "首页引用持久阶段不完整")
	}
	return intent, nil
}

func validateHomePin(intent homeReferenceIntent, pin homeReferencePin) error {
	if !platform.ValidIdentifier(pin.PinID) || pin.ExpiresAt.IsZero() || pin.Assets == nil || len(pin.Assets) != len(intent.References) {
		return platform.NewError(platform.CodeIntegrityError, false, "媒体pin不对应完整首页引用")
	}
	assets := map[gen.AssetVersionRef]int{}
	for _, reference := range intent.References {
		assets[reference.Asset]++
	}
	for _, asset := range pin.Assets {
		if assets[asset] == 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "媒体pin资源版本不一致")
		}
		assets[asset]--
	}
	return nil
}

// committedHomeReference 复用集中IO边界，只将明确存在且核完整的COMMITTED当不可反悔决定。
func (s *ActiveStore) committedHomeReference(ctx context.Context, intent homeReferenceIntent) (bool, error) {
	paths, err := configJournalPaths(s.activeRoot, "COMMITTED")
	if err != nil {
		return false, err
	}
	path := configRecordPath(s.activeRoot, intent.SourceRevisionID, "COMMITTED")
	found := false
	for _, candidate := range paths {
		if candidate == path {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	return s.verifyHomeCommittedReference(ctx, intent)
}

func (s *ActiveStore) verifyHomeCommittedReference(ctx context.Context, intent homeReferenceIntent) (bool, error) {
	record, err := readConfigRecord(configRecordPath(s.activeRoot, intent.SourceRevisionID, "COMMITTED"))
	if err != nil {
		return false, err
	}
	if record.State != "COMMITTED" || record.OperationID != intent.OperationID || record.SiteID != intent.SiteID || record.DataEpoch != intent.DataEpoch ||
		record.ActorID != intent.ActorID || record.RequestHash != intent.RequestHash || record.RevisionID != intent.SourceRevisionID || record.Object.Version != intent.SourceVersion {
		return false, platform.NewError(platform.CodeIntegrityError, false, "首页COMMITTED与资源来源决定不一致")
	}
	config, err := readConfigDocument(s.activeRoot, record)
	if err != nil {
		return false, err
	}
	_, digest, err := referenceMaterial(intent.Kind, config, record.RevisionID)
	if err != nil || digest != intent.ReferenceDigest {
		return false, platform.NewError(platform.CodeIntegrityError, false, "首页COMMITTED引用摘要不一致")
	}
	return true, nil
}

func ReadHomeReferenceDecision(ctx context.Context, op platform.OperationContext, input ReadHomeReferenceDecisionInput) (ReadHomeReferenceDecisionResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeForbidden, false, "首页来源决定仅允许媒体纯服务读取")
	}
	s := currentActiveStore()
	if s == nil {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	var epoch string
	if err := s.db.QueryRowContext(ctx, `SELECT data_epoch FROM site_config_state WHERE singleton=1`).Scan(&epoch); err != nil {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "首页来源活动代次不可读")
	}
	if epoch != op.DataEpoch {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "首页来源不属于活动dataEpoch")
	}
	intent, err := s.readHomeReferenceIntent(ctx, s.db, input.SourceOperationID)
	if err != nil {
		return ReadHomeReferenceDecisionResult{}, err
	}
	if intent.DataEpoch != epoch {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "首页来源不属于活动dataEpoch")
	}
	committed, err := s.committedHomeReference(ctx, intent)
	if err != nil {
		return ReadHomeReferenceDecisionResult{}, err
	}
	state := intent.Decision
	if committed {
		state = "committed"
	} else if state == "committed" {
		return ReadHomeReferenceDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "首页来源缺少COMMITTED权威")
	}
	return ReadHomeReferenceDecisionResult{State: state, SiteID: intent.SiteID, SourceOperationID: intent.OperationID,
		SourceObjectID: intent.SourceObjectID, SourceRevisionID: intent.SourceRevisionID, SourceVersion: intent.SourceVersion, ReferenceDigest: intent.ReferenceDigest, DataEpoch: intent.DataEpoch}, nil
}

func (s *ActiveStore) checkHomeConfigCommit(ctx context.Context, tx *sql.Tx, record configJournalRecord, config gen.SiteConfig) error {
	intent, err := s.readHomeReferenceIntent(ctx, tx, record.OperationID)
	var failure *platform.Error
	if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	committed, err := s.committedHomeReference(ctx, intent)
	if err != nil {
		return err
	}
	subject := "首页"
	if intent.Kind == "branding" {
		subject = "品牌"
	}
	if !committed && (intent.Decision == "aborted" || intent.Decision == "rejected") {
		return platform.NewError(platform.CodeConflict, false, subject+"资源来源已终止，不能迟到提交")
	}
	_, digest, err := referenceMaterial(intent.Kind, config, record.RevisionID)
	if err != nil || intent.SourceRevisionID != record.RevisionID || intent.SourceVersion != record.Object.Version || intent.ReferenceDigest != digest || intent.Pin == nil {
		return platform.NewError(platform.CodeIntegrityError, false, subject+"提交缺少匹配的持久pin")
	}
	if !committed && !intent.Pin.ExpiresAt.After(time.Now()) {
		return platform.NewError(platform.CodeConflict, false, subject+"引用pin已到期，须用新操作保存")
	}
	return nil
}

func markHomeConfigCommitted(ctx context.Context, tx *sql.Tx, operationID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE home_reference_decisions SET decision='committed',phase=CASE WHEN phase='confirmed' THEN 'confirmed' ELSE 'confirm_pending' END WHERE operation_id=?`, operationID)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存首页引用确认阶段失败")
	}
	return nil
}

// homeReferenceContext 只为本域已登记的资源协议装配纯服务主体，不保存或复用用户proof。
func homeReferenceContext(ctx context.Context, op platform.OperationContext, intent homeReferenceIntent, phase string) (context.Context, platform.OperationContext) {
	op.CallerService = "site-config"
	op.RootOperationID = intent.RootOperationID
	op.OperationID = platform.HashBytes([]byte("site-config.home.references.v1|" + phase + "|" + intent.OperationID))
	return homeServiceContext(ctx, op, intent.SiteID)
}

// persistHomePin 远端成功但本域未落盘时，重放相同sourceOperationId找回同一pin。
func (s *ActiveStore) persistHomePin(ctx context.Context, op platform.OperationContext, intent homeReferenceIntent, pin homeReferencePin) error {
	if err := validateHomePin(intent, pin); err != nil {
		return err
	}
	work, current, cancel, err := s.homeWriteContext(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	ctx, op = work, current
	raw, err := platform.CanonicalJSON(pin)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE home_reference_decisions SET pin_json=?,pin_hash=?,phase=CASE WHEN phase IN ('confirmed','released') THEN phase WHEN decision='committed' THEN 'confirm_pending' WHEN decision='pending' THEN 'pin_acquired' ELSE 'release_pending' END
        WHERE operation_id=? AND data_epoch=? AND (pin_hash IS NULL OR pin_hash=?)`, string(raw), platform.HashBytes(raw), intent.OperationID, op.DataEpoch, platform.HashBytes(raw))
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存首页pin材料失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "首页pin材料与原操作冲突")
	}
	return nil
}

func (s *ActiveStore) acquireHomePin(ctx context.Context, op platform.OperationContext, intent homeReferenceIntent) (homeReferenceIntent, error) {
	if intent.Pin != nil {
		return intent, nil
	}
	client := s.homeDependencies().Media
	if client == nil {
		return intent, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体引用客户端未装配")
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return intent, err
	}
	request := struct {
		SiteID            string                  `json:"siteId"`
		SourceObjectID    string                  `json:"sourceObjectId"`
		SourceRevisionID  string                  `json:"sourceRevisionId"`
		SourceVersion     int                     `json:"sourceVersion"`
		ReferenceDigest   string                  `json:"referenceDigest"`
		SourceOperationID string                  `json:"sourceOperationId"`
		References        []homeResourceReference `json:"references"`
	}{intent.SiteID, intent.SourceObjectID, intent.SourceRevisionID, intent.SourceVersion, intent.ReferenceDigest, intent.OperationID, intent.References}
	work, current, cancel, err := s.homeWriteContext(ctx, op)
	if err != nil {
		return intent, err
	}
	defer cancel()
	remoteCtx, _ := homeReferenceContext(work, current, intent, "acquire")
	raw, err := client.AcquireResourceReferences(remoteCtx, request)
	if err != nil {
		return intent, err
	}
	var response struct {
		Pin homeReferencePin `json:"pin"`
	}
	if decodeConfigJSON(raw, &response) != nil || validateHomePin(intent, response.Pin) != nil {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "媒体获取pin响应不符合来源材料")
	}
	if err := s.persistHomePin(ctx, op, intent, response.Pin); err != nil {
		return intent, err
	}
	intent.Pin = &response.Pin
	return intent, nil
}

// decideHomeTermination 与CommitConfigHead共享锁顺序；无明确COMMITTED才允许持久终止。
func (s *ActiveStore) decideHomeTermination(ctx context.Context, op platform.OperationContext, operationID, code string) (homeReferenceIntent, error) {
	work, current, cancel, err := s.homeWriteContext(ctx, op)
	if err != nil {
		return homeReferenceIntent{}, err
	}
	defer cancel()
	ctx, op = work, current
	s.configMu.Lock()
	defer s.configMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return homeReferenceIntent{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开首页终局事务失败")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE site_config_state SET singleton=singleton WHERE singleton=1`); err != nil {
		return homeReferenceIntent{}, platform.NewError(platform.CodeDependencyUnavailable, true, "取得首页终局写锁失败")
	}
	intent, err := s.readHomeReferenceIntent(ctx, tx, operationID)
	if err != nil {
		return intent, err
	}
	committed, err := s.committedHomeReference(ctx, intent)
	if err != nil {
		return intent, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return intent, err
	}
	if committed {
		if err := markHomeConfigCommitted(ctx, tx, operationID); err != nil {
			return intent, err
		}
		intent.Decision, intent.Phase = "committed", "confirm_pending"
	} else if intent.Decision == "committed" {
		return intent, platform.NewError(platform.CodeIntegrityError, false, "首页已提交来源缺少文件决定")
	} else if intent.Decision == "pending" {
		_, err := tx.ExecContext(ctx, `UPDATE home_reference_decisions SET decision='aborted',phase='release_pending',failure_code=? WHERE operation_id=? AND decision='pending'`, code, operationID)
		if err != nil {
			return intent, platform.NewError(platform.CodeDependencyUnavailable, true, "保存首页终止决定失败")
		}
		intent.Decision, intent.Phase, intent.FailureCode = "aborted", "release_pending", code
	}
	if err := tx.Commit(); err != nil {
		return intent, platform.NewError(platform.CodeDependencyUnavailable, true, "提交首页终局决定失败")
	}
	return intent, nil
}

func (s *ActiveStore) settleHomeReference(ctx context.Context, op platform.OperationContext, intent homeReferenceIntent) error {
	if intent.Decision == "pending" {
		return platform.NewError(platform.CodeConflict, false, "首页引用尚未决定，不能确认或释放")
	}
	if intent.Decision == "committed" && intent.Pin == nil {
		return platform.NewError(platform.CodeIntegrityError, false, "已提交首页缺少持久pin材料")
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return err
	}
	client := s.homeDependencies().Media
	if client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "媒体引用客户端未装配")
	}
	var raw json.RawMessage
	phase := "confirmed"
	work, current, cancel, err := s.homeWriteContext(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	if intent.Decision == "committed" {
		remoteCtx, _ := homeReferenceContext(work, current, intent, "confirm")
		raw, err = client.ConfirmResourceReferences(remoteCtx, struct {
			PinID            string `json:"pinId"`
			SourceRevisionID string `json:"sourceRevisionId"`
			SourceVersion    int    `json:"sourceVersion"`
		}{intent.Pin.PinID, intent.SourceRevisionID, intent.SourceVersion})
		var result struct {
			Confirmed bool `json:"confirmed"`
		}
		if err == nil && (decodeConfigJSON(raw, &result) != nil || !result.Confirmed) {
			err = platform.NewError(platform.CodeIntegrityError, false, "媒体未确认已提交首页引用")
		}
	} else {
		phase = "released"
		reason := "source_aborted"
		if intent.Decision == "rejected" {
			reason = "source_rejected"
		}
		remoteCtx, _ := homeReferenceContext(work, current, intent, "release")
		var pinID *string
		if intent.Pin != nil {
			pinID = &intent.Pin.PinID
		}
		raw, err = client.ReleaseResourceReferences(remoteCtx, struct {
			SourceOperationID string  `json:"sourceOperationId"`
			SiteID            string  `json:"siteId"`
			SourceObjectID    string  `json:"sourceObjectId"`
			SourceRevisionID  string  `json:"sourceRevisionId"`
			SourceVersion     int     `json:"sourceVersion"`
			ReferenceDigest   string  `json:"referenceDigest"`
			PinID             *string `json:"pinId"`
			Reason            string  `json:"reason"`
		}{intent.OperationID, intent.SiteID, intent.SourceObjectID, intent.SourceRevisionID, intent.SourceVersion, intent.ReferenceDigest, pinID, reason})
		var result struct {
			Released bool `json:"released"`
		}
		if err == nil && (decodeConfigJSON(raw, &result) != nil || !result.Released) {
			err = platform.NewError(platform.CodeIntegrityError, false, "媒体未释放已终止首页pin")
		}
	}
	if err != nil {
		return err
	}
	confirmWork, confirmOp, confirmCancel, err := s.homeWriteContext(ctx, op)
	if err != nil {
		return err
	}
	defer confirmCancel()
	ctx, op = confirmWork, confirmOp
	result, err := s.db.ExecContext(ctx, `UPDATE home_reference_decisions SET phase=? WHERE operation_id=? AND data_epoch=? AND decision=?`, phase, intent.OperationID, intent.DataEpoch, intent.Decision)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存首页引用收敛回执失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "首页引用收敛水位已改变")
	}
	return nil
}

func RecoverHomeReferences(ctx context.Context, op platform.OperationContext, input RecoverHomeReferencesInput) (RecoverHomeReferencesResult, error) {
	if op.CallerService != "site-config" || op.Actor.Kind != "service" {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeForbidden, false, "首页引用恢复仅允许本域受控入口")
	}
	s := currentActiveStore()
	if s == nil {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return RecoverHomeReferencesResult{}, err
	}
	var epoch string
	if err := s.db.QueryRowContext(ctx, `SELECT data_epoch FROM site_config_state WHERE singleton=1`).Scan(&epoch); err != nil {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "首页活动代次不可读")
	}
	if epoch != op.DataEpoch {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "首页恢复不属于当前打开代次")
	}
	var missing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_operation_materials m JOIN config_operations o ON o.operation_id=m.operation_id LEFT JOIN home_reference_decisions h ON h.operation_id=m.operation_id WHERE m.kind IN ('home','branding') AND o.data_epoch=? AND h.operation_id IS NULL`, op.DataEpoch).Scan(&missing); err != nil || missing != 0 {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "首页或品牌操作缺少持久引用intent")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT operation_id FROM home_reference_decisions WHERE data_epoch=? AND phase NOT IN ('confirmed','released') ORDER BY source_version,operation_id LIMIT 100`, op.DataEpoch)
	if err != nil {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取首页待恢复引用失败")
	}
	operations := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "首页恢复条目损坏")
		}
		operations = append(operations, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return RecoverHomeReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "枚举首页待恢复引用失败")
	}
	result := RecoverHomeReferencesResult{}
	for _, id := range operations {
		intent, err := s.decideHomeTermination(ctx, op, id, platform.CodeConflict)
		if err != nil {
			return result, err
		}
		if err := s.settleHomeReference(ctx, op, intent); err != nil {
			var failure *platform.Error
			if errors.As(err, &failure) && (failure.Code == platform.CodeDependencyUnavailable || failure.Code == platform.CodeDeadlineExceeded || failure.Code == platform.CodeCancelled || failure.Code == platform.CodeLocked) {
				result.PendingCount++
				continue
			}
			return result, err
		}
		result.CompletedCount++
	}
	return result, nil
}

// compareHomeInput 比较原始候选字段，未完成重试不重造updatedAt或源版本。
func compareHomeInput(config gen.SiteConfig, input gen.UpdateHomeInput) bool {
	expected := input.Home
	expected.Revision = input.ExpectedHomeRevision + 1
	left, _ := platform.CanonicalJSON(expected)
	right, _ := platform.CanonicalJSON(config.Home)
	return config.Revision == input.ExpectedRevision+1 && bytes.Equal(left, right)
}

// UpdateHome 只将固定候选和pin保护后的hero写入既有settings权威提交链。
func UpdateHome(ctx context.Context, op platform.OperationContext, input gen.UpdateHomeInput) (gen.UpdateHomeResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateHomeResult{}, err
	}
	s := currentActiveStore()
	if s == nil {
		return gen.UpdateHomeResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	requestRaw, err := platform.CanonicalJSON(struct {
		Kind  string              `json:"kind"`
		Input gen.UpdateHomeInput `json:"input"`
	}{"home", input})
	if err != nil {
		return gen.UpdateHomeResult{}, err
	}
	requestHash := platform.HashBytes(requestRaw)
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.UpdateHomeResult{}, err
	}
	if receipt.State == "committed" {
		config, err := s.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.UpdateHomeResult{Config: config}, err
	}
	if receipt.State == "in_progress" {
		intent, err := s.readHomeReferenceIntent(ctx, s.db, op.OperationID)
		if err != nil {
			return gen.UpdateHomeResult{}, err
		}
		s.configMu.Lock()
		decided, decisionErr := s.committedHomeReference(ctx, intent)
		s.configMu.Unlock()
		if decisionErr != nil {
			return gen.UpdateHomeResult{}, decisionErr
		}
		if decided {
			// 已有不可反悔文件决定只补齐SQL/head，不重做当前可见性/ready或pin过期策略。
			if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
				return gen.UpdateHomeResult{}, err
			}
			record, err := readConfigRecord(configRecordPath(s.activeRoot, intent.SourceRevisionID, "COMMITTED"))
			if err != nil {
				return gen.UpdateHomeResult{}, err
			}
			completed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: record.Object, ExpectedRevision: record.ExpectedRevision, RequestHash: requestHash, ChangedSections: record.ChangedSections})
			return gen.UpdateHomeResult{Config: completed.Config}, err
		}
		if intent.Decision == "aborted" || intent.Decision == "rejected" {
			return gen.UpdateHomeResult{}, homeTerminatedError(intent)
		}
	}
	// 旧成功只读重放；其余候选IO/新pin/新提交先核写窗口。
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateHomeResult{}, err
	}
	if input.Home.Revision != input.ExpectedHomeRevision {
		return gen.UpdateHomeResult{}, platform.NewError(platform.CodeVersionMismatch, false, "首页材料revision与expectedHomeRevision不一致")
	}
	s.stateMu.RLock()
	localHTTP := s.allowLocalHTTP
	s.stateMu.RUnlock()
	var candidate configCandidate
	if receipt.State == "in_progress" {
		candidate, err = s.readConfigCandidate(ctx, s.db, op.OperationID, "home")
		if err == nil && !compareHomeInput(candidate.Config, input) {
			err = platform.NewError(platform.CodeIntegrityError, false, "首页候选与原始请求不一致")
		}
	} else {
		current, readErr := s.loadCurrentConfig(ctx, op)
		if readErr != nil {
			return gen.UpdateHomeResult{}, readErr
		}
		if current.Revision != input.ExpectedRevision || current.Home.Revision != input.ExpectedHomeRevision {
			return gen.UpdateHomeResult{}, platform.NewError(platform.CodeVersionMismatch, false, "首页或配置expectedRevision已变化")
		}
		config := current
		config.Home = input.Home
		config.Home.Revision = input.ExpectedHomeRevision + 1
		config.Revision, config.UpdatedAt = current.Revision+1, time.Now().UTC()
		if _, err := ValidateSiteConfig(ValidateSiteConfigInput{Config: config, AllowLocalHTTP: localHTTP}); err != nil {
			return gen.UpdateHomeResult{}, err
		}
		candidate = configCandidate{Config: config}
	}
	if err != nil {
		return gen.UpdateHomeResult{}, err
	}
	if _, err := ValidateHomeLinks(ctx, op, ValidateHomeLinksInput{Home: candidate.Config.Home, SiteID: candidate.Config.SiteId, Language: candidate.Config.BaseLanguage}); err != nil {
		return gen.UpdateHomeResult{}, err
	}
	if receipt.State != "in_progress" {
		candidate, err = s.reserveConfigCandidate(ctx, op, "home", requestHash, candidate.Config)
		if err != nil {
			return gen.UpdateHomeResult{}, err
		}
	}
	intent, err := s.readHomeReferenceIntent(ctx, s.db, op.OperationID)
	if err != nil {
		return gen.UpdateHomeResult{}, err
	}
	intent, err = s.acquireHomePin(ctx, op, intent)
	if err != nil {
		return gen.UpdateHomeResult{}, s.finishHomeFailure(ctx, op, err)
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.UpdateHomeResult{}, s.finishHomeFailure(ctx, op, err)
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash, ChangedSections: []string{"home"}})
	if err != nil {
		return gen.UpdateHomeResult{}, s.finishHomeFailure(ctx, op, err)
	}
	// confirm_pending已与正式水位/outbox原子持久化，由受信恢复入口持续收敛。
	return gen.UpdateHomeResult{Config: committed.Config}, nil
}

func homeTerminatedError(intent homeReferenceIntent) error {
	code := intent.FailureCode
	if _, ok := platform.ProblemHTTPStatus[code]; !ok {
		return platform.NewError(platform.CodeIntegrityError, false, "首页终局缺少有效失败回执")
	}
	subject := "首页"
	if intent.Kind == "branding" {
		subject = "品牌"
	}
	return platform.NewError(code, false, subject+"操作未提交且已终止，请使用新的操作保存")
}

func (s *ActiveStore) finishHomeFailure(ctx context.Context, op platform.OperationContext, cause error) error {
	code := platform.CodeConflict
	var failure *platform.Error
	if errors.As(cause, &failure) {
		code = failure.Code
	}
	intent, err := s.decideHomeTermination(ctx, op, op.OperationID, code)
	if err != nil {
		return err
	}
	if intent.Decision == "committed" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "首页已有不可反悔提交决定，须按原操作恢复权威水位")
	}
	// 释放是明确持久终止后的恢复阶段；网络未知不影响已记录终局，也不假报释放完成。
	return homeTerminatedError(intent)
}

func homeServiceContext(ctx context.Context, op platform.OperationContext, siteID string) (context.Context, platform.OperationContext) {
	op.CallerService = "site-config"
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "site-config", SiteID: siteID, Grants: []string{}, ExpiresAt: op.Deadline}
	ctx = platform.WithPortalProof(ctx, "")
	return platform.WithOperationContext(ctx, op), op
}

type PublicHeroButton struct {
	ButtonID       string                    `json:"buttonId"`
	Text           string                    `json:"text"`
	Style          gen.HeroButtonStyle       `json:"style"`
	Kind           string                    `json:"kind"`
	Disabled       bool                      `json:"disabled"`
	DisabledReason *string                   `json:"disabledReason"`
	Href           *string                   `json:"href"`
	Publication    *gen.PublishedRevisionRef `json:"publication"`
}

type PublicHome struct {
	MainTitle               string                  `json:"mainTitle"`
	Subtitle                string                  `json:"subtitle"`
	BackgroundText          string                  `json:"backgroundText"`
	TranslateBackgroundText bool                    `json:"translateBackgroundText"`
	Buttons                 []PublicHeroButton      `json:"buttons"`
	Background              gen.HeroBackground      `json:"background"`
	OverlayPercent          int                     `json:"overlayPercent"`
	TextColor               gen.HomeConfigTextColor `json:"textColor"`
	Revision                int                     `json:"revision"`
}

type PublicBranding struct {
	Logo              *gen.AssetVersionRef `json:"logo"`
	Favicon           *gen.AssetVersionRef `json:"favicon"`
	ThemeColor        string               `json:"themeColor"`
	DefaultShareImage *gen.AssetVersionRef `json:"defaultShareImage"`
}

type ReadPublicHomeInput struct {
	Language *string `json:"language,omitempty"`
}
type ReadPublicHomeResult struct {
	SiteName       string            `json:"siteName"`
	BaseLanguage   string            `json:"baseLanguage"`
	Language       string            `json:"language"`
	Home           PublicHome        `json:"home"`
	ConfigRevision int               `json:"configRevision"`
	Branding       PublicBranding    `json:"branding"`
	Footer         gen.FooterConfig  `json:"footer"`
	Banner         *gen.Announcement `json:"banner"`
}

type homeResolvedDocument struct {
	Visible          bool                      `json:"visible"`
	Href             *string                   `json:"href"`
	Publication      *gen.PublishedRevisionRef `json:"publication"`
	ResolvedLanguage *string                   `json:"resolvedLanguage"`
	Reason           string                    `json:"reason"`
}

func resolveHomeDocument(ctx context.Context, op platform.OperationContext, runtime HomeRuntime, siteID, language string, target homeLinkValue) (homeResolvedDocument, error) {
	if runtime.PublicLinks == nil {
		return homeResolvedDocument{}, platform.NewError(platform.CodeDependencyUnavailable, true, "公开链接权威未装配")
	}
	remoteCtx, _ := homeServiceContext(ctx, op, siteID)
	raw, err := runtime.PublicLinks.ResolvePublishedLink(remoteCtx, struct {
		DocumentID string  `json:"documentId"`
		Language   string  `json:"language"`
		BlockID    *string `json:"blockId"`
	}{target.DocumentID, language, target.BlockID})
	if err != nil {
		return homeResolvedDocument{}, err
	}
	var result homeResolvedDocument
	if decodeConfigJSON(raw, &result) != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "公开链接响应损坏")
	}
	if result.Visible {
		if result.Reason != "available" || result.Href == nil || !safeHomeRelativeHref(*result.Href, target.BlockID) || result.Publication == nil || result.Publication.DocumentId != target.DocumentID || result.Publication.DataEpoch != op.DataEpoch || !platform.ValidIdentifier(result.Publication.PublishedRevisionId) || !platform.ValidHash(result.Publication.ContentHash) || result.Publication.SchemaVersion < 1 || result.Publication.Revision < 1 || result.ResolvedLanguage == nil || *result.ResolvedLanguage != language {
			return result, platform.NewError(platform.CodeIntegrityError, false, "公开链接响应缺少匹配的当前引用")
		}
	} else if result.Href != nil || result.Publication != nil || result.ResolvedLanguage != nil || (result.Reason != "unavailable" && result.Reason != "anchor_missing" && result.Reason != "language_unavailable") {
		return result, platform.NewError(platform.CodeIntegrityError, false, "公开链接不可见结果不符")
	}
	return result, nil
}

func safeHomeRelativeHref(raw string, blockID *string) bool {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") || strings.IndexFunc(raw, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return false
	}
	value, err := url.Parse(raw)
	if err != nil || value.IsAbs() || value.Host != "" || value.User != nil || value.Opaque != "" || value.RawQuery != "" || value.ForceQuery {
		return false
	}
	if blockID == nil {
		return value.Fragment == "" && !strings.Contains(raw, "#")
	}
	return value.Fragment == *blockID
}

func disablePublicHomeButton(button *PublicHeroButton, reason string) {
	button.Disabled, button.DisabledReason, button.Href, button.Publication = true, &reason, nil, nil
}

// ReadPublicHome 明确投影公开字段；独立公开读取入口不改变既有私有配置授权。
func ReadPublicHome(ctx context.Context, op platform.OperationContext, input ReadPublicHomeInput) (ReadPublicHomeResult, error) {
	s := currentActiveStore()
	if s == nil {
		return ReadPublicHomeResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	config, err := s.loadPublicHomeConfig(ctx, op)
	if err != nil {
		return ReadPublicHomeResult{}, err
	}
	language := config.BaseLanguage
	if input.Language != nil {
		normalized, err := ValidateLanguageTag(ValidateLanguageTagInput{Language: *input.Language})
		if err != nil {
			return ReadPublicHomeResult{}, err
		}
		language = normalized.Canonical
	}
	home := config.Home
	public := PublicHome{MainTitle: home.MainTitle, Subtitle: home.Subtitle, BackgroundText: home.BackgroundText, TranslateBackgroundText: home.TranslateBackgroundText, Buttons: make([]PublicHeroButton, 0, len(home.Buttons)), Background: home.Background, OverlayPercent: home.OverlayPercent, TextColor: home.TextColor, Revision: home.Revision}
	runtime := s.homeDependencies()
	var qaAvailable bool
	qaReason, qaRead := "dependency_unavailable", false
	for _, button := range home.Buttons {
		target, err := decodeHomeTarget(button.Target)
		if err != nil {
			return ReadPublicHomeResult{}, err
		}
		projected := PublicHeroButton{ButtonID: button.ButtonId, Text: button.Text, Style: button.Style, Kind: target.Kind}
		switch target.Kind {
		case "document":
			resolved, err := resolveHomeDocument(ctx, op, runtime, config.SiteId, language, target)
			if err != nil {
				disablePublicHomeButton(&projected, "dependency_unavailable")
			} else if !resolved.Visible {
				reason := "target_unavailable"
				if resolved.Reason == "anchor_missing" || resolved.Reason == "language_unavailable" {
					reason = resolved.Reason
				}
				disablePublicHomeButton(&projected, reason)
			} else {
				projected.Href, projected.Publication = resolved.Href, resolved.Publication
			}
		case "qa":
			if !qaRead {
				qaAvailable, qaReason = readHomeQACapability(ctx, op, runtime, config.SiteId)
				qaRead = true
			}
			if !qaAvailable {
				disablePublicHomeButton(&projected, qaReason)
			} else {
				base, err := url.Parse(config.PublicBaseURL)
				if err != nil || !base.IsAbs() || base.Host == "" || strings.HasPrefix(base.EscapedPath(), "//") {
					return ReadPublicHomeResult{}, platform.NewError(platform.CodeIntegrityError, false, "公开首页基址损坏或存在歧义路径")
				}
				href := strings.TrimRight(base.EscapedPath(), "/") + "/" + url.PathEscape(language) + "/ask"
				projected.Href = &href
			}
		case "external":
			if err := validateConfigWebLink(target.URL, true); err != nil {
				return ReadPublicHomeResult{}, err
			}
			projected.Href = &target.URL
		}
		public.Buttons = append(public.Buttons, projected)
	}
	banner := config.Banner
	now := time.Now().UTC()
	if banner != nil && (!banner.Enabled || (banner.StartsAt != nil && now.Before(*banner.StartsAt)) || (banner.EndsAt != nil && !now.Before(*banner.EndsAt))) {
		banner = nil
	}
	return ReadPublicHomeResult{SiteName: config.Name, BaseLanguage: config.BaseLanguage, Language: language, Home: public, ConfigRevision: config.Revision, Branding: PublicBranding{Logo: config.Logo, Favicon: config.Favicon, ThemeColor: config.ThemeColor, DefaultShareImage: config.DefaultShareImage}, Footer: config.Footer, Banner: banner}, nil
}

func readHomeQACapability(ctx context.Context, op platform.OperationContext, runtime HomeRuntime, siteID string) (bool, string) {
	if runtime.QA == nil {
		return false, "dependency_unavailable"
	}
	remoteCtx, _ := homeServiceContext(ctx, op, siteID)
	raw, err := runtime.QA.ReadPublicCapability(remoteCtx)
	if err != nil {
		return false, "dependency_unavailable"
	}
	var result struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}
	if decodeConfigJSON(raw, &result) != nil {
		return false, "dependency_unavailable"
	}
	if result.Available && result.Reason == "available" {
		return true, ""
	}
	if !result.Available && result.Reason == "not_configured" {
		return false, "ai_not_configured"
	}
	return false, "dependency_unavailable"
}

// RecoverHomeReferenceState 启动只核本地材料；远端确认/释放必须等HTTP与公共后台生命周期启动。
func (s *ActiveStore) RecoverHomeReferenceState(ctx context.Context, op platform.OperationContext) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	s.configMu.Lock()
	defer s.configMu.Unlock()
	var epoch string
	if err := s.db.QueryRowContext(ctx, `SELECT data_epoch FROM site_config_state WHERE singleton=1`).Scan(&epoch); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "首页恢复活动代次不可读")
	}
	if epoch != op.DataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "首页恢复不属于已打开活动代次")
	}
	var missing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_operation_materials m JOIN config_operations o ON o.operation_id=m.operation_id LEFT JOIN home_reference_decisions h ON h.operation_id=m.operation_id WHERE m.kind IN ('home','branding') AND o.data_epoch=? AND h.operation_id IS NULL`, epoch).Scan(&missing); err != nil || missing != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "首页或品牌操作缺少持久引用intent")
	}
	paths, err := configJournalPaths(s.activeRoot, "COMMITTED")
	if err != nil {
		return err
	}
	committed := map[string]bool{}
	for _, path := range paths {
		committed[path] = true
	}
	rows, err := s.db.QueryContext(ctx, `SELECT operation_id FROM home_reference_decisions ORDER BY source_version,operation_id`)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "枚举首页引用决定失败")
	}
	operations := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "首页引用决定损坏")
		}
		operations = append(operations, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "枚举首页引用决定失败")
	}
	for _, id := range operations {
		intent, err := s.readHomeReferenceIntent(ctx, s.db, id)
		if err != nil {
			return err
		}
		if intent.DataEpoch != epoch {
			return platform.NewError(platform.CodeVersionMismatch, false, "首页引用决定不属于活动代次")
		}
		if committed[configRecordPath(s.activeRoot, intent.SourceRevisionID, "COMMITTED")] {
			if _, err := s.verifyHomeCommittedReference(ctx, intent); err != nil {
				return err
			}
		} else if intent.Decision == "committed" {
			return platform.NewError(platform.CodeIntegrityError, false, "首页提交决定缺少权威文件")
		}
	}
	s.stateMu.Lock()
	s.homeRecovered = true
	s.stateMu.Unlock()
	return nil
}

// homeWriteContext 复用公共有界写窗口；本域只核对当前打开epoch，不自造截止机制。
func (s *ActiveStore) homeWriteContext(ctx context.Context, op platform.OperationContext) (context.Context, platform.OperationContext, context.CancelFunc, error) {
	runtime := s.homeDependencies()
	if runtime.Runtime == nil || runtime.WriteWindow == nil {
		return nil, op, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "首页安全写窗口未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), platform.WriteWindowConfig{Runtime: runtime.Runtime, WriteWindow: runtime.WriteWindow})
	if err != nil {
		return nil, op, nil, err
	}
	var openedEpoch string
	if err := s.db.QueryRowContext(work, `SELECT data_epoch FROM site_config_state WHERE singleton=1`).Scan(&openedEpoch); err != nil {
		cancel()
		return nil, op, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "首页安全窗口的活动代次不可读")
	}
	if view.DataEpoch != op.DataEpoch || view.DataEpoch != openedEpoch {
		cancel()
		return nil, op, nil, platform.NewError(platform.CodeVersionMismatch, false, "首页安全窗口不属于已打开代次")
	}
	current, _ := platform.OperationContextFrom(work)
	return work, current, cancel, nil
}
