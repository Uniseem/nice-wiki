package runtime

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"path/filepath"
)

type OperationContext = platform.OperationContext

type LoadRuntimeInput struct {
	DataDirectory string `json:"dataDirectory"`
	ServiceID     string `json:"serviceId"`
}
type LoadRuntimeResult struct {
	Ready            bool `json:"ready"`
	ActiveGeneration int  `json:"activeGeneration"`
}

// LoadRuntime 检查本域SQL清单与全部活动代数；损坏即关闭就绪，不重建或调用模型。
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error) {
	if input.ServiceID != "knowledge" || op.CallerService != "knowledge" || op.Actor.Kind != "service" || !filepath.IsAbs(input.DataDirectory) {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "索引启动身份或私有目录不合法")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return LoadRuntimeResult{}, err
	}
	if !filepath.IsAbs(runtime.DataRoot) || filepath.Clean(input.DataDirectory) != filepath.Clean(runtime.DataRoot) {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "索引启动目录不匹配可信装配")
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return LoadRuntimeResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "索引启动快照失败")
	}
	defer tx.Rollback()
	var site, epoch string
	if err = tx.QueryRowContext(ctx, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&site, &epoch); err != nil {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeIntegrityError, false, "索引存储身份缺失")
	}
	if site != op.Actor.SiteID || epoch != op.DataEpoch {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeStaleResult, false, "索引启动代次或站点不一致")
	}
	var integrity string
	if err = tx.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity); err != nil || integrity != "ok" {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeIntegrityError, false, "索引私有库不可用")
	}
	rows, err := tx.QueryContext(ctx, `SELECT build_id,generation,document_id,space_id FROM active_indexes WHERE site_id=? AND data_epoch=? ORDER BY document_id,space_id`, site, epoch)
	if err != nil {
		return LoadRuntimeResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动清单读取失败")
	}
	type pointer struct {
		id, document, space string
		generation          int
	}
	pointers := []pointer{}
	for rows.Next() {
		var p pointer
		if err = rows.Scan(&p.id, &p.generation, &p.document, &p.space); err != nil {
			rows.Close()
			return LoadRuntimeResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动清单读取失败")
		}
		pointers = append(pointers, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return LoadRuntimeResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动清单读取失败")
	}
	result := LoadRuntimeResult{Ready: true}
	for _, p := range pointers {
		record, err := indexer.ReadBuildRecord(ctx, tx, op, p.id)
		if err != nil {
			return LoadRuntimeResult{}, err
		}
		if string(record.Build.State) != "active" || record.Build.Generation != p.generation || record.Build.Document.DocumentId != p.document || record.Build.Space.SpaceId != p.space {
			return LoadRuntimeResult{}, platform.NewError(platform.CodeIntegrityError, false, "活动指针来源或空间不一致，索引不可用")
		}
		_, hash, err := indexer.ReadGeneration(ctx, tx, record)
		if err != nil {
			return LoadRuntimeResult{}, err
		}
		if hash != record.ManifestHash {
			return LoadRuntimeResult{}, platform.NewError(platform.CodeIntegrityError, false, "活动清单摘要不一致，索引不可用")
		}
		if p.generation > result.ActiveGeneration {
			result.ActiveGeneration = p.generation
		}
	}
	return result, nil
}
