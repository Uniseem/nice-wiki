package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type OperationContext = platform.OperationContext
type GetBuildInput struct {
	BuildID string `json:"buildId"`
}
type GetBuildResult = gen.GetBuildResult
type ListBuildsInput struct {
	DocumentID string `json:"documentId,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit"`
}
type ListBuildsResult = gen.ListBuildsResult

type adminRuntimeKey struct{}

func WithAdminSigningKey(ctx context.Context, key platform.SigningKey) context.Context {
	return context.WithValue(ctx, adminRuntimeKey{}, key)
}
func authorizeAdmin(op OperationContext) error {
	if op.Actor.Kind != "admin" || op.CallerService != "portal" || !slices.Contains(op.Actor.Grants, "knowledge.manage") {
		return platform.NewError(platform.CodeForbidden, false, "当前账户没有索引管理权限")
	}
	return nil
}
func beginAdminRead(ctx context.Context, op OperationContext) (*sql.Tx, error) {
	if err := authorizeAdmin(op); err != nil {
		return nil, err
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "索引管理快照读取失败")
	}
	var site, epoch string
	if err := tx.QueryRowContext(ctx, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&site, &epoch); err != nil {
		tx.Rollback()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "索引存储身份缺失")
	}
	if site != op.Actor.SiteID || epoch != op.DataEpoch {
		tx.Rollback()
		return nil, platform.NewError(platform.CodeStaleResult, false, "索引管理快照身份已改变")
	}
	return tx, nil
}

func GetBuild(ctx context.Context, op OperationContext, input GetBuildInput) (GetBuildResult, error) {
	tx, err := beginAdminRead(ctx, op)
	if err != nil {
		return GetBuildResult{}, err
	}
	defer tx.Rollback()
	record, err := indexer.ReadBuildRecord(ctx, tx, op, input.BuildID)
	if err != nil {
		return GetBuildResult{}, err
	}
	return GetBuildResult{Build: record.Build}, nil
}

// ListBuilds 只按 buildId keyset 扫描；游标签名绑定管理主体、代次及过滤条件。
func ListBuilds(ctx context.Context, op OperationContext, input ListBuildsInput) (ListBuildsResult, error) {
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 100 {
		return ListBuildsResult{}, platform.NewError(platform.CodeValidation, false, "索引管理分页数量不合法")
	}
	tx, err := beginAdminRead(ctx, op)
	if err != nil {
		return ListBuildsResult{}, err
	}
	defer tx.Rollback()
	key, ok := ctx.Value(adminRuntimeKey{}).(platform.SigningKey)
	if !ok {
		return ListBuildsResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "管理游标签名器未装配")
	}
	binding, _ := platform.CanonicalJSON(struct {
		Actor           platform.AuthContext
		Epoch, Document string
		Limit           int
	}{op.Actor, op.DataEpoch, input.DocumentID, input.Limit})
	scope := "knowledge.builds.v1:" + platform.HashBytes(binding)
	position := ""
	if input.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(key, scope, input.Cursor)
		if err != nil {
			return ListBuildsResult{}, err
		}
		if json.Unmarshal(raw, &position) != nil || !platform.ValidIdentifier(position) {
			return ListBuildsResult{}, platform.NewError(platform.CodeValidation, false, "索引管理游标不合法")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND build_id>? AND (?='' OR document_id=?) ORDER BY build_id LIMIT ?`, op.Actor.SiteID, op.DataEpoch, position, input.DocumentID, input.DocumentID, input.Limit+1)
	if err != nil {
		return ListBuildsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建列表读取失败")
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ListBuildsResult{}, platform.NewError(platform.CodeInternal, false, "构建列表损坏")
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ListBuildsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建列表读取中断")
	}
	result := ListBuildsResult{Builds: make([]gen.IndexBuild, 0)}
	for _, id := range ids[:min(input.Limit, len(ids))] {
		record, err := indexer.ReadBuildRecord(ctx, tx, op, id)
		if err != nil {
			return ListBuildsResult{}, err
		}
		result.Builds = append(result.Builds, record.Build)
	}
	if len(ids) > input.Limit {
		raw, _ := json.Marshal(ids[input.Limit-1])
		cursor, err := serviceauth.SignCursor(key, scope, raw)
		if err != nil {
			return ListBuildsResult{}, err
		}
		result.NextCursor = &cursor
	}
	return result, nil
}
