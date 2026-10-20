package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/admin"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"time"
)

type CreateBuildInput = gen.CreateBuildInput
type CreateBuildResult = gen.CreateBuildResult

type BuildJobInput struct {
	BuildID  string                   `json:"buildId"`
	Document gen.PublishedRevisionRef `json:"document"`
	Config   gen.IndexConfig          `json:"config"`
}

// CreateBuild 确认当前公开修订后持久接受构建；待派发意图与原回执同事务。
func CreateBuild(ctx context.Context, op OperationContext, input CreateBuildInput) (CreateBuildResult, error) {
	if err := admin.AuthorizeAdmin(op); err != nil {
		return CreateBuildResult{}, err
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return CreateBuildResult{}, err
	}
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return CreateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建原回执读取失败")
	}
	var original CreateBuildResult
	found, err := indexer.ReadOperationReceipt(ctx, read, op, "CreateBuild", input.IdempotencyKey, input, &original)
	_ = read.Rollback()
	if err != nil {
		return CreateBuildResult{}, err
	}
	if found {
		return original, nil
	}
	if runtime.Content == nil || runtime.ValidatePublicationResult == nil {
		return CreateBuildResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "发布权威未装配")
	}
	observed, visibility, err := indexer.ValidatePublication(ctx, op, runtime.Content, runtime.ValidatePublicationResult, input.Document)
	if err != nil {
		return CreateBuildResult{}, err
	}
	work, tx, cancel, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return CreateBuildResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	found, err = indexer.ReadOperationReceipt(work, tx, op, "CreateBuild", input.IdempotencyKey, input, &original)
	if err != nil {
		return CreateBuildResult{}, err
	}
	if found {
		return original, nil
	}
	if time.Since(observed) > time.Second {
		return CreateBuildResult{}, platform.NewError(platform.CodeStaleResult, false, "构建创建前权威证明已过期")
	}
	config, err := indexer.ReadConfig(work, tx, op)
	if err != nil {
		return CreateBuildResult{}, err
	}
	if !config.Enabled {
		return CreateBuildResult{}, platform.NewError(platform.CodeValidation, false, "知识入库未启用")
	}
	var known int
	var withdrawn int
	err = tx.QueryRowContext(work, `SELECT visibility_revision,withdrawn FROM document_watermarks WHERE site_id=? AND data_epoch=? AND document_id=?`, op.Actor.SiteID, op.DataEpoch, input.Document.DocumentId).Scan(&known, &withdrawn)
	if err != nil && err != sql.ErrNoRows {
		return CreateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "发布水位读取失败")
	}
	if known > visibility || (known == visibility && withdrawn != 0) {
		return CreateBuildResult{}, platform.NewError(platform.CodeStaleResult, false, "发布证明已被本域撤回水位替代")
	}
	build, err := QueueBuild(work, tx, op, config, input.Document, input.Force)
	if err != nil {
		return CreateBuildResult{}, err
	}
	result := CreateBuildResult{Build: build}
	if err = indexer.SaveOperationReceipt(work, tx, op, "CreateBuild", input.IdempotencyKey, input, result); err != nil {
		return CreateBuildResult{}, err
	}
	if _, err = tx.ExecContext(work, `INSERT INTO document_watermarks(site_id,data_epoch,document_id,visibility_revision,withdrawn) VALUES(?,?,?,?,0) ON CONFLICT(site_id,data_epoch,document_id) DO UPDATE SET visibility_revision=excluded.visibility_revision,withdrawn=0 WHERE document_watermarks.visibility_revision<=excluded.visibility_revision`, op.Actor.SiteID, op.DataEpoch, input.Document.DocumentId, visibility); err != nil {
		return CreateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "发布创建水位保存失败")
	}
	if err = tx.Commit(); err != nil {
		return CreateBuildResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建接受提交失败")
	}
	// 202只表示本域持久接受；jobs不可达时意图仍在，由恢复批次继续同一去重键。
	_ = DispatchBuild(ctx, op, build.BuildId)
	return result, nil
}

// QueueBuild 只在已授权创建或已验原事件的外层事务使用，不直接调用jobs。
func QueueBuild(ctx context.Context, tx *sql.Tx, op OperationContext, config gen.IndexConfig, document gen.PublishedRevisionRef, force bool) (gen.IndexBuild, error) {
	if op.CostClass != platform.CostClassBackground {
		return gen.IndexBuild{}, platform.NewError(platform.CodeForbidden, false, "入库仅允许可信后台费用类别")
	}
	documentRaw, _ := platform.CanonicalJSON(document)
	spaceRaw, _ := platform.CanonicalJSON(config.Space)
	if !force {
		var id string
		err := tx.QueryRowContext(ctx, `SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=? AND document_json=? ORDER BY generation DESC LIMIT 1`, op.Actor.SiteID, op.DataEpoch, document.DocumentId, config.Space.SpaceId, documentRaw).Scan(&id)
		if err == nil {
			record, err := indexer.ReadBuildRecord(ctx, tx, op, id)
			if err != nil {
				return gen.IndexBuild{}, err
			}
			return record.Build, nil
		}
		if err != sql.ErrNoRows {
			return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建去重读取失败")
		}
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return gen.IndexBuild{}, err
	}
	if err = indexer.EnsureBuildDiskBudget(runtime.DataRoot); err != nil {
		return gen.IndexBuild{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO generation_watermarks(site_id,data_epoch,document_id,space_id,generation) SELECT ?,?,?,?,COALESCE(MAX(generation),0) FROM builds WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=? ON CONFLICT(site_id,data_epoch,document_id,space_id) DO NOTHING`, op.Actor.SiteID, op.DataEpoch, document.DocumentId, config.Space.SpaceId, op.Actor.SiteID, op.DataEpoch, document.DocumentId, config.Space.SpaceId); err != nil {
		return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建代数高水位建立失败")
	}
	var generation int
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM generation_watermarks WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=?`, op.Actor.SiteID, op.DataEpoch, document.DocumentId, config.Space.SpaceId).Scan(&generation); err != nil {
		return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建代数读取失败")
	}
	if generation == 9007199254740991 {
		return gen.IndexBuild{}, platform.NewError(platform.CodeVersionMismatch, false, "构建代数已达上限")
	}
	generation++
	if _, err := tx.ExecContext(ctx, `UPDATE generation_watermarks SET generation=? WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=?`, generation, op.Actor.SiteID, op.DataEpoch, document.DocumentId, config.Space.SpaceId); err != nil {
		return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建代数保存失败")
	}
	result := gen.IndexBuild{BuildId: platform.NewIdentifier(), Document: document, Space: config.Space, State: gen.IndexBuildState("queued"), Generation: generation, CompletedChunks: 0, TotalChunks: 0}
	configRaw, _ := platform.CanonicalJSON(config)
	_, err = tx.ExecContext(ctx, `INSERT INTO builds(build_id,site_id,data_epoch,document_id,document_json,space_json,space_id,language,embedding_profile_id,state,generation,config_json,created_at,workflow_version) VALUES(?,?,?,?,?,?,?,?,?,'queued',?,?,?,1)`, result.BuildId, op.Actor.SiteID, op.DataEpoch, document.DocumentId, documentRaw, spaceRaw, config.Space.SpaceId, config.Language, config.EmbeddingProfileId, generation, configRaw, platform.Now())
	if err != nil {
		return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建记录创建失败")
	}
	payload, _ := platform.CanonicalJSON(BuildJobInput{result.BuildId, document, config})
	ref := platform.ObjectRef{Owner: "knowledge", ObjectID: result.BuildId, Version: int64(generation), SHA256: platform.HashBytes(payload), MediaType: "application/json", ByteSize: int64(len(payload))}
	refRaw, _ := json.Marshal(ref)
	// 持久批准只存元数据，不存portal委托证明、cookie或模型凭证；worker稍后从本域事实建立服务身份。
	durable := op
	durable.LoginNetworkScopes = nil
	durable.NetworkBudgetScopes = nil
	durable.Deadline = time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339Nano)
	dedupe := "knowledge.build." + result.BuildId
	// jobs 以幂等键作为任务的操作号，持久批准里的操作号必须与之相同，领取时才对得上。
	durable.OperationID = dedupe
	operationRaw, _ := json.Marshal(durable)
	_, err = tx.ExecContext(ctx, `INSERT INTO build_dispatches(build_id,input_json,input_ref_json,operation_json,dedupe_key,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, result.BuildId, payload, refRaw, operationRaw, dedupe, platform.Now(), platform.Now())
	if err != nil {
		return gen.IndexBuild{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "构建派发意图保存失败")
	}
	return result, nil
}

// DispatchBuild 使用持久原inputRef/去重键；网络调用前后均不持有SQL事务。
func DispatchBuild(ctx context.Context, op OperationContext, buildID string) error {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return err
	}
	if runtime.Jobs == nil || runtime.Validators["JobsCreateJobResult"] == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "任务生产方未装配")
	}
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "派发意图读取失败")
	}
	record, err := indexer.ReadBuildRecord(ctx, read, op, buildID)
	if err != nil {
		_ = read.Rollback()
		return err
	}
	var refRaw, operationRaw, payloadRaw []byte
	var key, state string
	var cancelled int
	err = read.QueryRowContext(ctx, `SELECT input_ref_json,operation_json,input_json,dedupe_key,dispatch_state,cancel_requested FROM build_dispatches WHERE build_id=?`, buildID).Scan(&refRaw, &operationRaw, &payloadRaw, &key, &state, &cancelled)
	_ = read.Rollback()
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "派发原引用读取失败")
	}
	if (state != "pending" && state != "dispatching") || cancelled != 0 || string(record.Build.State) != "queued" {
		return nil
	}
	var ref platform.ObjectRef
	var original OperationContext
	if json.Unmarshal(refRaw, &ref) != nil || json.Unmarshal(operationRaw, &original) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "派发原身份或引用损坏")
	}
	var payload BuildJobInput
	if json.Unmarshal(payloadRaw, &payload) != nil || ref.Owner != "knowledge" || ref.ObjectID != buildID || ref.Version != int64(record.Build.Generation) || ref.MediaType != "application/json" || ref.ByteSize != int64(len(payloadRaw)) || ref.SHA256 != platform.HashBytes(payloadRaw) || payload.BuildID != buildID || payload.Document != record.Build.Document || payload.Config.Space != record.Build.Space || original.RootOperationID == "" || original.DataEpoch != op.DataEpoch || original.Actor.SiteID != op.Actor.SiteID || original.CostClass != platform.CostClassBackground {
		return platform.NewError(platform.CodeIntegrityError, false, "派发原输入与完整身份不一致")
	}
	service := original
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: original.Deadline, Grants: []string{}}
	claimWork, claim, closeClaim, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return err
	}
	claimed, err := claim.ExecContext(claimWork, `UPDATE build_dispatches SET dispatch_state='dispatching',updated_at=? WHERE build_id=? AND dispatch_state IN ('pending','dispatching') AND cancel_requested=0`, platform.Now(), buildID)
	if err != nil {
		_ = claim.Rollback()
		closeClaim()
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "派发资格保存失败")
	}
	n, err := claimed.RowsAffected()
	if err != nil {
		_ = claim.Rollback()
		closeClaim()
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "派发资格检查失败")
	}
	if n == 0 {
		_ = claim.Rollback()
		closeClaim()
		return nil
	}
	if err = claim.Commit(); err != nil {
		closeClaim()
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "派发资格提交失败")
	}
	closeClaim()
	raw, err := runtime.Jobs.CreateJob(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service), struct {
		Owner     string             `json:"owner"`
		Type      string             `json:"type"`
		InputRef  platform.ObjectRef `json:"inputRef"`
		DedupeKey string             `json:"dedupeKey"`
	}{"knowledge", "knowledge.build", ref, key}, key)
	if err != nil {
		return err
	}
	if err = runtime.Validators["JobsCreateJobResult"](raw); err != nil {
		return err
	}
	var accepted struct {
		Job struct {
			JobID       string `json:"jobId"`
			OperationID string `json:"operationId"`
			State       string `json:"state"`
		} `json:"job"`
	}
	if json.Unmarshal(raw, &accepted) != nil || accepted.Job.State != "queued" {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "任务接受回执不可读")
	}
	work, tx, cancel, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	// 取消可能在RPC期间发生，仍保存真实jobId，后续取消批次能够清理，不能丢弃已创建任务引用。
	if _, err = tx.ExecContext(work, `UPDATE build_dispatches SET job_id=?,jobs_operation_id=?,dispatch_state='dispatched',updated_at=? WHERE build_id=? AND dispatch_state='dispatching'`, accepted.Job.JobID, accepted.Job.OperationID, platform.Now(), buildID); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "任务接受引用保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "任务派发保存失败")
	}
	return nil
}
