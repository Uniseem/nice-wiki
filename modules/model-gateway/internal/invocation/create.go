package invocation

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/provider"
	"nice-wiki/modules/model-gateway/internal/routing"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
	"time"
)

type CreateInvocationInput = gen.CreateInvocationInput
type CreateInvocationResult = gen.CreateInvocationResult

func CreateInvocation(ctx context.Context, op OperationContext, input CreateInvocationInput) (CreateInvocationResult, error) {
	task, exists := gen.AITaskCatalog[input.TaskId]
	if !exists || task.Owner != op.CallerService || op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService {
		return CreateInvocationResult{}, platform.NewError(platform.CodeForbidden, false, "模型任务只允许目录owner纯服务执行")
	}
	if op.CostClass != platform.CostClassQA && op.CostClass != platform.CostClassBackground && !(input.TaskId == "memory.assistant" && op.CallerService == "memory" && op.CostClass == platform.CostClassAdminProbe) {
		return CreateInvocationResult{}, platform.NewError(platform.CodeForbidden, false, "公开模型创建仅原QA/后台或已批准memory.assistant费用链")
	}
	if input.TaskId != "localization.suggest" && input.LanguageSuggestionApprovalId != nil {
		return CreateInvocationResult{}, platform.NewError(platform.CodeValidation, false, "语言建议批准不能用于其他任务")
	}
	if err := ValidateInvocationPayload(input); err != nil {
		return CreateInvocationResult{}, err
	}
	options, err := gatewayruntime.ExecutionDependencies()
	if err != nil {
		return CreateInvocationResult{}, err
	}
	if options.SiteID == "" || op.Actor.SiteID != options.SiteID {
		return CreateInvocationResult{}, platform.NewError(platform.CodeForbidden, false, "新调用必须绑定当前实际站点")
	}
	canonical := CanonicalInvocationInput(op, input)
	read, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return CreateInvocationResult{}, err
	}
	var result CreateInvocationResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(read, op, "model-gateway.CreateInvocation", input.IdempotencyKey, canonical, &result)
	if err != nil {
		read.Close()
		return result, err
	}
	if replay {
		record, err := gatewayruntime.ReadInvocationRecord(read, op, result.Invocation.InvocationId)
		read.Close()
		if err != nil {
			return result, err
		}
		result.Invocation = record.Invocation
		if record.Invocation.State == "accepted" || record.Invocation.State == "reserved" {
			if _, err := StartAcceptedInvocation(ctx, options, record); err != nil {
				return CreateInvocationResult{}, err
			}
		}
		return result, nil
	}
	if err := RequireInvocationLookupTracking(read, op); err != nil {
		read.Close()
		return result, err
	}
	read.Close()
	if err := gatewayruntime.CheckLanguageSuggestionApproval(ctx, op, input); err != nil {
		return result, err
	}
	if options.Tasks == nil {
		return CreateInvocationResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型调用任务组未装配")
	}
	var binding *gatewayruntime.FeeScopeBinding
	if op.CostClass == platform.CostClassQA {
		if input.AdmissionPermit == nil || input.AdmissionPermit.Token == "" {
			return result, platform.NewError(platform.CodeForbidden, false, "QA模型消费缺少guard已消费签名")
		}
		verified, err := serviceauth.VerifyAdmissionPermit(options.Anchors, input.AdmissionPermit.Token)
		if err != nil {
			return result, err
		}
		raw, _ := platform.CanonicalJSON(input.AdmissionPermit)
		actual, _ := platform.CanonicalJSON(verified)
		if string(raw) != string(actual) || verified.RequestID != op.RootOperationID || verified.DataEpoch != op.DataEpoch || (verified.Action != "qa.ask" && verified.Action != "qa.followup") {
			return result, platform.NewError(platform.CodeForbidden, false, "QA原根、代际或付费动作与签名不一致")
		}
		expiry, _ := platform.ParseTimestamp(verified.ExpiresAt)
		binding = &gatewayruntime.FeeScopeBinding{RootOperationID: op.RootOperationID, AdmissionID: verified.AdmissionID, SessionBudgetScope: verified.SessionBudgetScope, NetworkBudgetScope: verified.NetworkBudgetScope, DataEpoch: op.DataEpoch, ExpiresAt: expiry}
	} else if input.AdmissionPermit != nil {
		return result, platform.NewError(platform.CodeForbidden, false, "后台费用不能冒用QA访客准入")
	}
	route, err := routing.ResolveRoute(ctx, op, routing.ResolveRouteInput{TaskId: input.TaskId, CostClass: op.CostClass})
	if err != nil {
		return result, err
	}
	if route.Resolved.Profile.ProfileId != input.ProfileId {
		return result, platform.NewError(platform.CodeStaleResult, false, "模型档位与当前任务固定路线不一致")
	}
	capability := string(input.Capability)
	if capability == "translation" {
		capability = "chat"
	}
	if !slices.Contains(route.Resolved.RequiredCapabilities, capability) {
		return result, platform.NewError(platform.CodeValidation, false, "请求能力不属于任务当前路线")
	}
	versions, err := provider.AuthorizeImages(ctx, op, input.Messages)
	if err != nil {
		return result, err
	}
	fixed := gatewayruntime.FixedRoute{Version: route.RouteVersion, Assignment: &route.Resolved.Assignment, RequiredCapabilities: route.Resolved.RequiredCapabilities, Fallback: route.Resolved.Fallback, AssetVersions: versions, Candidates: make([]gatewayruntime.FixedCandidate, 0, len(route.Candidates))}
	for index, candidate := range route.Candidates {
		if index == 0 && input.ExpectedEmbeddingSpace != nil {
			space, err := catalog.ModelEmbeddingSpace(candidate.Model)
			if err != nil {
				return result, err
			}
			expected := input.ExpectedEmbeddingSpace
			if expected.SpaceId != space.SpaceID || expected.Provider != space.Provider || expected.ModelVersion != space.ModelVersion || expected.Dimensions != space.Dimensions || expected.PreprocessingVersion != space.PreprocessingVersion {
				return result, platform.NewError(platform.CodeStaleResult, false, "接受前嵌入主模型空间与请求固定空间不一致")
			}
		}
		price, err := billing.ReadPriceSnapshot(ctx, op, candidate.Model.ModelId, time.Now().UTC(), true)
		if err != nil {
			return result, err
		}
		fixed.Candidates = append(fixed.Candidates, gatewayruntime.FixedCandidate{Model: candidate.Model, CredentialVersion: candidate.CredentialVersion, Price: price})
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return result, err
	}
	defer tx.Close()
	replay, err = gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.CreateInvocation", input.IdempotencyKey, canonical, &result)
	if err != nil {
		return result, err
	}
	if replay {
		record, err := gatewayruntime.ReadInvocationRecord(tx, op, result.Invocation.InvocationId)
		tx.Close()
		if err != nil {
			return result, err
		}
		if record.Invocation.State == "accepted" || record.Invocation.State == "reserved" {
			if _, err := StartAcceptedInvocation(ctx, options, record); err != nil {
				return CreateInvocationResult{}, err
			}
		}
		result.Invocation = record.Invocation
		return result, nil
	}
	if err := RequireInvocationLookupTracking(tx, op); err != nil {
		return result, err
	}
	currentRoute, err := catalog.SelectCandidatesInTransaction(tx.Context, op, tx.Tx, routing.ResolveRouteInput{TaskId: input.TaskId, CostClass: op.CostClass})
	if err != nil {
		return result, err
	}
	if currentRoute.RouteVersion != route.RouteVersion || (input.ExpectedRouteVersion != nil && *input.ExpectedRouteVersion != currentRoute.RouteVersion) {
		return result, platform.NewError(platform.CodeStaleResult, false, "实际接受事务完整路线不同于原固定/批准路线")
	}
	if input.ExpectedPriceSnapshots != nil {
		expected := *input.ExpectedPriceSnapshots
		if len(expected) != len(route.Candidates) {
			return result, platform.NewError(platform.CodeStaleResult, false, "批准报价未完整覆盖实际有序候选")
		}
		seen := map[string]bool{}
		for i, item := range expected {
			if seen[item.ModelId] {
				return result, platform.NewError(platform.CodeValidation, false, "批准报价模型身份不能重复")
			}
			seen[item.ModelId] = true
			if item.ModelId != route.Candidates[i].Model.ModelId || item.PriceSnapshotId != fixed.Candidates[i].Price.PriceSnapshotId {
				return result, platform.NewError(platform.CodeStaleResult, false, "批准模型/价目与实际候选顺序不同")
			}
		}
	}
	var profileVersion, assignmentVersion int
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT version FROM profiles WHERE profile_id=? AND data_epoch=?`, route.Resolved.Profile.ProfileId, op.DataEpoch).Scan(&profileVersion); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT version FROM task_assignments WHERE task_id=? AND data_epoch=?`, input.TaskId, op.DataEpoch).Scan(&assignmentVersion); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if profileVersion != route.Resolved.Profile.Version || assignmentVersion != route.Resolved.Assignment.Version {
		return result, platform.NewError(platform.CodeStaleResult, false, "模型路线读取期间配置已变化")
	}
	for index, candidate := range route.Candidates {
		price, e := billing.ReadPriceSnapshotInTransaction(tx.Context, op, tx.Tx, candidate.Model.ModelId, time.Now().UTC(), true)
		if e != nil {
			return result, e
		}
		if price.PriceSnapshotId != fixed.Candidates[index].Price.PriceSnapshotId {
			return result, platform.NewError(platform.CodeStaleResult, false, "接受事务实际价目已变化，不能挪用旧报价")
		}
	}
	for index, candidate := range route.Candidates {
		var modelVersion int
		if err := tx.Tx.QueryRowContext(tx.Context, `SELECT version FROM models WHERE model_id=? AND data_epoch=?`, candidate.Model.ModelId, op.DataEpoch).Scan(&modelVersion); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		provider, err := ReadDispatchProvider(tx, op, candidate.Model.ProviderId)
		if err != nil {
			return result, err
		}
		if modelVersion != candidate.Model.Version || provider.Version != candidate.ProviderVersion || provider.CredentialVersion != candidate.CredentialVersion {
			return result, platform.NewError(platform.CodeStaleResult, false, "模型/渠道/凭据在固定路线期间变化")
		}
		if index == 0 {
			if err := ValidateInputModel(input, candidate.Model, string(provider.Adapter)); err != nil {
				return result, err
			}
		}
		fixed.Candidates[index].Provider = provider
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "可信原调用截止损坏")
	}
	now := time.Now().UTC()
	id := platform.NewIdentifier()
	if binding != nil {
		binding.InvocationID = id
	}
	invocation := gen.Invocation{InvocationId: id, RootOperationId: op.RootOperationID, CostClass: gen.CostClass(op.CostClass), State: gen.InvocationState("accepted"), Deadline: deadline, PriceSnapshotId: fixed.Candidates[0].Price.PriceSnapshotId, Attempts: make([]gen.Attempt, 0)}
	record := gatewayruntime.InvocationRecord{Invocation: invocation, Request: input, Operation: op, Profile: route.Resolved.Profile, Binding: binding, CreatedNS: now.UnixNano()}
	record.Route, _ = platform.CanonicalJSON(fixed)
	requestRaw, _ := platform.CanonicalJSON(input)
	profileRaw, _ := platform.CanonicalJSON(record.Profile)
	return AcceptPreparedInvocation(ctx, op, tx, record, fixed, requestRaw, profileRaw, "model-gateway.CreateInvocation", input.IdempotencyKey, canonical, options)
}

// AcceptPreparedInvocation共享已批准调用的原子接受与本进程任务启动；入口权限仍由各自调用方完成。
func AcceptPreparedInvocation(ctx context.Context, op OperationContext, tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, fixed gatewayruntime.FixedRoute, requestRaw []byte, profileRaw []byte, function string, key string, canonical any, options gatewayruntime.ExecutionOptions) (CreateInvocationResult, error) {
	for _, candidate := range fixed.Candidates {
		if err := gatewayruntime.CheckProviderMaterial(ctx, tx.Tx, op, candidate.Provider); err != nil {
			return CreateInvocationResult{}, err
		}
	}
	invocation := record.Invocation
	id := invocation.InvocationId
	deadline := time.Time(invocation.Deadline)
	now := time.Unix(0, record.CreatedNS).UTC()
	var result CreateInvocationResult
	raw, _ := platform.CanonicalJSON(invocation)
	operationRaw, _ := platform.CanonicalJSON(op)
	priceRaw, _ := platform.CanonicalJSON(fixed.Candidates[0].Price)
	var bindingRaw any
	if record.Binding != nil {
		bindingRaw, _ = platform.CanonicalJSON(record.Binding)
	}
	requestHash, _ := platform.CanonicalJSON(canonical)
	_, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO invocations(invocation_id,request_hash,root_operation_id,cost_class,state,deadline,price_snapshot_json,data_epoch,updated_at,owner_service,site_id,created_at,invocation_json,invocation_hash,request_json,operation_json,route_json,profile_json,fee_binding_json,active_attempt_id,created_ns,execution_hash,create_command_key) VALUES(?,?,?,?,'accepted',?,?,?,?,?,?,?,?,?,?,?,?,?,?,'',?,?,?)`, id, platform.HashBytes(requestHash), op.RootOperationID, op.CostClass, deadline.UTC().Format(time.RFC3339Nano), priceRaw, op.DataEpoch, platform.Now(), op.CallerService, op.Actor.SiteID, now.Format(time.RFC3339Nano), raw, platform.HashBytes(raw), requestRaw, operationRaw, record.Route, profileRaw, bindingRaw, record.CreatedNS, gatewayruntime.ExecutionSnapshotHash(requestRaw, operationRaw, record.Route, profileRaw, record.Binding, record.CreatedNS, op.CallerService, op.Actor.SiteID, op.DataEpoch), key)
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	for _, candidate := range fixed.Candidates {
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT OR IGNORE INTO route_credential_refs(invocation_id,provider_id,credential_version,data_epoch) VALUES(?,?,?,?)`, id, candidate.Provider.ProviderId, candidate.CredentialVersion, op.DataEpoch); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		for _, ref := range candidate.Provider.ExtraHeaderSecretRefs {
			if _, err := tx.Tx.ExecContext(tx.Context, `INSERT OR IGNORE INTO route_header_refs(invocation_id,ref_id) VALUES(?,?)`, id, ref); err != nil {
				return result, gatewayruntime.ExecutionStorageError(err)
			}
		}
	}
	if err := gatewayruntime.SaveInvocationEvent(tx, &record, gatewayruntime.StreamEvent{Event: "accepted", Invocation: &record.Invocation}); err != nil {
		return result, err
	}
	payload := struct {
		ID        string `json:"invocationId"`
		Root      string `json:"rootOperationId"`
		CostClass string `json:"costClass"`
		State     string `json:"state"`
		Epoch     string `json:"dataEpoch"`
	}{id, op.RootOperationID, op.CostClass, "accepted", op.DataEpoch}
	if err := gatewayruntime.EnqueueExecutionEvent(tx, op, "invocation.accepted", id, 1, payload); err != nil {
		return result, err
	}
	result.Invocation = record.Invocation
	if err := gatewayruntime.SaveExecutionReceipt(tx, op, function, key, canonical, result); err != nil {
		return result, err
	}
	// 先取得平台任务组容量，但只有业务事务成功后才允许执行，满容量不会留下假accepted。
	ready, aborted := make(chan struct{}), make(chan struct{})
	committed := false
	defer func() {
		if !committed {
			close(aborted)
		}
	}()
	taskCtx, cancelTask := context.WithDeadline(platform.WithOperationContext(context.WithoutCancel(ctx), op), deadline)
	defer cancelTask()
	_, err = options.Tasks.Start(taskCtx, id, func(work context.Context) error {
		select {
		case <-ready:
			_, err := ExecuteInvocation(work, op, ExecuteInvocationInput{InvocationID: id})
			return err
		case <-aborted:
			return nil
		case <-work.Done():
			return nil
		}
	})
	if err != nil {
		return CreateInvocationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreateInvocationResult{}, err
	}
	tx.Close()
	committed = true
	close(ready)
	return result, nil
}
func StartAcceptedInvocation(ctx context.Context, options gatewayruntime.ExecutionOptions, record gatewayruntime.InvocationRecord) (bool, error) {
	if options.Tasks == nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, false, "模型调用任务组未装配")
	}
	work, cancel := context.WithDeadline(platform.WithOperationContext(context.WithoutCancel(ctx), record.Operation), record.Invocation.Deadline)
	defer cancel()
	return options.Tasks.Start(work, record.Invocation.InvocationId, func(ctx context.Context) error {
		_, err := ExecuteInvocation(ctx, record.Operation, ExecuteInvocationInput{InvocationID: record.Invocation.InvocationId})
		return err
	})
}
func CanonicalInvocationInput(op OperationContext, input CreateInvocationInput) any {
	input.IdempotencyKey = ""
	if input.AdmissionPermit != nil {
		copy := *input.AdmissionPermit
		copy.Token = ""
		copy.ExpiresAt = time.Time{}
		input.AdmissionPermit = &copy
	}
	return struct {
		Owner, Site, Epoch, Root, CostClass string
		Input                               CreateInvocationInput
	}{op.CallerService, op.Actor.SiteID, op.DataEpoch, op.RootOperationID, op.CostClass, input}
}
func ValidateInvocationPayload(input CreateInvocationInput) error {
	for _, message := range input.Messages {
		if message.Parts != nil {
			for _, part := range *message.Parts {
				if part.Kind != "text" {
					if input.Capability != "vision" || message.Role != "user" {
						return platform.NewError(platform.CodeUnsupportedCapability, false, "非文本媒体只允许vision用户消息")
					}
					if _, err := provider.MediaPartAsset(part); err != nil {
						return err
					}
				}
			}
		}
	}
	if input.ExpectedEmbeddingSpace != nil && input.Capability != "embedding" {
		return platform.NewError(platform.CodeValidation, false, "只有embedding调用可声明固定向量空间")
	}
	if input.Decision != nil && input.Capability != "decision" {
		return platform.NewError(platform.CodeValidation, false, "只有decision调用可携带决策题目")
	}
	switch string(input.Capability) {
	case "embedding":
		if len(input.Texts) == 0 || len(input.Texts) > 64 || len(input.Messages) != 0 || len(input.CandidateIds) != 0 || input.MaximumOutputTokens != 0 || input.Query != nil {
			return platform.NewError(platform.CodeValidation, false, "嵌入只接受最多64段文本及0输出预算")
		}
	case "rerank":
		if input.Query == nil || *input.Query == "" || len(input.Texts) == 0 || len(input.Texts) != len(input.CandidateIds) || len(input.Messages) != 0 {
			return platform.NewError(platform.CodeValidation, false, "原生重排必须明确query与等长候选文本/ID")
		}
	case "decision":
		if input.Decision == nil || len(input.Decision.Questions) == 0 || len(input.Messages) != 0 || len(input.Texts) != 0 || len(input.CandidateIds) != 0 || input.Query != nil || input.MaximumOutputTokens != 0 {
			return platform.NewError(platform.CodeValidation, false, "决策调用只接受状态与题目及0输出预算")
		}
		for _, question := range input.Decision.Questions {
			if (question.Type == "choice" || question.Type == "score") && question.Criteria == nil {
				return platform.NewError(platform.CodeValidation, false, "单选和打分题必须给出选项或等级")
			}
		}
	case "chat", "vision", "translation":
		if len(input.Messages) == 0 || len(input.Texts) != 0 || len(input.CandidateIds) != 0 || input.Query != nil || input.MaximumOutputTokens < 1 {
			return platform.NewError(platform.CodeValidation, false, "聊天类调用必须明确消息及正输出预算")
		}
	default:
		return platform.NewError(platform.CodeValidation, false, "未知模型能力")
	}
	return nil
}
func ReadDispatchProvider(tx *gatewayruntime.ExecutionTransaction, op OperationContext, id string) (gen.Provider, error) {
	var provider gen.Provider
	var names, refs []byte
	var epoch string
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT provider_id,version,adapter,base_url,enabled,credential_version,connect_timeout_ms,request_timeout_ms,display_name,extra_header_names_json,extra_header_secret_refs_json,concurrency_limit,stream_idle_timeout_ms,data_epoch FROM providers WHERE provider_id=?`, id).Scan(&provider.ProviderId, &provider.Version, &provider.Adapter, &provider.BaseUrl, &provider.Enabled, &provider.CredentialVersion, &provider.ConnectTimeoutMs, &provider.RequestTimeoutMs, &provider.DisplayName, &names, &refs, &provider.ConcurrencyLimit, &provider.StreamIdleTimeoutMs, &epoch)
	if err == sql.ErrNoRows {
		return provider, platform.NewError(platform.CodeNotFound, false, "固定路线渠道不存在")
	}
	if err != nil {
		return provider, gatewayruntime.ExecutionStorageError(err)
	}
	if epoch != op.DataEpoch {
		return provider, platform.NewError(platform.CodeStaleResult, false, "固定路线渠道代际已变化")
	}
	if json.Unmarshal(names, &provider.ExtraHeaderNames) != nil || json.Unmarshal(refs, &provider.ExtraHeaderSecretRefs) != nil || len(provider.ExtraHeaderNames) != len(provider.ExtraHeaderSecretRefs) {
		return provider, platform.NewError(platform.CodeIntegrityError, false, "固定路线渠道头映射损坏")
	}
	return provider, nil
}

// ValidateInputModel每个主/备用尝试在预留和派发前核原模型真实媒体能力。
func ValidateInputModel(input CreateInvocationInput, model gen.Model, adapter string) error {
	for _, message := range input.Messages {
		if message.Parts == nil {
			continue
		}
		for _, part := range *message.Parts {
			if part.Kind == "text" {
				continue
			}
			if !slices.Contains(model.Capabilities, "vision") || model.MaxInputTokens < 1 {
				return platform.NewError(platform.CodeUnsupportedCapability, false, "媒体模型必须明确vision能力及完整输入窗口")
			}
			if part.Kind == "file" && (!slices.Contains(model.Capabilities, "file") || !slices.Contains([]string{"openai_chat", "anthropic_messages", "gemini"}, adapter)) {
				return platform.NewError(platform.CodeUnsupportedCapability, false, "PDF模型必须明确file与vision能力")
			}
		}
	}
	return nil
}
