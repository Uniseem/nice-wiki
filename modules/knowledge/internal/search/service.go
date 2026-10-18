package search

import (
	"context"
 qaruntime "nice-wiki/modules/knowledge/internal/runtime"
	"database/sql"
	"encoding/json"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/modules/knowledge/internal/gen"
)

type RetrieveInput = gen.RetrieveInput
type RetrieveResult = gen.RetrieveResult

// SnapshotRow 的来源、词项和向量来自同一个只读事务。
type SnapshotRow struct {
	Chunk  gen.Chunk
	Terms  map[string]int
	Length int
	Vector []float64
}
type Snapshot struct {
	SiteID      string
	DataEpoch   string
	Language    string
	Space       gen.EmbeddingSpace
	VersionHash string
	Rows        []SnapshotRow
}
type SnapshotReader interface {
	ReadActiveSnapshot(context.Context, OperationContext, gen.EmbeddingSpace) (Snapshot, error)
}

// Runtime 只由服务装配传入，不能从 HTTP 输入建立。
// Rerank 必须装配网关持久调用链，不能直接访问模型供应方。
type Runtime struct {
	Store                     SnapshotReader
	Content                   *gen.ContentClient
	ValidatePublicationResult func(json.RawMessage) error
	Space                     gen.EmbeddingSpace
	Language                  string
	EmbeddingProfileID        string
	RerankProfileID           string
	FastProfileID             string
	RerankEnabled             bool
}
type runtimeKey struct{}
type snapshotKey struct{}

func NewSearchRuntime(runtime Runtime) (Runtime, error) {
	validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "ContentValidatePublicationsResult")
	if err != nil {
		return Runtime{}, err
	}
	runtime.ValidatePublicationResult = validator
	return runtime, nil
}
func WithSearchRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}
func searchRuntime(ctx context.Context) (Runtime, error) {
	runtime, ok := ctx.Value(runtimeKey{}).(Runtime)
	if !ok || runtime.Store == nil || runtime.Content == nil || runtime.ValidatePublicationResult == nil {
		return Runtime{}, platform.NewError(platform.CodeDependencyUnavailable, false, "检索运行依赖未装配")
	}
	return runtime, nil
}
func activeSnapshot(ctx context.Context, op OperationContext) (Snapshot, error) {
	if snapshot, ok := ctx.Value(snapshotKey{}).(Snapshot); ok {
		return snapshot, nil
	}
	runtime, err := searchRuntime(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := runtime.Store.ReadActiveSnapshot(ctx, op, runtime.Space)
	if err != nil {
		return Snapshot{}, err
	}
	if snapshot.Language == "" && len(snapshot.Rows) == 0 {
		snapshot.Language = runtime.Language
	}
	if snapshot.SiteID != op.Actor.SiteID || snapshot.DataEpoch != op.DataEpoch || snapshot.Space != runtime.Space || snapshot.Language != runtime.Language {
		return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动索引范围或空间不匹配")
	}
	return snapshot, nil
}

// Retrieve 固定一次 active 快照，先权威过滤再重排，选材后再次核对可见性。
func Retrieve(ctx context.Context, op OperationContext, input RetrieveInput) (RetrieveResult, error) {
 var bindErr error
 ctx, bindErr = qaruntime.BindAdmissionPermit(ctx, op, input.AdmissionPermit)
 if bindErr != nil { return RetrieveResult{}, bindErr }
	runtime, err := searchRuntime(ctx)
	if err != nil {
		return RetrieveResult{}, err
	}
	if input.Space != runtime.Space || input.Language != runtime.Language {
		return RetrieveResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询不是当前基准语言和空间")
	}
	query, err := NormalizeQuery(ctx, op, NormalizeQueryInput{Query: input.Query, Language: input.Language})
	if err != nil {
		return RetrieveResult{}, err
	}
	snapshot, err := activeSnapshot(ctx, op)
	if err != nil {
		return RetrieveResult{}, err
	}
	ctx = context.WithValue(ctx, snapshotKey{}, snapshot)
	lexical, err := SearchLexical(ctx, op, SearchLexicalInput{Normalized: query.Normalized, Language: input.Language, Limit: 80})
	if err != nil {
		return RetrieveResult{}, err
	}
	vector := SearchVectorResult{Candidates: make([]gen.Candidate, 0)}
	if !input.LexicalOnly {
		var embedding []float64
		if input.Embedding != nil {
			provided := input.Embedding
			for _, value := range provided.Vector {
				embedding = append(embedding, float64(value))
			}
			if provided.QueryHash != query.QueryHash || provided.DataEpoch != op.DataEpoch || provided.Space != input.Space || !validVector(embedding, input.Space.Dimensions) {
				return RetrieveResult{}, platform.NewError(platform.CodeIntegrityError, false, "复用查询嵌入身份不一致")
			}
			raw, err := platform.CanonicalJSON(provided.Vector)
			if err != nil || platform.HashBytes(raw) != provided.VectorHash {
				return RetrieveResult{}, platform.NewError(platform.CodeIntegrityError, false, "复用查询向量摘要不一致")
			}
		} else {
			result, err := EmbedQuery(ctx, op, EmbedQueryInput{Normalized: query.Normalized, Space: input.Space, ProfileID: runtime.EmbeddingProfileID, AdmissionPermit: input.AdmissionPermit})
			if err != nil {
				return RetrieveResult{}, err
			}
			embedding = result.Vector
		}
		vector, err = SearchVector(ctx, op, SearchVectorInput{Vector: embedding, Space: input.Space, Limit: 80})
		if err != nil {
			return RetrieveResult{}, err
		}
	}
	fused, err := FuseCandidates(ctx, op, FuseCandidatesInput{Lexical: lexical.Candidates, Vector: vector.Candidates, Limit: 40})
	if err != nil {
		return RetrieveResult{}, err
	}
	visible, err := ValidateCandidates(ctx, op, ValidateCandidatesInput{Candidates: fused.Candidates})
	if err != nil {
		return RetrieveResult{}, err
	}
	candidates := visible.Candidates
	if len(candidates) > 0 && runtime.RerankEnabled {
		reranked, err := RerankCandidates(ctx, op, RerankCandidatesInput{Query: query.Normalized, Candidates: candidates, ProfileID: runtime.RerankProfileID})
		if err != nil {
			return RetrieveResult{}, err
		}
		candidates = reranked.Candidates
	}
	selected, err := SelectContext(ctx, op, SelectContextInput{Candidates: candidates, MaximumTokens: input.MaximumTokens, MaximumChunks: 8})
	if err != nil {
		return RetrieveResult{}, err
	}
	visible, err = ValidateCandidates(ctx, op, ValidateCandidatesInput{Candidates: selected.Candidates})
	if err != nil {
		return RetrieveResult{}, err
	}
	return RetrieveResult{Result: gen.SearchResult{Candidates: visible.Candidates, Space: snapshot.Space, IndexVersionHash: snapshot.VersionHash, Degraded: input.LexicalOnly, Truncated: selected.Truncated || len(visible.Candidates) < len(selected.Candidates)}}, nil
}

// publicationKey 绑定引用全部字段，不把同文档旧修订合并为一个身份。
func publicationKey(reference gen.PublishedRevisionRef) string {
	raw, _ := json.Marshal(reference)
	return string(raw)
}
func sortedSnapshotRows(rows []SnapshotRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Chunk.ChunkId < rows[j].Chunk.ChunkId })
}

// SQLSnapshotReader 只读取本域私有 SQL 索引；事务固定快照后把材料载入内存。
type SQLSnapshotReader struct{ DB *sql.DB }

func (reader SQLSnapshotReader) ReadActiveSnapshot(ctx context.Context, op OperationContext, requested gen.EmbeddingSpace) (Snapshot, error) {
	tx, err := reader.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动索引读取失败")
	}
	defer tx.Rollback()
	snapshot := Snapshot{Rows: make([]SnapshotRow, 0), Space: requested}
	if err := tx.QueryRowContext(ctx, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&snapshot.SiteID, &snapshot.DataEpoch); err != nil {
		return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动索引身份缺失")
	}
	if snapshot.SiteID != op.Actor.SiteID || snapshot.DataEpoch != op.DataEpoch {
		return Snapshot{}, platform.NewError(platform.CodeStaleResult, false, "活动索引站点或代次已改变")
	}
	rows, err := tx.QueryContext(ctx, `SELECT build_id,generation FROM active_indexes WHERE site_id=? AND data_epoch=? AND space_id=? ORDER BY document_id`, snapshot.SiteID, snapshot.DataEpoch, requested.SpaceId)
	if err != nil {
		return Snapshot{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动代数读取失败")
	}
	type active struct {
		id         string
		generation int
	}
	activeBuilds := make([]active, 0)
	for rows.Next() {
		var entry active
		if err := rows.Scan(&entry.id, &entry.generation); err != nil {
			rows.Close()
			return Snapshot{}, platform.NewError(platform.CodeInternal, false, "活动代数损坏")
		}
		activeBuilds = append(activeBuilds, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Snapshot{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "活动代数读取中断")
	}
	manifest := make([]string, 0, len(activeBuilds))
	seen := make(map[string]bool)
	for _, active := range activeBuilds {
		record, err := indexer.ReadBuildRecord(ctx, tx, op, active.id)
		if err != nil {
			return Snapshot{}, err
		}
		if string(record.Build.State) != "active" || record.Build.Generation != active.generation || record.Build.Space != requested {
			return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动代数指针或空间不一致")
		}
		if snapshot.Language != "" && snapshot.Language != record.Language {
			return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动代数基准语言不一致")
		}
		snapshot.Language = record.Language
		entries, hash, err := indexer.ReadGeneration(ctx, tx, record)
		if err != nil {
			return Snapshot{}, err
		}
		if hash != record.ManifestHash {
			return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动代数清单摘要不一致")
		}
		manifest = append(manifest, active.id+":"+hash)
		for _, entry := range entries {
			if seen[entry.Chunk.ChunkId] {
				return Snapshot{}, platform.NewError(platform.CodeIntegrityError, false, "活动分块标识重复")
			}
			seen[entry.Chunk.ChunkId] = true
			snapshot.Rows = append(snapshot.Rows, SnapshotRow{Chunk: entry.Chunk, Terms: entry.Terms, Length: entry.Length, Vector: entry.Vector})
		}
	}
	raw, _ := platform.CanonicalJSON(struct {
		Space  gen.EmbeddingSpace
		Builds []string
	}{requested, manifest})
	snapshot.VersionHash = platform.HashBytes(raw)
	sortedSnapshotRows(snapshot.Rows)
	return snapshot, nil
}
