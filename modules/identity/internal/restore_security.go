package identity

import (
	"context"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
)

// InvalidateRestoredSessionsInput 是恢复后会话失效参数。
type InvalidateRestoredSessionsInput struct {
	TargetDataEpoch platform.Identifier `json:"targetDataEpoch"`
}

// InvalidateRestoredSessionsResult 是恢复后会话失效回执。
type InvalidateRestoredSessionsResult struct {
	InvalidatedCount int                 `json:"invalidatedCount"`
	NewAuthEpoch     platform.Identifier `json:"newAuthEpoch"`
}

type restoreIdentityContextKey struct{}

// WithIdentityRestoreID 将恢复编排已验证的restoreId放入启动上下文。
// 输入schema未携带restoreId，因此禁止函数自行从dataEpoch猜测恢复身份。
func WithIdentityRestoreID(ctx context.Context, restoreID platform.Identifier) context.Context {
	return context.WithValue(ctx, restoreIdentityContextKey{}, restoreID)
}

func identityRestoreIDFromContext(ctx context.Context) (platform.Identifier, bool) {
	restoreID, ok := ctx.Value(restoreIdentityContextKey{}).(platform.Identifier)
	return restoreID, ok && platform.ValidIdentifier(restoreID)
}

type restoredSessionsDecision struct {
	RestoreID       platform.Identifier `json:"restoreId"`
	TargetDataEpoch platform.Identifier `json:"targetDataEpoch"`
	NewAuthEpoch    platform.Identifier `json:"newAuthEpoch"`
	Invalidated     int                 `json:"invalidatedCount"`
}

// InvalidateRestoredSessions 在恢复换入后撤销全部恢复数据中的会话。
// 决定先写入open日志，数据库提交后再写入带回执的done日志；任一阶段失败
// 都拒绝继续开放入口，避免把未知状态当作已安全。
func InvalidateRestoredSessions(ctx context.Context, op OperationContext, input InvalidateRestoredSessionsInput) (InvalidateRestoredSessionsResult, error) {
	if err := checkIdentityRepositoryContext(ctx, op); err != nil {
		return InvalidateRestoredSessionsResult{}, err
	}
	if !platform.ValidIdentifier(input.TargetDataEpoch) || input.TargetDataEpoch != op.DataEpoch {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "恢复目标代次不一致")
	}
	if op.CallerService != "identity" || op.Actor.Kind != "service" {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeForbidden, false, "恢复会话失效仅允许身份启动流程")
	}
	restoreID, ok := identityRestoreIDFromContext(ctx)
	if !ok {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复标识未由启动编排注入")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok || store.db == nil || store.journal == nil {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份恢复存储尚未装配")
	}

	decision, state, err := findRestoredSessionsDecision(store, restoreID)
	if err != nil {
		return InvalidateRestoredSessionsResult{}, err
	}
	if state == "done" {
		if decision.TargetDataEpoch != input.TargetDataEpoch || !platform.ValidIdentifier(decision.NewAuthEpoch) {
			return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复会话决定与目标代次不一致")
		}
		return InvalidateRestoredSessionsResult{InvalidatedCount: decision.Invalidated, NewAuthEpoch: decision.NewAuthEpoch}, nil
	}
	if state == "open" {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复会话决定未完成")
	}

	newAuthEpoch := platform.NewIdentifier()
	decision = restoredSessionsDecision{
		RestoreID:       restoreID,
		TargetDataEpoch: input.TargetDataEpoch,
		NewAuthEpoch:    newAuthEpoch,
	}
	decisionID := platform.NewIdentifier()
	openPayload, err := json.Marshal(decision)
	if err != nil {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复会话决定不可编码")
	}
	if err := appendJournalLine(store.journal, &store.jmu, journalEntry{
		ID: decisionID, Kind: "identity.invalidate-restored-sessions", State: "open", Payload: openPayload, At: platform.Now(),
	}); err != nil {
		return InvalidateRestoredSessionsResult{}, err
	}

	if err := ctx.Err(); err != nil {
		return InvalidateRestoredSessionsResult{}, identityContextError(err)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复会话事务不可用")
	}
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE revoked_at IS NULL`, platform.Now())
	if err != nil {
		_ = tx.Rollback()
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复会话失效写入失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count < 0 || count > 10000000 {
		_ = tx.Rollback()
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复会话失效计数不一致")
	}
	decision.Invalidated = int(count)
	if err := tx.Commit(); err != nil {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复会话事务提交失败")
	}

	donePayload, err := json.Marshal(decision)
	if err != nil {
		return InvalidateRestoredSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复会话回执不可编码")
	}
	if err := appendJournalLine(store.journal, &store.jmu, journalEntry{
		ID: decisionID, Kind: "identity.invalidate-restored-sessions", State: "done", Payload: donePayload, At: platform.Now(),
	}); err != nil {
		return InvalidateRestoredSessionsResult{}, err
	}
	return InvalidateRestoredSessionsResult{InvalidatedCount: decision.Invalidated, NewAuthEpoch: newAuthEpoch}, nil
}

func findRestoredSessionsDecision(store *Store, restoreID platform.Identifier) (restoredSessionsDecision, string, error) {
	entries, err := readJournal(store.journal)
	if err != nil {
		return restoredSessionsDecision{}, "", err
	}
	var found restoredSessionsDecision
	state := ""
	for _, entry := range entries {
		if entry.Kind != "identity.invalidate-restored-sessions" {
			continue
		}
		var candidate restoredSessionsDecision
		if err := json.Unmarshal(entry.Payload, &candidate); err != nil || candidate.RestoreID == "" {
			return restoredSessionsDecision{}, "", platform.NewError(platform.CodeIntegrityError, false, "恢复会话决定损坏")
		}
		if candidate.RestoreID != restoreID {
			continue
		}
		found, state = candidate, entry.State
	}
	if state == "" {
		return restoredSessionsDecision{}, "", nil
	}
	if state != "open" && state != "done" {
		return restoredSessionsDecision{}, "", platform.NewError(platform.CodeIntegrityError, false, "恢复会话决定状态不合法")
	}
	return found, state, nil
}

func identityContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "身份操作已超过期限")
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, true, "身份操作已取消")
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "身份操作上下文不可用")
}
