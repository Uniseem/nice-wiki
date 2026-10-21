package chunk

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"strings"
	"unicode/utf8"
)

type EnrichChunkContextInput struct {
	BuildID      string      `json:"buildId"`
	Chunks       []gen.Chunk `json:"chunks"`
	DocumentText string      `json:"documentText"`
	// Context 是给模型参考的“全文目录 + 所在章节原文”（见 PlanContextBatches），不再整篇发送。
	Context string                 `json:"context"`
	Memory  platform.MemoryContext `json:"memory"`
	LeaseID string                 `json:"leaseId"`
}
type EnrichChunkContextResult struct {
	Prefixes            []string `json:"prefixes"`
	ContextVersionHash  string   `json:"contextVersionHash"`
	PrefixContextHashes []string `json:"prefixContextHashes"`
	ModelSnapshotHashes []string `json:"modelSnapshotHashes"`
}

// ContextInstruction 是背景短句任务的说明。
const ContextInstruction = "为检索片段各写一至两句背景，说明所属产品、功能或小节。全文目录、所在章节原文与记忆仅作参考，保持原文语言。只返回JSON对象，格式为{\"items\":[{\"chunkId\":\"原ID\",\"prefix\":\"背景\"}]}。每个原ID必须恰好出现一次，不修改原文，不输出代码围栏。"

// EnrichChunkContext 为一批片段写背景短句；整篇原文只用来核对来源，发给模型的是这批片段所在章节的原文和全文目录。
func EnrichChunkContext(ctx context.Context, op OperationContext, input EnrichChunkContextInput) (EnrichChunkContextResult, error) {
	if op.CostClass != platform.CostClassBackground {
		return EnrichChunkContextResult{}, platform.NewError(platform.CodeForbidden, false, "发布背景仅允许原批准后台费用类别")
	}
	if len(input.Chunks) == 0 || len(input.Chunks) > contextBatchChunks {
		return EnrichChunkContextResult{}, platform.NewError(platform.CodeValidation, false, "上下文批次超预算")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	_, initial, record, closeInitial, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	_ = initial.Rollback()
	closeInitial()
	if platform.HashBytes([]byte(input.DocumentText)) != record.Build.Document.ContentHash || input.Memory.TaskID != "knowledge.context" || input.Memory.Language != record.Language || input.Memory.ByteSize != int64(len(input.Memory.Text)) {
		return EnrichChunkContextResult{}, platform.NewError(platform.CodeIntegrityError, false, "整篇固定来源或任务记忆不一致")
	}
	route, err := indexer.ReadTaskRoute(ctx, op, runtime, "knowledge.context")
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	profile := route.Profile
	assignment := route.Assignment
	snapshots := []string{}
	for _, model := range route.Models {
		snapshots = append(snapshots, model.ModelSnapshotHash)
	}
	if len(snapshots) == 0 {
		return EnrichChunkContextResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "上下文策略缺MG模型快照证明")
	}
	plan := ContextPlan{Document: record.Build.Document, Memory: input.Memory, Profile: profile, Assignment: assignment, ModelSnapshots: snapshots}
	if err = FreezeContextPlan(ctx, op, input.BuildID, input.LeaseID, plan); err != nil {
		return EnrichChunkContextResult{}, err
	}
	binding, _ := platform.CanonicalJSON(plan)
	result := EnrichChunkContextResult{Prefixes: make([]string, len(input.Chunks)), PrefixContextHashes: make([]string, len(input.Chunks)), ModelSnapshotHashes: make([]string, len(input.Chunks)), ContextVersionHash: platform.HashBytes(binding)}
	missing := []gen.Chunk{}
	seen := map[string]bool{}
	for i, value := range input.Chunks {
		if value.Document != record.Build.Document || seen[value.ChunkId] {
			return EnrichChunkContextResult{}, platform.NewError(platform.CodeIntegrityError, false, "上下文片段来源或身份冲突")
		}
		seen[value.ChunkId] = true
		for _, snapshot := range snapshots {
			contextHash := ContextResultHash(record.Build.Document.ContentHash, input.Memory, snapshot)
			var prefix, hash, evidence string
			err = runtime.DB.QueryRowContext(ctx, `SELECT prefix,prefix_hash,model_snapshot_hash FROM chunk_contexts WHERE site_id=? AND data_epoch=? AND context_hash=? AND source_hash=?`, op.Actor.SiteID, op.DataEpoch, contextHash, ContextSourceKey(value)).Scan(&prefix, &hash, &evidence)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return EnrichChunkContextResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原背景结果读取失败")
			}
			if evidence == "" {
				continue
			}
			if evidence != snapshot || platform.HashBytes([]byte(prefix)) != hash {
				return EnrichChunkContextResult{}, platform.NewError(platform.CodeIntegrityError, false, "已付费背景来源或摘要损坏")
			}
			result.Prefixes[i] = prefix
			result.PrefixContextHashes[i] = contextHash
			result.ModelSnapshotHashes[i] = snapshot
			break
		}
		if result.Prefixes[i] == "" {
			missing = append(missing, value)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}
	requestChunks := make([]struct {
		ID   string `json:"chunkId"`
		Text string `json:"text"`
	}, len(missing))
	for i, value := range missing {
		requestChunks[i].ID = value.ChunkId
		requestChunks[i].Text = value.Text
	}
	batch, _ := json.Marshal(requestChunks)
	if utf8.RuneCount(batch) > 65536 {
		return EnrichChunkContextResult{}, platform.NewError(platform.CodeValidation, false, "上下文片段消息超过固定预算")
	}
	claimWork, claim, _, releaseClaim, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	claimErr := ClaimContextIntents(claimWork, claim, op, input.BuildID, result.ContextVersionHash, missing)
	if claimErr == nil {
		claimErr = claim.Commit()
	}
	_ = claim.Rollback()
	releaseClaim()
	if claimErr != nil {
		return EnrichChunkContextResult{}, claimErr
	}
	ctx, err = modelruntime.WithModelOwner(ctx, input.BuildID)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	// 每条要回带约 70 字符的块 ID 加一两句背景，按块数给输出上限；固定 500 时十几块就会被截断成半截 JSON。
	maximumOutput := int64(200 + 160*len(missing))
	step, _ := platform.CanonicalJSON([]any{result.ContextVersionHash, requestChunks, maximumOutput, platform.HashBytes([]byte(input.Context))})
	messages := []platform.GatewayMessage{{Role: "system", Content: ContextInstruction}}
	if input.Memory.Text != "" {
		messages = append(messages, platform.GatewayMessage{Role: "system", Content: input.Memory.Text})
	}
	messages = append(messages, platform.GatewayMessage{Role: "user", Content: input.Context}, platform.GatewayMessage{Role: "user", Content: string(batch)})
	outcome, err := modelruntime.InvokeModel(ctx, op, modelruntime.InvokeModelInput{SemanticKey: ContextSemanticKey(input.Chunks), Request: platform.CreateInvocationInput{ExpectedRouteVersion: route.RouteVersion, TaskID: "knowledge.context", ProfileID: profile.ProfileID, Capability: platform.CapabilityChat, MaximumOutputTokens: maximumOutput, IDempotencyKey: "context." + platform.HashBytes(step), Messages: messages}})
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	snapshot, err := modelruntime.WinningModelSnapshot(outcome)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	actualContextHash := ContextResultHash(record.Build.Document.ContentHash, input.Memory, snapshot)
	prefixes, err := ReadContextPrefixes(outcome, missing)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	work, tx, _, cancel, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return EnrichChunkContextResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	for _, value := range missing {
		prefix := prefixes[value.ChunkId]
		_, err = tx.ExecContext(work, `INSERT INTO chunk_contexts(site_id,data_epoch,context_hash,source_hash,chunk_hash,prefix,prefix_hash,invocation_id,created_at,model_snapshot_hash) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, op.Actor.SiteID, op.DataEpoch, actualContextHash, ContextSourceKey(value), value.ChunkHash, prefix, platform.HashBytes([]byte(prefix)), outcome.InvocationID, platform.Now(), snapshot)
		if err != nil {
			return EnrichChunkContextResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "机器背景保存失败")
		}
		var saved, savedHash, evidence string
		err = tx.QueryRowContext(work, `SELECT prefix,prefix_hash,model_snapshot_hash FROM chunk_contexts WHERE site_id=? AND data_epoch=? AND context_hash=? AND source_hash=?`, op.Actor.SiteID, op.DataEpoch, actualContextHash, ContextSourceKey(value)).Scan(&saved, &savedHash, &evidence)
		if err != nil || savedHash != platform.HashBytes([]byte(saved)) || evidence != snapshot {
			return EnrichChunkContextResult{}, platform.NewError(platform.CodeIntegrityError, false, "背景并发原结果或来源不一致")
		}
		prefixes[value.ChunkId] = saved
	}
	if err = tx.Commit(); err != nil {
		return EnrichChunkContextResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "机器背景提交失败")
	}
	for i, value := range input.Chunks {
		if result.Prefixes[i] == "" {
			result.Prefixes[i] = prefixes[value.ChunkId]
			result.PrefixContextHashes[i] = actualContextHash
			result.ModelSnapshotHashes[i] = snapshot
		}
	}
	return result, nil
}

// ReadContextPrefixes 严格匹配原ID和完整输出，机器背景不会充作原引文。
func ReadContextPrefixes(outcome modelruntime.InvokeModelResult, chunks []gen.Chunk) (map[string]string, error) {
	if !modelruntime.HasCompleteResult(outcome) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "背景模型未完成，保留原调用")
	}
	text, err := modelruntime.CompletedText(outcome, 65536)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Items []struct {
			ID     string `json:"chunkId"`
			Prefix string `json:"prefix"`
		} `json:"items"`
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&parsed) != nil || decoder.Decode(&struct{}{}) != io.EOF || len(parsed.Items) != len(chunks) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "背景模型输出结构或数量不一致")
	}
	allowed := map[string]bool{}
	for _, value := range chunks {
		allowed[value.ChunkId] = true
	}
	result := map[string]string{}
	for _, item := range parsed.Items {
		if !allowed[item.ID] || result[item.ID] != "" || !utf8.ValidString(item.Prefix) || strings.TrimSpace(item.Prefix) == "" || len(item.Prefix) > 4096 || strings.ContainsAny(item.Prefix, "\r\n\x00") {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "背景模型身份或短句不合法")
		}
		sentences := 0
		inSentence := false
		for _, r := range item.Prefix {
			if strings.ContainsRune("。！？.!?", r) {
				if inSentence {
					sentences++
					inSentence = false
				}
			} else if r != ' ' && r != '\t' {
				inSentence = true
			}
		}
		if inSentence {
			sentences++
		}
		if sentences < 1 || sentences > 2 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "背景模型必须返回一至两句")
		}
		result[item.ID] = item.Prefix
	}
	return result, nil
}

// ContextSourceKey 保留真实原块/路径/区间语境，完整源hash相同的重发布可复用，重复短句不串背景。
func ContextSourceKey(value gen.Chunk) string {
	raw, _ := platform.CanonicalJSON([]any{value.Block.BlockId, value.Block.BlockHash, value.SourcePath, value.ByteStart, value.ByteEnd, value.ChunkHash})
	return platform.HashBytes(raw)
}

// ClaimContextIntents 外部付费前同事务占原来源意图，其他构建只等待原已付费结果，不重复花费。
func ClaimContextIntents(ctx context.Context, tx *sql.Tx, op OperationContext, buildID, contextHash string, chunks []gen.Chunk) error {
	for _, value := range chunks {
		sourceHash := ContextSourceKey(value)
		_, err := tx.ExecContext(ctx, `INSERT INTO chunk_context_intents(site_id,data_epoch,context_hash,source_hash,build_id,root_operation_id,operation_id,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, op.Actor.SiteID, op.DataEpoch, contextHash, sourceHash, buildID, op.RootOperationID, op.OperationID, platform.Now())
		if err != nil {
			return platform.NormalizeIOError(err, platform.CodeInternal, false, "背景原意图保存失败")
		}
		var owner, root, operation string
		err = tx.QueryRowContext(ctx, `SELECT build_id,root_operation_id,operation_id FROM chunk_context_intents WHERE site_id=? AND data_epoch=? AND context_hash=? AND source_hash=?`, op.Actor.SiteID, op.DataEpoch, contextHash, sourceHash).Scan(&owner, &root, &operation)
		if err != nil {
			return platform.NormalizeIOError(err, platform.CodeInternal, false, "背景原意图读取失败")
		}
		if owner != buildID || root != op.RootOperationID || operation != op.OperationID {
			// 计费暂停时，原意图所属构建已失败或取消就由本构建接手，不再永远等一个不会来的结果。
			var state string
			if !platform.BillingEnabled && tx.QueryRowContext(ctx, `SELECT state FROM builds WHERE build_id=? AND site_id=? AND data_epoch=?`, owner, op.Actor.SiteID, op.DataEpoch).Scan(&state) == nil && (state == "failed" || state == "cancelled") {
				if _, err = tx.ExecContext(ctx, `UPDATE chunk_context_intents SET build_id=?,root_operation_id=?,operation_id=? WHERE site_id=? AND data_epoch=? AND context_hash=? AND source_hash=? AND build_id=?`, buildID, op.RootOperationID, op.OperationID, op.Actor.SiteID, op.DataEpoch, contextHash, sourceHash, owner); err != nil {
					return platform.NormalizeIOError(err, platform.CodeInternal, false, "背景原意图接手失败")
				}
				continue
			}
			return platform.NewError(platform.CodeDependencyUnavailable, false, "同源背景已有原付费意图，等待原结果")
		}
	}
	return nil
}

// ApplyChunkContext 将机器前缀单独保存，完整检索文本重新真实BPE；原text/hash/证据偏移保持。
func ApplyChunkContext(ctx context.Context, op OperationContext, chunks []gen.Chunk, context EnrichChunkContextResult, modelID string, space gen.EmbeddingSpace) ([]gen.Chunk, error) {
	if len(chunks) != len(context.Prefixes) || len(chunks) != len(context.PrefixContextHashes) || len(chunks) != len(context.ModelSnapshotHashes) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "机器背景数量与原片段不一致")
	}
	result := append([]gen.Chunk{}, chunks...)
	for i := range result {
		prefix := context.Prefixes[i]
		contextHash := context.PrefixContextHashes[i]
		snapshot := context.ModelSnapshotHashes[i]
		result[i].ContextPrefix = &prefix
		result[i].ContextVersionHash = &contextHash
		result[i].ContextModelSnapshotHash = &snapshot
		tokenized, err := modelruntime.TokenizeSource(ctx, op, modelruntime.TokenizeSourceInput{ModelID: modelID, Space: space, Text: indexer.RetrievalText(result[i])})
		if err != nil {
			return nil, err
		}
		count := tokenized.TokenCount
		result[i].EmbeddingTokenCount = &count
	}
	return result, nil
}

// ContextSemanticKey 只含固定源片段身份，不让模型或记忆更新绕过旧调用。
func ContextSemanticKey(chunks []gen.Chunk) string {
	ids := make([]string, len(chunks))
	for i, value := range chunks {
		ids[i] = value.ChunkId
	}
	raw, _ := platform.CanonicalJSON(ids)
	return "context." + platform.HashBytes(raw)
}

type ContextPlan struct {
	Document       gen.PublishedRevisionRef
	Memory         platform.MemoryContext
	Profile        indexer.ProfileDeclaration
	Assignment     *indexer.TaskAssignment
	ModelSnapshots []string
}

// FreezeContextPlan 首次完整任务记忆/档位持久固定；重试漂移在认领新付费意图前拒绝。
func FreezeContextPlan(ctx context.Context, op OperationContext, buildID, leaseID string, plan ContextPlan) error {
	raw, err := platform.CanonicalJSON(plan)
	if err != nil {
		return err
	}
	work, tx, _, cancel, err := indexer.BeginBuildWrite(ctx, op, buildID, leaseID)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	var original []byte
	var hash string
	err = tx.QueryRowContext(work, `SELECT plan_json,plan_hash FROM build_context_plans WHERE build_id=?`, buildID).Scan(&original, &hash)
	if err == nil {
		if hash != platform.HashBytes(original) {
			return platform.NewError(platform.CodeIntegrityError, false, "原上下文任务计划损坏")
		}
		if hash != platform.HashBytes(raw) {
			return platform.NewError(platform.CodeConflict, false, "同构建上下文记忆或模型计划已变化")
		}
	} else if err == sql.ErrNoRows {
		_, err = tx.ExecContext(work, `INSERT INTO build_context_plans(build_id,plan_json,plan_hash,created_at) VALUES(?,?,?,?)`, buildID, raw, platform.HashBytes(raw), platform.Now())
	} else {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原上下文计划读取失败")
	}
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "上下文原计划保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "上下文原计划提交失败")
	}
	return nil
}

// ContextResultHash 以MG实际winning快照/整篇源/真实记忆版本标记机器背景，不计算MG快照算法。
func ContextResultHash(documentHash string, memory platform.MemoryContext, snapshot string) string {
	raw, _ := platform.CanonicalJSON([]any{"knowledge.context.result.v2", documentHash, memory.VersionHash, memory.MappingRevision, snapshot})
	return platform.HashBytes(raw)
}
