package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"strings"
	"unicode/utf8"
)

// PrepareEmbeddingPlan 固定原模型声明和单块批次，缓存变化不能改组绕过已派发原语义。
func PrepareEmbeddingPlan(ctx context.Context, op OperationContext, config gen.IndexConfig, chunks []gen.Chunk, runtime indexer.Runtime) (EmbeddingPlan, error) {
	catalog, err := indexer.ReadModelCatalog(ctx, op, runtime)
	if err != nil {
		return EmbeddingPlan{}, err
	}
	profile, model, err := indexer.EmbeddingProfile(catalog, config.EmbeddingProfileId, op.CostClass, config.Space)
	if err != nil {
		return EmbeddingPlan{}, err
	}
	if model.ModelSnapshotHash == "" {
		return EmbeddingPlan{}, platform.NewError(platform.CodeDependencyUnavailable, false, "嵌入计划缺实际模型目录snapshot证明")
	}
	plan := EmbeddingPlan{Profile: profile, Model: model, Batches: [][]string{}}
	for _, value := range chunks {
		if value.EmbeddingTokenCount == nil || *value.EmbeddingTokenCount > model.MaximumInputTokens || utf8.RuneCountInString(indexer.RetrievalText(value)) > 4096 {
			return EmbeddingPlan{}, platform.NewError(platform.CodeBudgetExceeded, false, "完整嵌入文本超过固定模型输入预算")
		}
		plan.Batches = append(plan.Batches, []string{value.ChunkId})
	}
	return plan, nil
}

// ReuseEmbeddingBatch 必须完整命中同检索文字/完整空间才写；部分缓存不改原分组或覆盖已付费向量。
func ReuseEmbeddingBatch(ctx context.Context, op OperationContext, buildID, leaseID string, ids []string, space gen.EmbeddingSpace) (bool, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return false, err
	}
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "原付费向量快照不可读")
	}
	vectors := [][]float64{}
	spaceRaw, _ := platform.CanonicalJSON(space)
	for _, id := range ids {
		var targetRaw []byte
		if err = read.QueryRowContext(ctx, `SELECT chunk_json FROM chunks WHERE build_id=? AND chunk_id=?`, buildID, id).Scan(&targetRaw); err != nil {
			read.Rollback()
			return false, platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "嵌入原片段不存在")
		}
		var target gen.Chunk
		if json.Unmarshal(targetRaw, &target) != nil {
			read.Rollback()
			return false, platform.NewError(platform.CodeIntegrityError, false, "嵌入原片段损坏")
		}
		rows, err := read.QueryContext(ctx, `SELECT c.chunk_json,v.vector_json,v.vector_hash FROM vectors v JOIN chunks c ON c.build_id=v.build_id AND c.chunk_id=v.chunk_id JOIN builds b ON b.build_id=v.build_id WHERE b.site_id=? AND b.data_epoch=? AND v.space_json=? AND c.chunk_hash=? ORDER BY CASE WHEN v.build_id=? AND v.chunk_id=? THEN 0 ELSE 1 END,v.build_id,v.chunk_id`, op.Actor.SiteID, op.DataEpoch, spaceRaw, target.ChunkHash, buildID, id)
		if err != nil {
			read.Rollback()
			return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "付费向量缓存不可读")
		}
		var vector []float64
		for rows.Next() {
			var oldRaw, raw []byte
			var hash string
			if err = rows.Scan(&oldRaw, &raw, &hash); err != nil {
				rows.Close()
				read.Rollback()
				return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "付费向量缓存读取失败")
			}
			var old gen.Chunk
			if json.Unmarshal(oldRaw, &old) != nil || platform.HashBytes([]byte(old.Text)) != old.ChunkHash {
				rows.Close()
				read.Rollback()
				return false, platform.NewError(platform.CodeIntegrityError, false, "缓存向量来源片段损坏")
			}
			if indexer.RetrievalText(old) != indexer.RetrievalText(target) {
				continue
			}
			vector, err = indexer.DecodeVector(raw, hash, space.Dimensions)
			if err != nil {
				rows.Close()
				read.Rollback()
				return false, err
			}
			break
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			read.Rollback()
			return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "缓存向量读取失败")
		}
		if vector == nil {
			read.Rollback()
			return false, nil
		}
		vectors = append(vectors, vector)
	}
	_ = read.Rollback()
	_, err = indexer.StoreEmbeddings(ctx, op, indexer.StoreEmbeddingsInput{BuildID: buildID, LeaseID: leaseID, ChunkIDs: ids, Vectors: vectors, Space: space})
	return err == nil, err
}

// CheckEmbeddingPlan 只在没有原模型阶段时核冻结声明，已经派发的原语义仍只找回原key。
func CheckEmbeddingPlan(ctx context.Context, op OperationContext, buildID string, ids []string, space gen.EmbeddingSpace, plan EmbeddingPlan) error {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return err
	}
	binding := strings.Join(ids, "\x00")
	semantic := "embedding." + platform.HashBytes([]byte(binding))
	var existing int
	if err = runtime.DB.QueryRowContext(ctx, `SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND semantic_key=? AND dispatch_started=1`, op.Actor.SiteID, op.DataEpoch, buildID, semantic).Scan(&existing); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原嵌入语义阶段不可读")
	}
	if existing > 0 {
		return nil
	}
	catalog, err := indexer.ReadModelCatalog(ctx, op, runtime)
	if err != nil {
		return err
	}
	profile, model, err := indexer.EmbeddingProfile(catalog, plan.Profile.ProfileID, op.CostClass, space)
	if err != nil {
		return err
	}
	oldRaw, _ := platform.CanonicalJSON([]any{plan.Profile, plan.Model})
	currentRaw, _ := platform.CanonicalJSON([]any{profile, model})
	if string(oldRaw) != string(currentRaw) {
		return platform.NewError(platform.CodeConflict, false, "原嵌入批准模型计划已变化，保留原分组不另开付费")
	}
	return nil
}
