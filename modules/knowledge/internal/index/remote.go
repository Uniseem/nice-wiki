package index

import (
	"context"
	"database/sql"
	"math"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/searchclient"
)

// 配置了 Rust 检索服务时，知识库把每篇文档的活动构建（词项 + 向量）推过去；
// 检索服务只负责排名，分块正文仍在这里，命中的分块按构建与分块 ID 回本库取。

func nonZero(vector []float64) bool {
	for _, value := range vector {
		if value != 0 && !math.IsNaN(value) {
			return true
		}
	}
	return false
}

// PushBuild 整篇替换检索服务里这篇文档的内容。
func PushBuild(ctx context.Context, runtime Runtime, site string, record BuildRecord, entries []GenerationEntry) error {
	if runtime.Search == nil {
		return nil
	}
	chunks := make([]searchclient.Chunk, 0, len(entries))
	for _, entry := range entries {
		chunk := searchclient.Chunk{ChunkID: entry.Chunk.ChunkId, Terms: entry.Terms}
		if nonZero(entry.Vector) {
			chunk.Vector = make([]float32, len(entry.Vector))
			for i, value := range entry.Vector {
				chunk.Vector[i] = float32(value)
			}
		}
		chunks = append(chunks, chunk)
	}
	return runtime.Search.ReplaceDocument(ctx, site, record.Build.Document.DocumentId, searchclient.Replace{SpaceID: record.Build.Space.SpaceId, BuildID: record.Build.BuildId, Chunks: chunks})
}

// ReconcileSearch 把检索服务里这个站点的内容和权威的活动索引对齐：缺的补推，多的删掉。
// 激活时的即时推送失败、检索服务重装、文档撤回时检索服务不可达，都靠它在一分钟内收敛。
func ReconcileSearch(ctx context.Context, op OperationContext) error {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil || runtime.Search == nil {
		return nil
	}
	site := op.Actor.SiteID
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return storageError(err)
	}
	defer tx.Rollback()
	config, err := ReadConfig(ctx, tx, op)
	if err != nil || !config.Enabled {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT document_id,build_id FROM active_indexes WHERE site_id=? AND data_epoch=? AND space_id=?`, site, op.DataEpoch, config.Space.SpaceId)
	if err != nil {
		return storageError(err)
	}
	active := map[string]string{}
	for rows.Next() {
		var document, build string
		if err := rows.Scan(&document, &build); err != nil {
			rows.Close()
			return storageError(err)
		}
		active[document] = build
	}
	rows.Close()
	manifest, err := runtime.Search.Manifest(ctx, site)
	if err != nil {
		return err
	}
	for document, build := range active {
		if manifest[document] == build {
			continue
		}
		record, err := ReadBuildRecord(ctx, tx, op, build)
		if err != nil {
			return err
		}
		entries, _, err := ReadGeneration(ctx, tx, record)
		if err != nil {
			return err
		}
		if err := PushBuild(ctx, runtime, site, record, entries); err != nil {
			return err
		}
	}
	for document := range manifest {
		if _, ok := active[document]; !ok {
			if err := runtime.Search.DeleteDocument(ctx, site, document); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ = platform.CodeInternal
