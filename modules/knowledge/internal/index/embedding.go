package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
 "strings"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type StoreEmbeddingsInput struct {
	BuildID  string             `json:"buildId"`
	ChunkIDs []string           `json:"chunkIds"`
	Vectors  [][]float64        `json:"vectors"`
	Space    gen.EmbeddingSpace `json:"space"`
	LeaseID  string             `json:"leaseId"`
}
type StoreEmbeddingsResult struct {
	Stored int `json:"stored"`
}

// StoreEmbeddings 校验完整空间、固定 chunkHash 与有限分量后原子写入。
func StoreEmbeddings(ctx context.Context, op OperationContext, input StoreEmbeddingsInput) (StoreEmbeddingsResult, error) {
	if len(input.ChunkIDs) != len(input.Vectors) || len(input.Vectors) > 64 {
		return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入数量不匹配或超过批次上限")
	}
	work, tx, record, cancel, err := BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return StoreEmbeddingsResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	if record.Build.Space != input.Space {
		return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入空间已变化")
	}
	space, _ := platform.CanonicalJSON(input.Space)
	seen := make(map[string]bool)
	for i, id := range input.ChunkIDs {
		if seen[id] || len(input.Vectors[i]) != input.Space.Dimensions {
			return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块重复或向量维度不匹配")
		}
		seen[id] = true
		for _, value := range input.Vectors[i] {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "向量含非有限数")
			}
		}
		var hash string
		if err := tx.QueryRowContext(work, `SELECT chunk_hash FROM chunks WHERE build_id=? AND chunk_id=?`, input.BuildID, id).Scan(&hash); err != nil {
			if err == sql.ErrNoRows {
				return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入来源分块不存在")
			}
			return StoreEmbeddingsResult{}, storageError(err)
		}
		raw, err := platform.CanonicalJSON(input.Vectors[i])
		if err != nil {
			return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "向量无法编码")
		}
		var existingHash string
		err = tx.QueryRowContext(work, `SELECT vector_hash FROM vectors WHERE build_id=? AND chunk_id=?`, input.BuildID, id).Scan(&existingHash)
		if err == nil {
			if existingHash != platform.HashBytes(raw) {
				return StoreEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "同一分块已有不同的付费向量")
			}
			continue
		}
		if err != sql.ErrNoRows {
			return StoreEmbeddingsResult{}, storageError(err)
		}
		_, err = tx.ExecContext(work, `INSERT INTO vectors(build_id,chunk_id,chunk_hash,space_json,vector_json,vector_hash) VALUES(?,?,?,?,?,?)`, input.BuildID, id, hash, space, raw, platform.HashBytes(raw))
		if err != nil {
			return StoreEmbeddingsResult{}, storageError(err)
		}
	}
	if _, err := tx.ExecContext(work, `UPDATE builds SET completed_chunks=(SELECT COUNT(*) FROM vectors WHERE build_id=?) WHERE build_id=?`, input.BuildID, input.BuildID); err != nil {
		return StoreEmbeddingsResult{}, storageError(err)
	}
	if err := tx.Commit(); err != nil {
		return StoreEmbeddingsResult{}, storageError(err)
	}
	return StoreEmbeddingsResult{Stored: len(input.Vectors)}, nil
}

// DecodeVector 校验持久向量摘要；供构建与只读快照共用。
func DecodeVector(raw []byte, hash string, dimensions int) ([]float64, error) {
	var vector []float64
	if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &vector) != nil || len(vector) != dimensions {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "持久向量摘要或维度不匹配")
	}
	for _, value := range vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "持久向量非有限")
		}
	}
	return vector, nil
}

type RequestEmbeddingsInput struct {
	BuildID  string             `json:"buildId"`
	ChunkIDs []string           `json:"chunkIds"`
	Space    gen.EmbeddingSpace `json:"space"`
	LeaseID  string             `json:"leaseId"`
}
type RequestEmbeddingsResult struct {
	InvocationID string             `json:"invocationId"`
	Vectors      [][]float64        `json:"vectors"`
	Space        gen.EmbeddingSpace `json:"space"`
}

// RequestEmbeddings 对固定来源最多64项发起一次幂等网关调用，之后写入另核当前租约。
func RequestEmbeddings(ctx context.Context, op OperationContext, input RequestEmbeddingsInput) (RequestEmbeddingsResult, error) {
	if len(input.ChunkIDs) == 0 || len(input.ChunkIDs) > 64 {
		return RequestEmbeddingsResult{}, platform.NewError(platform.CodeValidation, false, "嵌入批次必须一至六十四项")
	}
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return RequestEmbeddingsResult{}, err
	}

	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return RequestEmbeddingsResult{}, storageError(err)
	}
	defer tx.Rollback()
	record, err := ReadBuildRecord(ctx, tx, op, input.BuildID)
	if err != nil {
		return RequestEmbeddingsResult{}, err
	}
	if record.LeaseID != input.LeaseID || !record.LeaseUntil.After(time.Now()) || record.Build.Space != input.Space {
		return RequestEmbeddingsResult{}, platform.NewError(platform.CodeStaleResult, false, "嵌入构建租约或空间已变化")
	}
	chunks, err := ReadChunks(ctx, tx, input.BuildID)
	if err != nil {
		return RequestEmbeddingsResult{}, err
	}
	byID := make(map[string]gen.Chunk)
	for _, chunk := range chunks {
		byID[chunk.ChunkId] = chunk
	}
	_=tx.Rollback()
 catalog,err:=ReadModelCatalog(ctx,op,runtime)
 if err!=nil {return RequestEmbeddingsResult{},err}
 _,selectedModel,err:=EmbeddingProfile(catalog,record.EmbeddingProfileID,op.CostClass,input.Space)
 if err!=nil {return RequestEmbeddingsResult{},err}
 texts := make([]string, 0, len(input.ChunkIDs))
	hashes := make([]string, 0, len(input.ChunkIDs))
	tokens := 0
	seen := make(map[string]bool)
	for _, id := range input.ChunkIDs {
		chunk, exists := byID[id]
		if !exists || seen[id] {
			return RequestEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入分块重复或不存在")
		}
		seen[id] = true
		text:=RetrievalText(chunk)
  if utf8.RuneCountInString(text) > 4096 {
			return RequestEmbeddingsResult{}, platform.NewError(platform.CodeValidation, false, "嵌入输入超过模型或网关预算")
		}
		tokenized,err:=modelruntime.TokenizeSource(ctx,op,modelruntime.TokenizeSourceInput{ModelID:selectedModel.ModelID,Space:input.Space,Text:text})
  if err!=nil {return RequestEmbeddingsResult{},err}
  if tokenized.TokenCount>selectedModel.MaximumInputTokens-tokens {return RequestEmbeddingsResult{},platform.NewError(platform.CodeValidation,false,"嵌入批次超过真实模型token预算")}
  if chunk.EmbeddingTokenCount!=nil&&*chunk.EmbeddingTokenCount!=tokenized.TokenCount {return RequestEmbeddingsResult{},platform.NewError(platform.CodeIntegrityError,false,"已保存完整检索文本token计数不一致")}
  tokens += tokenized.TokenCount
		texts = append(texts, text)
		hashes = append(hashes, platform.HashBytes([]byte(text)))
	}
	binding, _ := platform.CanonicalJSON(struct {
		Build       string
		Space       gen.EmbeddingSpace
		IDs, Hashes []string
	}{input.BuildID, input.Space, input.ChunkIDs, hashes})
	_,qualified,_,releaseQualification,err:=BeginBuildWrite(ctx,op,input.BuildID,input.LeaseID)
 if err!=nil {return RequestEmbeddingsResult{},err};_=qualified.Rollback();releaseQualification()
 ctx,err=modelruntime.WithModelOwner(ctx,input.BuildID)
 if err!=nil {return RequestEmbeddingsResult{},err}
 expected:=&platform.EmbeddingSpace{SpaceID:input.Space.SpaceId,Provider:input.Space.Provider,ModelVersion:input.Space.ModelVersion,Dimensions:int64(input.Space.Dimensions),PreprocessingVersion:input.Space.PreprocessingVersion}
 outcome, err := modelruntime.InvokeModel(ctx, op, modelruntime.InvokeModelInput{SemanticKey:"embedding."+platform.HashBytes([]byte(strings.Join(input.ChunkIDs,"\x00"))),Request: platform.CreateInvocationInput{TaskID: "knowledge.embed", ProfileID: record.EmbeddingProfileID, Capability: platform.CapabilityEmbedding, ExpectedEmbeddingSpace:expected, Texts: texts, MaximumOutputTokens: 0, IDempotencyKey: "build." + platform.HashBytes(binding)}})
	if err != nil {
		return RequestEmbeddingsResult{}, err
	}
	output, err := modelruntime.CompletedOutput(outcome, platform.CapabilityEmbedding)
	if err != nil {
		return RequestEmbeddingsResult{}, err
	}
	if output.EmbeddingSpace == nil || len(output.Vectors) != len(input.ChunkIDs) {
		return RequestEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块嵌入数量或空间缺失")
	}
	space := output.EmbeddingSpace
	if space.SpaceID != input.Space.SpaceId || space.Provider != input.Space.Provider || space.ModelVersion != input.Space.ModelVersion || space.Dimensions != int64(input.Space.Dimensions) || space.PreprocessingVersion != input.Space.PreprocessingVersion {
		return RequestEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块嵌入空间不一致")
	}
	for _, vector := range output.Vectors {
		if len(vector) != input.Space.Dimensions {
			return RequestEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块嵌入维度不一致")
		}
		for _, value := range vector {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return RequestEmbeddingsResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块嵌入非有限")
			}
		}
	}
	return RequestEmbeddingsResult{InvocationID: outcome.InvocationID, Vectors: output.Vectors, Space: input.Space}, nil
}
