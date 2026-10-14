package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

// ValidateRuntimeTransition 校验 RuntimePhase 的唯一迁移矩阵；它不写控制库。
func ValidateRuntimeTransition(input gen.ValidateRuntimeTransitionInput) (gen.ValidateRuntimeTransitionResult, error) {
	if err := validateRuntimeStateShape(input.Current); err != nil {
		return gen.ValidateRuntimeTransitionResult{}, err
	}
	if input.Command.ExpectedRevision != input.Current.Revision {
		return gen.ValidateRuntimeTransitionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态revision已变化")
	}
	if err := validateTransitionFields(input.Current, input.Command); err != nil {
		return gen.ValidateRuntimeTransitionResult{}, err
	}
	return gen.ValidateRuntimeTransitionResult{Valid: true}, nil
}

// TransitionRuntimeState 按 expectedRevision 原子迁移控制库状态。
func TransitionRuntimeState(ctx context.Context, op platform.OperationContext, input gen.RuntimeTransition) (gen.TransitionRuntimeStateResult, error) {
	if err := checkOperationDeadline(ctx, op); err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	if op.CallerService != "data-protection" {
		return gen.TransitionRuntimeStateResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可迁移运行状态")
	}
	store := activeRuntimeStore()
	if store == nil {
		return gen.TransitionRuntimeStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	inputHash, err := hashInput(input)
	if err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.TransitionRuntimeStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开运行状态事务失败")
	}
	defer tx.Rollback()
	current, err := store.readStateFrom(ctx, tx)
	if err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	if stored, found, err := store.readOperation(ctx, tx, op.OperationID, "transition", inputHash); err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	} else if found {
		return gen.TransitionRuntimeStateResult{State: stored}, nil
	}
	if op.DataEpoch != "" && string(current.DataEpoch) != string(op.DataEpoch) {
		return gen.TransitionRuntimeStateResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态dataEpoch已变化")
	}
	if _, err := ValidateRuntimeTransition(gen.ValidateRuntimeTransitionInput{Current: current, Command: input}); err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	next, err := applyTransition(current, input)
	if err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	if err := updateRuntimeState(ctx, tx, current, next); err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	if err := store.writeOperation(ctx, tx, op.OperationID, "transition", inputHash, next); err != nil {
		return gen.TransitionRuntimeStateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return gen.TransitionRuntimeStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交运行状态事务失败")
	}
	return gen.TransitionRuntimeStateResult{State: next}, nil
}

// RegisterActivation 以 (restoreId,module) 唯一键登记域回执，并用 revision
// CAS 串行化并发域回执。
func RegisterActivation(ctx context.Context, op platform.OperationContext, input gen.RegisterActivationInput) (gen.RegisterActivationResult, error) {
	if err := checkOperationDeadline(ctx, op); err != nil {
		return gen.RegisterActivationResult{}, err
	}
	if op.CallerService == "" || op.CallerService != string(input.Receipt.Module) {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeForbidden, false, "服务身份与激活回执module不一致")
	}
	store := activeRuntimeStore()
	if store == nil {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	inputHash, err := hashInput(input)
	if err != nil {
		return gen.RegisterActivationResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开激活回执事务失败")
	}
	defer tx.Rollback()
	current, err := store.readStateFrom(ctx, tx)
	if err != nil {
		return gen.RegisterActivationResult{}, err
	}
	if stored, found, err := store.readOperation(ctx, tx, op.OperationID, "activation", inputHash); err != nil {
		return gen.RegisterActivationResult{}, err
	} else if found {
		return gen.RegisterActivationResult{State: stored}, nil
	}
	if op.DataEpoch != "" && string(current.DataEpoch) != string(op.DataEpoch) {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态dataEpoch已变化")
	}
	if input.ExpectedRevision != current.Revision {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态revision已变化")
	}
	if err := validateActivationInput(current, input); err != nil {
		return gen.RegisterActivationResult{}, err
	}
	next := current
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	next.ActivationReceipts = append([]gen.ActivationReceipt(nil), current.ActivationReceipts...)
	next.ActivationReceipts = appendOrReplaceReceipt(next.ActivationReceipts, input.Receipt)
	if err := upsertActivationReceipt(ctx, tx, input.RestoreId, input.Receipt); err != nil {
		return gen.RegisterActivationResult{}, err
	}
	if err := updateRuntimeState(ctx, tx, current, next); err != nil {
		return gen.RegisterActivationResult{}, err
	}
	if err := store.writeOperation(ctx, tx, op.OperationID, "activation", inputHash, next); err != nil {
		return gen.RegisterActivationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return gen.RegisterActivationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交激活回执事务失败")
	}
	return gen.RegisterActivationResult{State: next}, nil
}

func validateTransitionFields(current gen.RuntimeState, command gen.RuntimeTransition) error {
	if command.BarrierId == "" {
		return platform.NewError(platform.CodeLocked, false, "迁移缺少barrierId")
	}
	if current.Phase == gen.Running && command.Action == gen.BeginBackup {
		if command.RestoreId != "" || command.TargetDataEpoch != "" {
			return platform.NewError(platform.CodeIntegrityError, false, "备份迁移包含恢复字段")
		}
		return nil
	}
	if current.Phase == gen.BackupBarrier && command.Action == gen.CompleteBackup {
		if command.RestoreId != "" || command.TargetDataEpoch != "" || command.BarrierId != current.BarrierId {
			return platform.NewError(platform.CodeLocked, false, "备份屏障参数不一致")
		}
		return nil
	}
	if current.Phase == gen.Running && command.Action == gen.BeginRestore {
		if !platform.ValidIdentifier(platform.Identifier(command.RestoreId)) || !platform.ValidIdentifier(platform.Identifier(command.TargetDataEpoch)) || command.TargetDataEpoch == current.DataEpoch {
			return platform.NewError(platform.CodeValidation, false, "恢复迁移参数不合法")
		}
		return nil
	}
	if current.Phase == gen.RestoreStaging && (command.Action == gen.BeginSwitch || command.Action == gen.AbortRestore || command.Action == gen.MarkBlocked) {
		return validateRestoreCommand(current, command)
	}
	if current.Phase == gen.RestoreSwitching && (command.Action == gen.CompleteRestore || command.Action == gen.BeginRollback || command.Action == gen.MarkBlocked) {
		if command.Action == gen.CompleteRestore && !allReceiptsInState(current, gen.Switched) {
			return platform.NewError(platform.CodeLocked, false, "全部有状态域switched回执未齐")
		}
		return validateRestoreCommand(current, command)
	}
	if current.Phase == gen.RestoreBlocked && (command.Action == gen.BeginRollback || command.Action == gen.BeginSwitch) {
		return validateRestoreCommand(current, command)
	}
	if current.Phase == gen.RestoreRollingBack && command.Action == gen.CompleteRollback {
		if !allReceiptsInState(current, gen.RolledBack) {
			return platform.NewError(platform.CodeLocked, false, "已换入域rolled-back回执未齐")
		}
		return validateRestoreCommand(current, command)
	}
	return platform.NewError(platform.CodeLocked, false, "不允许的RuntimePhase迁移")
}

func validateRestoreCommand(current gen.RuntimeState, command gen.RuntimeTransition) error {
	if command.RestoreId == "" || command.RestoreId != current.RestoreId {
		return platform.NewError(platform.CodeVersionMismatch, false, "restoreId已变化")
	}
	if command.TargetDataEpoch == "" || command.TargetDataEpoch != current.TargetDataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "targetDataEpoch已变化")
	}
	if command.BarrierId == "" || command.BarrierId != current.BarrierId {
		return platform.NewError(platform.CodeVersionMismatch, false, "barrierId已变化")
	}
	return nil
}

func updateRuntimeState(ctx context.Context, tx *sql.Tx, current, next gen.RuntimeState) error {
	requiredJSON, err := json.Marshal(next.RequiredModules)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码requiredModules失败")
	}
	nullable := func(value string) any {
		if value == "" {
			return nil
		}
		return value
	}
	result, err := tx.ExecContext(ctx, `
        UPDATE runtime_state
           SET data_epoch = ?, target_data_epoch = ?, phase = ?, maintenance = ?,
               restore_id = ?, barrier_id = ?, revision = ?, required_modules = ?, updated_at = ?
         WHERE singleton = 1 AND revision = ?`,
		string(next.DataEpoch), nullable(string(next.TargetDataEpoch)), string(next.Phase), boolInt(next.Maintenance),
		nullable(string(next.RestoreId)), nullable(string(next.BarrierId)), int64(next.Revision), string(requiredJSON),
		next.UpdatedAt.UTC().Format(time.RFC3339Nano), int64(current.Revision))
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "写入运行状态失败")
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "运行状态revision已变化")
	}
	return nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func applyTransition(current gen.RuntimeState, command gen.RuntimeTransition) (gen.RuntimeState, error) {
	next := current
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	switch command.Action {
	case gen.BeginBackup:
		next.Phase, next.Maintenance, next.BarrierId = gen.BackupBarrier, true, command.BarrierId
	case gen.CompleteBackup, gen.AbortRestore:
		next.Phase, next.Maintenance = gen.Running, false
		next.TargetDataEpoch, next.RestoreId, next.BarrierId = "", "", ""
	case gen.CompleteRestore:
		next.DataEpoch = current.TargetDataEpoch
		next.Phase, next.Maintenance = gen.Running, false
		next.TargetDataEpoch, next.RestoreId, next.BarrierId = "", "", ""
	case gen.CompleteRollback:
		next.Phase, next.Maintenance = gen.Running, false
		next.TargetDataEpoch, next.RestoreId, next.BarrierId = "", "", ""
	case gen.BeginRestore:
		next.Phase, next.Maintenance = gen.RestoreStaging, true
		next.RestoreId, next.TargetDataEpoch, next.BarrierId = command.RestoreId, command.TargetDataEpoch, command.BarrierId
	case gen.BeginSwitch:
		next.Phase, next.Maintenance = gen.RestoreSwitching, true
	case gen.MarkBlocked:
		next.Phase, next.Maintenance = gen.RestoreBlocked, true
	case gen.BeginRollback:
		next.Phase, next.Maintenance = gen.RestoreRollingBack, true
	default:
		return gen.RuntimeState{}, platform.NewError(platform.CodeValidation, false, "未知RuntimeTransition action")
	}
	return next, validateRuntimeStateShape(next)
}

func validateActivationInput(current gen.RuntimeState, input gen.RegisterActivationInput) error {
	if current.Phase != gen.RestoreSwitching && current.Phase != gen.RestoreRollingBack {
		return platform.NewError(platform.CodeLocked, false, "当前阶段不接受激活回执")
	}
	if input.RestoreId == "" || input.RestoreId != current.RestoreId {
		return platform.NewError(platform.CodeVersionMismatch, false, "restoreId与当前恢复不一致")
	}
	receipt := input.Receipt
	if !isRequiredModule(string(receipt.Module)) || !platform.ValidIdentifier(platform.Identifier(receipt.StageId)) || receipt.TargetDataEpoch != current.TargetDataEpoch || !platform.ValidHash(platform.Hash(receipt.ManifestHash)) || receipt.Signature == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执字段不合法")
	}
	if current.Phase == gen.RestoreSwitching && receipt.State != gen.Switched {
		return platform.NewError(platform.CodeLocked, false, "切换阶段只接受switched回执")
	}
	if current.Phase == gen.RestoreRollingBack && receipt.State != gen.RolledBack {
		return platform.NewError(platform.CodeLocked, false, "换回阶段只接受rolled-back回执")
	}
	if receipt.RecordedAt.IsZero() {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执时间缺失")
	}
	for _, existing := range current.ActivationReceipts {
		if existing.Module != receipt.Module {
			continue
		}
		if existing.StageId == receipt.StageId && existing.TargetDataEpoch == receipt.TargetDataEpoch && existing.State == receipt.State && existing.ManifestHash == receipt.ManifestHash && existing.RecordedAt.Equal(receipt.RecordedAt) && existing.Signature == receipt.Signature {
			return nil
		}
		if existing.State == receipt.State {
			return platform.NewError(platform.CodeStaleResult, false, "同一域已有不同激活回执")
		}
	}
	return nil
}

func allReceiptsInState(state gen.RuntimeState, expected gen.ActivationReceiptState) bool {
	seen := make(map[string]bool, len(state.ActivationReceipts))
	for _, receipt := range state.ActivationReceipts {
		if receipt.State == expected {
			seen[string(receipt.Module)] = true
		}
	}
	for _, module := range state.RequiredModules {
		if !seen[module] {
			return false
		}
	}
	return true
}

func appendOrReplaceReceipt(receipts []gen.ActivationReceipt, receipt gen.ActivationReceipt) []gen.ActivationReceipt {
	for i := range receipts {
		if receipts[i].Module == receipt.Module {
			receipts[i] = receipt
			return receipts
		}
	}
	return append(receipts, receipt)
}

func upsertActivationReceipt(ctx context.Context, tx *sql.Tx, restoreID gen.Identifier, receipt gen.ActivationReceipt) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO activation_receipts
            (restore_id, module, stage_id, target_data_epoch, state,
             manifest_hash, recorded_at, signature)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (restore_id, module) DO UPDATE SET
            stage_id = excluded.stage_id,
            target_data_epoch = excluded.target_data_epoch,
            state = excluded.state,
            manifest_hash = excluded.manifest_hash,
            recorded_at = excluded.recorded_at,
            signature = excluded.signature`,
		string(restoreID), string(receipt.Module), string(receipt.StageId),
		string(receipt.TargetDataEpoch), string(receipt.State), string(receipt.ManifestHash),
		receipt.RecordedAt.UTC().Format(time.RFC3339Nano), receipt.Signature)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "保存激活回执失败")
	}
	return nil
}

func hashInput(input any) (string, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "运行操作参数不可编码")
	}
	return string(platform.HashBytes(raw)), nil
}
