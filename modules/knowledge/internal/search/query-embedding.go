package search

import (
	"context"
	"database/sql"
	"encoding/json"
	qaruntime "nice-wiki/modules/knowledge/internal/model"
	"sync"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
)

type CreateQueryEmbeddingInput = gen.CreateQueryEmbeddingInput
type CreateQueryEmbeddingResult = gen.CreateQueryEmbeddingResult

var queryFlights = struct {
	sync.Mutex
	pending map[string]chan struct{}
}{pending: make(map[string]chan struct{})}

func acquireQueryFlight(ctx context.Context, key string) (func(), error) {
	for {
		queryFlights.Lock()
		waiting, exists := queryFlights.pending[key]
		if !exists {
			done := make(chan struct{})
			queryFlights.pending[key] = done
			queryFlights.Unlock()
			return func() { queryFlights.Lock(); delete(queryFlights.pending, key); close(done); queryFlights.Unlock() }, nil
		}
		queryFlights.Unlock()
		select {
		case <-ctx.Done():
			return nil, platform.NormalizeIOError(ctx.Err(), platform.CodeInternal, false, "查询嵌入等待已取消")
		case <-waiting:
		}
	}
}

// CreateQueryEmbedding 以可信根、规范查询、完整空间和代次复用一次持久调用。
// 未终态调用重启后仍使用原 operationId，不自动创建新的付费 attempt。
func CreateQueryEmbedding(ctx context.Context, op OperationContext, input CreateQueryEmbeddingInput) (CreateQueryEmbeddingResult, error) {
	var bindErr error
	ctx, bindErr = qaruntime.BindAdmissionPermit(ctx, op, input.AdmissionPermit)
	if bindErr != nil {
		return CreateQueryEmbeddingResult{}, bindErr
	}
	runtime, err := searchRuntime(ctx)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	if runtime.Space != input.Space {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入空间不是当前配置")
	}
	query, err := NormalizeQuery(ctx, op, NormalizeQueryInput{Query: input.Query, Language: runtime.Language})
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	space, _ := platform.CanonicalJSON(input.Space)
	identity, _ := platform.CanonicalJSON(struct {
		Site, Epoch, Root, Query string
		Space                    gen.EmbeddingSpace
	}{op.Actor.SiteID, op.DataEpoch, op.RootOperationID, query.QueryHash, input.Space})
	key := platform.HashBytes(identity)
	release, err := acquireQueryFlight(ctx, key)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	defer release()
	store, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	if store.WriteWindow.Runtime == nil || store.WriteWindow.WriteWindow == nil {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "查询嵌入写窗口未装配")
	}
	workKey, viewKey, cancelKey, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), store.WriteWindow)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	if viewKey.DataEpoch != op.DataEpoch {
		cancelKey()
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeStaleResult, false, "查询嵌入键的代次已变化")
	}
	_, err = store.DB.ExecContext(workKey, `INSERT INTO query_embedding_keys(site_id,data_epoch,actor_id,idempotency_key,payload_hash,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT DO NOTHING`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, input.IdempotencyKey, key, platform.Now())
	if err != nil {
		cancelKey()
		return CreateQueryEmbeddingResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "查询嵌入幂等键登记失败")
	}
	var storedHash string
	err = store.DB.QueryRowContext(workKey, `SELECT payload_hash FROM query_embedding_keys WHERE site_id=? AND data_epoch=? AND actor_id=? AND idempotency_key=?`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, input.IdempotencyKey).Scan(&storedHash)
	cancelKey()
	if err != nil {
		return CreateQueryEmbeddingResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "查询嵌入幂等键读取失败")
	}
	if storedHash != key {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeConflict, false, "查询嵌入幂等键已用于其他输入")
	}
	load := func(work context.Context) (string, []byte, error) {
		var operationID string
		var result, storedSpace []byte
		err := store.DB.QueryRowContext(work, `SELECT operation_id,result_json,space_json FROM query_embeddings WHERE site_id=? AND data_epoch=? AND root_operation_id=? AND query_hash=? AND space_id=?`, op.Actor.SiteID, op.DataEpoch, op.RootOperationID, query.QueryHash, input.Space.SpaceId).Scan(&operationID, &result, &storedSpace)
		if err == nil && string(storedSpace) != string(space) {
			return "", nil, platform.NewError(platform.CodeIntegrityError, false, "相同空间标识声明已变化")
		}
		return operationID, result, err
	}
	operationID, raw, err := load(ctx)
	if err == sql.ErrNoRows {
		work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), store.WriteWindow)
		if err != nil {
			return CreateQueryEmbeddingResult{}, err
		}
		if view.DataEpoch != op.DataEpoch {
			cancel()
			return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeStaleResult, false, "查询嵌入数据代次已变化")
		}
		_, err = store.DB.ExecContext(work, `INSERT INTO query_embeddings(site_id,data_epoch,root_operation_id,query_hash,space_id,space_json,operation_id) VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, op.Actor.SiteID, op.DataEpoch, op.RootOperationID, query.QueryHash, input.Space.SpaceId, space, op.OperationID)
		cancel()
		if err != nil {
			return CreateQueryEmbeddingResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "查询嵌入登记失败")
		}
		operationID, raw, err = load(ctx)
	}
	if err != nil {
		return CreateQueryEmbeddingResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "查询嵌入读取失败")
	}
	if len(raw) > 0 {
		var result CreateQueryEmbeddingResult
		if json.Unmarshal(raw, &result) != nil || result.Embedding.QueryHash != query.QueryHash || result.Embedding.Space != input.Space || result.Embedding.DataEpoch != op.DataEpoch {
			return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "持久查询嵌入身份损坏")
		}
		encoded, err := platform.CanonicalJSON(result.Embedding.Vector)
		if err != nil || platform.HashBytes(encoded) != result.Embedding.VectorHash {
			return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "持久查询嵌入摘要不一致")
		}
		return result, nil
	}
	catalog, err := indexer.ReadModelCatalog(ctx, op, store)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	profile, _, err := indexer.EmbeddingProfile(catalog, "", op.CostClass, input.Space)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	expected := &platform.EmbeddingSpace{SpaceID: input.Space.SpaceId, Provider: input.Space.Provider, ModelVersion: input.Space.ModelVersion, Dimensions: int64(input.Space.Dimensions), PreprocessingVersion: input.Space.PreprocessingVersion}
	call := op
	call.OperationID = operationID
	outcome, err := modelruntime.InvokeModel(ctx, call, modelruntime.InvokeModelInput{SemanticKey: "query-embedding." + key, Request: platform.CreateInvocationInput{TaskID: "knowledge.embed", ProfileID: profile.ProfileID, Capability: platform.CapabilityEmbedding, ExpectedEmbeddingSpace: expected, Texts: []string{query.Normalized}, MaximumOutputTokens: 0, IDempotencyKey: "query." + key}})
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	output, err := modelruntime.CompletedOutput(outcome, platform.CapabilityEmbedding)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	if output.EmbeddingSpace == nil || len(output.Vectors) != 1 {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入结果缺失")
	}
	actual := output.EmbeddingSpace
	if actual.SpaceID != input.Space.SpaceId || actual.Provider != input.Space.Provider || actual.ModelVersion != input.Space.ModelVersion || actual.Dimensions != int64(input.Space.Dimensions) || actual.PreprocessingVersion != input.Space.PreprocessingVersion || !validVector(output.Vectors[0], input.Space.Dimensions) {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入结果空间不一致")
	}
	vectorRaw, _ := platform.CanonicalJSON(output.Vectors[0])
	result := CreateQueryEmbeddingResult{Embedding: gen.QueryEmbedding{QueryHash: query.QueryHash, Space: input.Space, Vector: output.Vectors[0], VectorHash: platform.HashBytes(vectorRaw), InvocationId: outcome.InvocationID, DataEpoch: op.DataEpoch}}
	raw, _ = platform.CanonicalJSON(result)
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), store.WriteWindow)
	if err != nil {
		return CreateQueryEmbeddingResult{}, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return CreateQueryEmbeddingResult{}, platform.NewError(platform.CodeStaleResult, false, "查询嵌入保存代次已变化")
	}
	_, err = store.DB.ExecContext(work, `UPDATE query_embeddings SET result_json=? WHERE site_id=? AND data_epoch=? AND root_operation_id=? AND query_hash=? AND space_id=? AND operation_id=? AND result_json IS NULL`, raw, op.Actor.SiteID, op.DataEpoch, op.RootOperationID, query.QueryHash, input.Space.SpaceId, operationID)
	if err != nil {
		return CreateQueryEmbeddingResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "查询嵌入保存失败")
	}
	return result, nil
}
