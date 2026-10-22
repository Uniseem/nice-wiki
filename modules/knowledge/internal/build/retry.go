package build

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

// RetryFailedBuilds 自动补建索引：文档最近一次构建失败（不是取消）、同一发布版本失败不到 3 次、失败构建已创建 2 分钟以上，
// 就为它重新排队。首次构建偶发的并发冲突、限流造成的失败不用等管理员手动点“建立索引”。
// 版本已过期的会在构建开始读正文时自行退役，不另做判断。
func RetryFailedBuilds(ctx context.Context, op OperationContext) error {
	work, tx, cancel, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	config, err := indexer.ReadConfig(work, tx, op)
	var failure *platform.Error
	if errors.As(err, &failure) && failure.Code == platform.CodeNotFound || err == nil && !config.Enabled {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(work, `SELECT b.document_json FROM builds b
		WHERE b.site_id=? AND b.data_epoch=? AND b.space_id=? AND b.state='failed' AND b.failure_code<>'CANCELLED' AND b.created_at<?
		AND b.generation=(SELECT MAX(x.generation) FROM builds x WHERE x.site_id=b.site_id AND x.data_epoch=b.data_epoch AND x.document_id=b.document_id AND x.space_id=b.space_id)
		AND (SELECT COUNT(*) FROM builds y WHERE y.site_id=b.site_id AND y.data_epoch=b.data_epoch AND y.space_id=b.space_id AND y.document_json=b.document_json AND y.state='failed')<3
		AND NOT EXISTS(SELECT 1 FROM document_watermarks w WHERE w.site_id=b.site_id AND w.data_epoch=b.data_epoch AND w.document_id=b.document_id AND w.withdrawn<>0)
		LIMIT 20`, op.Actor.SiteID, op.DataEpoch, config.Space.SpaceId, time.Now().UTC().Add(-2*time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "失败构建读取失败")
	}
	documents := []gen.PublishedRevisionRef{}
	for rows.Next() {
		var raw []byte
		var document gen.PublishedRevisionRef
		if rows.Scan(&raw) == nil && json.Unmarshal(raw, &document) == nil {
			documents = append(documents, document)
		}
	}
	rows.Close()
	queued := []string{}
	for _, document := range documents {
		build, err := QueueBuild(work, tx, op, config, document, false)
		if err != nil {
			return err
		}
		queued = append(queued, build.BuildId)
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "补建索引提交失败")
	}
	for _, id := range queued {
		_ = DispatchBuild(ctx, op, id)
	}
	return nil
}
