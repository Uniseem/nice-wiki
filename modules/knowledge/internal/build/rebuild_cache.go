package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/chunk"
	"nice-wiki/modules/knowledge/internal/extract"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

// ReadRebuildExtractionCache 只读取当前安全模型声明覆盖的既有真实结果，不创建意图或恢复旧任务。
func ReadRebuildExtractionCache(ctx context.Context, op OperationContext, runtime indexer.Runtime, resource RebuildResourcePlan) (extract.ExtractionArtifact, bool, error) {
	plan := resource.Plan
	asset := resource.Source.Asset
	binding, _ := platform.CanonicalJSON([]any{asset.SHA256, asset.MediaType, asset.ByteSize, plan.TaskID, plan.Memory.VersionHash, plan.Memory.MappingRevision, plan.ExtractorVersion})
	rows, err := runtime.DB.QueryContext(ctx, `SELECT COALESCE(r.artifact_json,e.result_json),COALESCE(r.artifact_hash,e.result_hash) FROM extractions e LEFT JOIN extraction_recovered_results r ON r.extraction_id=e.extraction_id WHERE e.site_id=? AND e.data_epoch=? AND e.resource_key=? AND (e.state IN ('ready','unsupported') OR r.extraction_id IS NOT NULL) ORDER BY e.created_at,e.extraction_id`, op.Actor.SiteID, op.DataEpoch, platform.HashBytes(binding))
	if err != nil {
		return extract.ExtractionArtifact{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价原资源结果不可读")
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var hash string
		if err = rows.Scan(&raw, &hash); err != nil {
			return extract.ExtractionArtifact{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价原资源结果不可读")
		}
		var artifact extract.ExtractionArtifact
		if hash != platform.HashBytes(raw) || json.Unmarshal(raw, &artifact) != nil {
			return artifact, false, platform.NewError(platform.CodeIntegrityError, false, "估价原资源结果hash损坏")
		}
		eligible := plan.TaskID == ""
		if artifact.Extraction.ModelSnapshotHash != nil {
			for _, snapshot := range plan.ModelSnapshots {
				if snapshot == *artifact.Extraction.ModelSnapshotHash {
					eligible = true
					break
				}
			}
		}
		if eligible {
			return artifact, true, nil
		}
	}
	if err = rows.Err(); err != nil {
		return extract.ExtractionArtifact{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价原资源结果读取失败")
	}
	return extract.ExtractionArtifact{}, false, nil
}

func ReadRebuildCachedContext(ctx context.Context, op OperationContext, runtime indexer.Runtime, document gen.PublishedRevisionRef, route RebuildRoute, value gen.Chunk) (gen.Chunk, bool, error) {
	for _, model := range route.Models {
		hash := chunk.ContextResultHash(document.ContentHash, *route.Memory, model.ModelSnapshotHash)
		var prefix, prefixHash, evidence string
		err := runtime.DB.QueryRowContext(ctx, `SELECT prefix,prefix_hash,model_snapshot_hash FROM chunk_contexts WHERE site_id=? AND data_epoch=? AND context_hash=? AND source_hash=?`, op.Actor.SiteID, op.DataEpoch, hash, chunk.ContextSourceKey(value)).Scan(&prefix, &prefixHash, &evidence)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return value, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价原背景结果不可读")
		}
		if prefixHash != platform.HashBytes([]byte(prefix)) || evidence != model.ModelSnapshotHash {
			return value, false, platform.NewError(platform.CodeIntegrityError, false, "估价原背景来源或hash损坏")
		}
		value.ContextPrefix = &prefix
		value.ContextVersionHash = &hash
		value.ContextModelSnapshotHash = &evidence
		return value, true, nil
	}
	return value, false, nil
}

func HasRebuildVector(ctx context.Context, op OperationContext, runtime indexer.Runtime, target gen.Chunk, space gen.EmbeddingSpace) (bool, error) {
	binding, _ := platform.CanonicalJSON(space)
	rows, err := runtime.DB.QueryContext(ctx, `SELECT c.chunk_json,v.vector_json,v.vector_hash FROM vectors v JOIN chunks c ON c.build_id=v.build_id AND c.chunk_id=v.chunk_id JOIN builds b ON b.build_id=v.build_id WHERE b.site_id=? AND b.data_epoch=? AND v.space_json=? AND c.chunk_hash=? ORDER BY v.build_id,v.chunk_id`, op.Actor.SiteID, op.DataEpoch, binding, target.ChunkHash)
	if err != nil {
		return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价向量缓存不可读")
	}
	defer rows.Close()
	for rows.Next() {
		var sourceRaw, vectorRaw []byte
		var hash string
		if err = rows.Scan(&sourceRaw, &vectorRaw, &hash); err != nil {
			return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价向量缓存不可读")
		}
		var old gen.Chunk
		if json.Unmarshal(sourceRaw, &old) != nil || old.ChunkHash != platform.HashBytes([]byte(old.Text)) {
			return false, platform.NewError(platform.CodeIntegrityError, false, "估价向量原文hash损坏")
		}
		if indexer.RetrievalText(old) != indexer.RetrievalText(target) {
			continue
		}
		if _, err = indexer.DecodeVector(vectorRaw, hash, space.Dimensions); err != nil {
			return false, err
		}
		return true, nil
	}
	if err = rows.Err(); err != nil {
		return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "估价向量缓存读取失败")
	}
	return false, nil
}
