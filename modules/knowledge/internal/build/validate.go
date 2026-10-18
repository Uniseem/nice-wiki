package build

import (
	"context"
	"database/sql"
	"time"

	"nice-wiki/libs/platform"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type OperationContext = platform.OperationContext
type ValidateBuildInput struct {
	BuildID string `json:"buildId"`
	LeaseID string `json:"leaseId"`
}
type ValidateBuildResult struct {
	Valid        bool   `json:"valid"`
	ManifestHash string `json:"manifestHash"`
}

// ValidateBuild 先检查完整SQL代数，再取权威可见性并绑定不可变清单写回。
func ValidateBuild(ctx context.Context, op OperationContext, input ValidateBuildInput) (ValidateBuildResult, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	if runtime.Content == nil || runtime.ValidatePublicationResult == nil {
		return ValidateBuildResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "发布权威未装配")
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ValidateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建校验读取失败")
	}
	defer tx.Rollback()
	record, err := indexer.ReadBuildRecord(ctx, tx, op, input.BuildID)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	_, hash, err := indexer.ReadGeneration(ctx, tx, record)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	_ = tx.Rollback()
	observed, revision, err := indexer.ValidatePublication(ctx, op, runtime.Content, runtime.ValidatePublicationResult, record.Build.Document)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	work, write, current, cancel, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	defer cancel()
	defer write.Rollback()
	_, currentHash, err := indexer.ReadGeneration(work, write, current)
	if err != nil {
		return ValidateBuildResult{}, err
	}
	if currentHash != hash || current.Build.Document != record.Build.Document || time.Since(observed) > time.Second {
		return ValidateBuildResult{}, platform.NewError(platform.CodeStaleResult, false, "构建或可见性校验已过期")
	}
	_, err = write.ExecContext(work, `UPDATE builds SET manifest_hash=?,validated_at=?,visibility_revision=?,state='validating' WHERE build_id=?`, hash, observed.UTC().Format(time.RFC3339Nano), revision, input.BuildID)
	if err != nil {
		return ValidateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建证明写入失败")
	}
	if err := write.Commit(); err != nil {
		return ValidateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建证明提交失败")
	}
	return ValidateBuildResult{Valid: true, ManifestHash: hash}, nil
}
