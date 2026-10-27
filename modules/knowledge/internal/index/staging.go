package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/libs/platform/searchclient"
	"nice-wiki/modules/knowledge/internal/gen"
)

type OperationContext = platform.OperationContext

type Runtime struct {
	DB              *sql.DB
	DataRoot        string
	Gateway         *gen.ModelGatewayClient
	InferenceClient platform.GatewayClient
	Memory          *gen.MemoryClient
	Jobs            *gen.JobsClient
	// Events 是共用的事件队列，索引服务从它读发布事件。
	Events                      *queue.Queue
	Media                       *gen.MediaClient
	BaseLanguage                string
	Validators                  map[string]func(json.RawMessage) error
	Content                     *gen.ContentClient
	ValidatePublicationResult   func(json.RawMessage) error
	WriteWindow                 platform.WriteWindowConfig
	MaximumEmbeddingInputTokens int
	// Search 是 Rust 检索服务客户端；没配置时用进程内检索。
	Search *searchclient.Client
}
type runtimeKey struct{}

func NewIndexRuntime(runtime Runtime) (Runtime, error) {
	validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "ContentValidatePublicationsResult")
	if err != nil {
		return Runtime{}, err
	}
	runtime.ValidatePublicationResult = validator
	runtime.Validators = map[string]func(json.RawMessage) error{}
	for _, name := range []string{"MemoryResolveContextResult", "GatewayReadModelsResult", "GatewayReadProfilesResult", "GatewayReadTaskAssignmentsResult", "GatewayTokenizeTextResult", "GatewayQuoteCostResult", "GatewayResolveTaskRouteResult", "JobsReadJobResult", "JobsCreateJobResult", "JobsCancelJobResult", "ContentReadPublishedRevisionResult", "ContentReadPublicTreeResult", "MediaReadAssetResult", "PublicationChangedPayload", "PublicationWithdrawnPayload"} {
		validate, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if err != nil {
			return Runtime{}, err
		}
		runtime.Validators[name] = validate
	}
	return runtime, nil
}
func WithIndexRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}
func RuntimeFromContext(ctx context.Context) (Runtime, error) {
	runtime, ok := ctx.Value(runtimeKey{}).(Runtime)
	if !ok || runtime.DB == nil {
		return Runtime{}, platform.NewError(platform.CodeDependencyUnavailable, false, "索引存储未装配")
	}
	return runtime, nil
}

type WriteStagingChunksInput struct {
	BuildID string      `json:"buildId"`
	Chunks  []gen.Chunk `json:"chunks"`
	LeaseID string      `json:"leaseId"`
}
type WriteStagingChunksResult struct {
	Written    int `json:"written"`
	Generation int `json:"generation"`
}

type BuildRecord struct {
	Build              gen.IndexBuild
	Language           string
	EmbeddingProfileID string
	LeaseID            string
	LeaseUntil         time.Time
	ManifestHash       string
}

// ReadBuildRecord 只在调用方同一事务内读取本域构建范围与不可变来源。
func ReadBuildRecord(ctx context.Context, tx *sql.Tx, op OperationContext, id string) (BuildRecord, error) {
	var record BuildRecord
	var document, space []byte
	var site, epoch, leaseUntil string
	var state, failureCode string
	var cancelled int
	err := tx.QueryRowContext(ctx, `SELECT site_id,data_epoch,document_json,space_json,language,embedding_profile_id,state,generation,lease_id,lease_until,cancelled,completed_chunks,total_chunks,failure_code,manifest_hash FROM builds WHERE build_id=?`, id).Scan(&site, &epoch, &document, &space, &record.Language, &record.EmbeddingProfileID, &state, &record.Build.Generation, &record.LeaseID, &leaseUntil, &cancelled, &record.Build.CompletedChunks, &record.Build.TotalChunks, &failureCode, &record.ManifestHash)
	if err == sql.ErrNoRows {
		return BuildRecord{}, platform.NewError(platform.CodeNotFound, false, "构建不存在")
	}
	if err != nil {
		return BuildRecord{}, storageError(err)
	}
	if site != op.Actor.SiteID {
		return BuildRecord{}, platform.NewError(platform.CodeForbidden, false, "构建不属于当前站点")
	}
	if epoch != op.DataEpoch {
		return BuildRecord{}, platform.NewError(platform.CodeStaleResult, false, "构建不属于当前数据代次")
	}
	if json.Unmarshal(document, &record.Build.Document) != nil || json.Unmarshal(space, &record.Build.Space) != nil {
		return BuildRecord{}, platform.NewError(platform.CodeIntegrityError, false, "构建不可变来源损坏")
	}
	if failureCode != "" {
		record.Build.FailureCode = &failureCode
	}
	record.Build.BuildId = id
	record.Build.State = gen.IndexBuildState(state)
	if record.Build.Document.DataEpoch != epoch {
		return BuildRecord{}, platform.NewError(platform.CodeIntegrityError, false, "构建来源代次不一致")
	}
	if leaseUntil != "" {
		record.LeaseUntil, err = platform.ParseTimestamp(leaseUntil)
		if err != nil {
			return BuildRecord{}, platform.NewError(platform.CodeIntegrityError, false, "构建租约时间损坏")
		}
	}
	if cancelled != 0 {
		record.Build.State = gen.IndexBuildState("cancelled")
	}
	return record, nil
}

// BeginBuildWrite 使用平台写窗口，且截止不晚于本次持久构建租约。
func BeginBuildWrite(ctx context.Context, op OperationContext, buildID, leaseID string) (context.Context, *sql.Tx, BuildRecord, context.CancelFunc, error) {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return nil, nil, BuildRecord{}, nil, err
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return nil, nil, BuildRecord{}, nil, platform.NewError(platform.CodeDependencyUnavailable, false, "索引写窗口未装配")
	}
	work, view, cancelWindow, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, nil, BuildRecord{}, nil, err
	}
	fail := func(err error) (context.Context, *sql.Tx, BuildRecord, context.CancelFunc, error) {
		cancelWindow()
		return nil, nil, BuildRecord{}, nil, err
	}
	if view.DataEpoch != op.DataEpoch {
		return fail(platform.NewError(platform.CodeStaleResult, false, "索引写入代次已变化"))
	}
	var untilRaw string
	if err := runtime.DB.QueryRowContext(work, `SELECT lease_until FROM builds WHERE build_id=? AND site_id=? AND data_epoch=? AND lease_id=?`, buildID, op.Actor.SiteID, op.DataEpoch, leaseID).Scan(&untilRaw); err != nil {
		return fail(platform.NewError(platform.CodeStaleResult, false, "构建租约不存在"))
	}
	until, err := platform.ParseTimestamp(untilRaw)
	if err != nil || !until.After(time.Now()) {
		return fail(platform.NewError(platform.CodeStaleResult, false, "构建租约已失效"))
	}
	work, cancelLease := context.WithDeadline(work, until)
	previousCancel := cancelWindow
	cancelWindow = func() { cancelLease(); previousCancel() }
	tx, err := runtime.DB.BeginTx(work, nil)
	if err != nil {
		return fail(storageError(err))
	}
	record, err := ReadBuildRecord(work, tx, op, buildID)
	if err != nil {
		_ = tx.Rollback()
		return fail(err)
	}
	switch string(record.Build.State) {
	case "queued", "fetching", "extracting", "chunking", "embedding", "validating":
	default:
		_ = tx.Rollback()
		return fail(platform.NewError(platform.CodeStaleResult, false, "构建已失去写入资格"))
	}
	if record.LeaseID != leaseID || !record.LeaseUntil.After(time.Now()) {
		_ = tx.Rollback()
		return fail(platform.NewError(platform.CodeStaleResult, false, "构建租约已失效"))
	}
	return work, tx, record, cancelWindow, nil
}

// WriteStagingChunks 在一个事务保存至多 256 块；重放必须与原完整块一致。
func WriteStagingChunks(ctx context.Context, op OperationContext, input WriteStagingChunksInput) (WriteStagingChunksResult, error) {
	if len(input.Chunks) > 256 {
		return WriteStagingChunksResult{}, platform.NewError(platform.CodeValidation, false, "分块批次超过上限")
	}
	work, tx, record, cancel, err := BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return WriteStagingChunksResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	result := WriteStagingChunksResult{Generation: record.Build.Generation}
	for _, chunk := range input.Chunks {
		if chunk.Document != record.Build.Document || chunk.Block.Document != chunk.Document || platform.HashBytes([]byte(chunk.Text)) != chunk.ChunkHash || len(chunk.SourceOffsets) != len(chunk.Text) {
			return WriteStagingChunksResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块来源或正文摘要不匹配")
		}
		raw, err := platform.CanonicalJSON(chunk)
		if err != nil {
			return WriteStagingChunksResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块无法编码")
		}
		var previous []byte
		err = tx.QueryRowContext(work, `SELECT chunk_json FROM chunks WHERE build_id=? AND chunk_id=?`, input.BuildID, chunk.ChunkId).Scan(&previous)
		if err == nil {
			if string(previous) != string(raw) {
				return WriteStagingChunksResult{}, platform.NewError(platform.CodeIntegrityError, false, "同分块标识对应不同内容")
			}
			continue
		}
		if err != sql.ErrNoRows {
			return WriteStagingChunksResult{}, storageError(err)
		}
		if _, err = tx.ExecContext(work, `INSERT INTO chunks(build_id,chunk_id,chunk_hash,chunk_json,ordinal) VALUES(?,?,?,?,?)`, input.BuildID, chunk.ChunkId, chunk.ChunkHash, raw, chunk.Ordinal); err != nil {
			return WriteStagingChunksResult{}, storageError(err)
		}
		result.Written++
	}
	if _, err := tx.ExecContext(work, `UPDATE builds SET total_chunks=(SELECT COUNT(*) FROM chunks WHERE build_id=?) WHERE build_id=?`, input.BuildID, input.BuildID); err != nil {
		return WriteStagingChunksResult{}, storageError(err)
	}
	if err := tx.Commit(); err != nil {
		return WriteStagingChunksResult{}, storageError(err)
	}
	return result, nil
}

func storageError(err error) error {
	return platform.NormalizeIOError(err, platform.CodeInternal, false, "索引存储操作失败")
}

// ReadChunks 从同一事务读取完整确定性分块，不输出部分损坏的索引。
func ReadChunks(ctx context.Context, tx *sql.Tx, buildID string) ([]gen.Chunk, error) {
	rows, err := tx.QueryContext(ctx, `SELECT chunk_id,chunk_hash,chunk_json FROM chunks WHERE build_id=? ORDER BY ordinal`, buildID)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	result := make([]gen.Chunk, 0)
	for rows.Next() {
		var id, hash string
		var raw []byte
		var chunk gen.Chunk
		if err := rows.Scan(&id, &hash, &raw); err != nil {
			return nil, storageError(err)
		}
		if json.Unmarshal(raw, &chunk) != nil || chunk.ChunkId != id || chunk.ChunkHash != hash || platform.HashBytes([]byte(chunk.Text)) != hash || len(chunk.SourceOffsets) != len(chunk.Text) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "持久分块损坏")
		}
		result = append(result, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, storageError(err)
	}
	return result, nil
}

// GenerationEntry 是完整派生记录；只有词项与向量都存在才可用于激活。
type GenerationEntry struct {
	Chunk      gen.Chunk      `json:"chunk"`
	Terms      map[string]int `json:"terms"`
	Length     int            `json:"length"`
	Vector     []float64      `json:"vector"`
	VectorHash string         `json:"vectorHash"`
}

// ReadGeneration 在固定事务内核对各块对应的词项、空间及向量哈希。
func ReadGeneration(ctx context.Context, tx *sql.Tx, record BuildRecord) ([]GenerationEntry, string, error) {
	chunks, err := ReadChunks(ctx, tx, record.Build.BuildId)
	if err != nil {
		return nil, "", err
	}
	if len(chunks) != record.Build.TotalChunks {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "构建分块总数不一致")
	}
	var termsCount, vectorsCount int
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM terms WHERE build_id=?),(SELECT COUNT(*) FROM vectors WHERE build_id=?)`, record.Build.BuildId, record.Build.BuildId).Scan(&termsCount, &vectorsCount); err != nil {
		return nil, "", storageError(err)
	}
	if termsCount != len(chunks) || vectorsCount != len(chunks) {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "构建词项或向量数量不一致")
	}
	entries := make([]GenerationEntry, 0, len(chunks))
	for _, chunk := range chunks {
		var termsRaw, vectorRaw, spaceRaw []byte
		var language, tokenizer, chunkHash, vectorHash string
		var length int
		err := tx.QueryRowContext(ctx, `SELECT t.language,t.tokenizer_version,t.terms_json,t.length,v.chunk_hash,v.space_json,v.vector_json,v.vector_hash FROM terms t JOIN vectors v ON t.build_id=v.build_id AND t.chunk_id=v.chunk_id WHERE t.build_id=? AND t.chunk_id=?`, record.Build.BuildId, chunk.ChunkId).Scan(&language, &tokenizer, &termsRaw, &length, &chunkHash, &spaceRaw, &vectorRaw, &vectorHash)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, "", platform.NewError(platform.CodeIntegrityError, false, "构建缺少词项或向量")
			}
			return nil, "", storageError(err)
		}
		var terms map[string]int
		var space gen.EmbeddingSpace
		if language != record.Language || tokenizer != TokenizerVersion || chunkHash != chunk.ChunkHash || chunk.Document != record.Build.Document || json.Unmarshal(spaceRaw, &space) != nil || space != record.Build.Space || json.Unmarshal(termsRaw, &terms) != nil {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "派生索引来源或空间不一致")
		}
		count := 0
		for _, frequency := range terms {
			if frequency <= 0 {
				return nil, "", platform.NewError(platform.CodeIntegrityError, false, "词项频率不合法")
			}
			count += frequency
		}
		if count != length {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "词项长度不一致")
		}
		expected := make(map[string]int)
		for _, term := range TokenizeText(RetrievalText(chunk)) {
			expected[term]++
		}
		expectedRaw, _ := platform.CanonicalJSON(expected)
		actualRaw, _ := platform.CanonicalJSON(terms)
		if string(expectedRaw) != string(actualRaw) {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "词项与源分块不一致")
		}
		vector, err := DecodeVector(vectorRaw, vectorHash, space.Dimensions)
		if err != nil {
			return nil, "", err
		}
		entries = append(entries, GenerationEntry{Chunk: chunk, Terms: terms, Length: length, Vector: vector, VectorHash: vectorHash})
	}
	raw, err := platform.CanonicalJSON(entries)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "索引清单无法编码")
	}
	return entries, platform.HashBytes(raw), nil
}
