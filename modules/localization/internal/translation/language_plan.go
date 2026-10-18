package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type EstimateLanguagePlanInput = gen.EstimateLanguagePlanInput
type EstimateLanguagePlanResult = gen.EstimateLanguagePlanResult
type ApproveLanguagePlanInput = gen.ApproveLanguagePlanInput
type ApproveLanguagePlanResult = gen.ApproveLanguagePlanResult

const languagePlanTTL = 24 * time.Hour

type languagePlanRoute struct {
	TaskID            string        `json:"taskId"`
	ProfileID         string        `json:"profileId"`
	CostClass         string        `json:"costClass"`
	RouteVersion      platform.Hash `json:"routeVersion"`
	ModelID           string        `json:"modelId"`
	ModelSnapshotHash platform.Hash `json:"modelSnapshotHash"`
}

type languagePlanDocument struct {
	DocumentID  string                   `json:"documentId"`
	Reference   gen.PublishedRevisionRef `json:"reference"`
	UnitCount   int64                    `json:"unitCount"`
	InputTokens int64                    `json:"inputTokens"`
}

type languagePlanSnapshot struct {
	Language              string                 `json:"language"`
	LanguageVersion       int                    `json:"languageVersion"`
	ProfileID             string                 `json:"profileId"`
	GlossaryGeneration    int64                  `json:"glossaryGeneration"`
	CatalogGeneration     int64                  `json:"catalogGeneration"`
	InterfaceSourceHash   platform.Hash          `json:"interfaceSourceHash"`
	TreeRevision          int64                  `json:"treeRevision"`
	BatchOutputTokens     int64                  `json:"batchOutputTokens"`
	DocumentCount         int64                  `json:"documentCount"`
	UnitCount             int64                  `json:"unitCount"`
	InterfaceKeyCount     int64                  `json:"interfaceKeyCount"`
	DocumentInputTokens   int64                  `json:"documentInputTokens"`
	InterfaceInputTokens  int64                  `json:"interfaceInputTokens"`
	DocumentOutputTokens  int64                  `json:"documentOutputTokens"`
	InterfaceOutputTokens int64                  `json:"interfaceOutputTokens"`
	DocumentRoute         languagePlanRoute      `json:"documentRoute"`
	InterfaceRoute        languagePlanRoute      `json:"interfaceRoute"`
	Documents             []languagePlanDocument `json:"documents"`
	Quote                 gen.QuoteCostResult    `json:"quote"`
}

type storedLanguagePlan struct {
	PlanID             string
	Language           string
	SiteID             string
	DataEpoch          string
	ActorID            string
	OperationID        string
	Snapshot           languagePlanSnapshot
	SnapshotHash       platform.Hash
	Result             EstimateLanguagePlanResult
	QuoteHash          platform.Hash
	ExpiresAt          platform.Timestamp
	State              string
	ApprovalStartedAt  string
	ApprovalOperation  OperationContext
	ApprovalJSON       []byte
	ApprovedResult     *ApproveLanguagePlanResult
	ApprovedResultJSON []byte
}

type languagePlanJobPayload struct {
	PlanID                 string                           `json:"planId"`
	Language               string                           `json:"language"`
	Kind                   string                           `json:"kind"`
	TreeRevision           int64                            `json:"treeRevision"`
	Document               *gen.PublishedRevisionRef        `json:"document,omitempty"`
	UnitCount              int64                            `json:"unitCount"`
	InterfaceKeyCount      int64                            `json:"interfaceKeyCount"`
	QuoteHash              platform.Hash                    `json:"quoteHash"`
	OriginalExpiresAt      platform.Timestamp               `json:"originalExpiresAt"`
	BatchOutputTokens      int64                            `json:"batchOutputTokens"`
	Route                  languagePlanRoute                `json:"route"`
	ExpectedPriceSnapshots []platform.ExpectedPriceSnapshot `json:"expectedPriceSnapshots"`
}

type storedLanguagePlanJob struct {
	Kind        string
	DocumentID  string
	InputRef    platform.ObjectRef
	InputJSON   []byte
	InputHash   platform.Hash
	DedupeKey   string
	State       string
	JobID       string
	OperationID string
}

type interfacePlanData struct {
	SourceHash        platform.Hash
	CatalogGeneration int64
	Texts             []string
	KeyCount          int64
}

// EstimateLanguagePlan 只读取公开正文、翻译记忆和网关免费估价；不调用模型、不预留或扣减额度。
func EstimateLanguagePlan(ctx context.Context, op OperationContext, input EstimateLanguagePlanInput) (EstimateLanguagePlanResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	if op.CostClass != platform.CostClassBackground {
		return EstimateLanguagePlanResult{}, platform.NewError(platform.CodeForbidden, false, "语言估价只接受后台费用类别")
	}
	normalized, err := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.Language})
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	var config gen.Language
	var glossaryGeneration, catalogGeneration int64
	var interfaceData interfacePlanData
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var readErr error
		config, readErr = language.LanguageForTag(ctx, tx, op, normalized.Tag)
		if errors.Is(readErr, sql.ErrNoRows) {
			return platform.NewError(platform.CodeValidation, false, "目标语言尚未配置")
		}
		if readErr != nil {
			if problem, ok := readErr.(*platform.Error); ok && problem.Code == platform.CodeNotFound {
				return platform.NewError(platform.CodeValidation, false, "目标语言尚未配置")
			}
			return readErr
		}
		if config.AutomaticTranslation {
			return platform.NewError(platform.CodeValidation, false, "目标语言已启用自动翻译")
		}
		if config.ProfileId == nil || !platform.ValidIdentifier(string(*config.ProfileId)) {
			return platform.NewError(platform.CodeValidation, false, "目标语言没有可用翻译档位")
		}
		if readErr = tx.QueryRowContext(ctx, `SELECT glossary_generation,catalog_generation FROM domain_versions WHERE singleton=1`).Scan(&glossaryGeneration, &catalogGeneration); readErr != nil {
			return language.StorageError(readErr)
		}
		interfaceData, readErr = readInterfacePlan(ctx, tx, op, normalized.Tag)
		return readErr
	})
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	documentRoute, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.document")
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	interfaceRoute, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.ui")
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	expectedProfile := string(*config.ProfileId)
	if documentRoute.ProfileID != expectedProfile || interfaceRoute.ProfileID != expectedProfile ||
		documentRoute.ModelID != interfaceRoute.ModelID || documentRoute.ModelSnapshotHash != interfaceRoute.ModelSnapshotHash {
		return EstimateLanguagePlanResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "翻译任务路由与语言档位或单一保守报价不一致")
	}
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	sort.Slice(tree.Documents, func(i, j int) bool { return tree.Documents[i].DocumentID < tree.Documents[j].DocumentID })
	snapshot := languagePlanSnapshot{
		Language:            normalized.Tag,
		LanguageVersion:     config.Version,
		ProfileID:           expectedProfile,
		GlossaryGeneration:  glossaryGeneration,
		CatalogGeneration:   catalogGeneration,
		InterfaceSourceHash: interfaceData.SourceHash,
		TreeRevision:        tree.TreeRevision,
		BatchOutputTokens:   int64(config.BatchOutputTokens),
		DocumentRoute:       planRouteFromRuntime(documentRoute),
		InterfaceRoute:      planRouteFromRuntime(interfaceRoute),
		Documents:           make([]languagePlanDocument, 0, len(tree.Documents)),
	}
	for _, document := range tree.Documents {
		documentPlan, countErr := estimateDocumentPlan(ctx, op, normalized.Tag, document, documentRoute.ModelID, config.TranslationMemoryMinimumCharacters)
		if countErr != nil {
			return EstimateLanguagePlanResult{}, countErr
		}
		snapshot.Documents = append(snapshot.Documents, documentPlan)
		if documentPlan.UnitCount > 0 {
			snapshot.DocumentCount++
		}
		if snapshot.UnitCount > math.MaxInt64-documentPlan.UnitCount ||
			snapshot.DocumentInputTokens > math.MaxInt64-documentPlan.InputTokens {
			return EstimateLanguagePlanResult{}, platform.NewError(platform.CodeValidation, false, "语言计划计数超过整数预算")
		}
		snapshot.UnitCount += documentPlan.UnitCount
		snapshot.DocumentInputTokens += documentPlan.InputTokens
	}
	snapshot.InterfaceKeyCount = interfaceData.KeyCount
	snapshot.InterfaceInputTokens, err = localizationruntime.EstimatePlanTextTokens(ctx, op, interfaceRoute.ModelID, interfaceData.Texts)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	snapshot.DocumentOutputTokens, err = conservativeOutputTokens(snapshot.UnitCount, snapshot.BatchOutputTokens)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	snapshot.InterfaceOutputTokens, err = conservativeOutputTokens(snapshot.InterfaceKeyCount, snapshot.BatchOutputTokens)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	documentQuote, err := localizationruntime.QuotePlanCost(ctx, op, documentRoute.ModelID, snapshot.DocumentInputTokens, snapshot.DocumentOutputTokens)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	interfaceQuote, err := localizationruntime.QuotePlanCost(ctx, op, interfaceRoute.ModelID, snapshot.InterfaceInputTokens, snapshot.InterfaceOutputTokens)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	snapshot.Quote, err = combineLanguagePlanQuotes(documentQuote, interfaceQuote)
	if err != nil {
		return EstimateLanguagePlanResult{}, err
	}
	snapshotRaw, err := platform.CanonicalJSON(snapshot)
	if err != nil {
		return EstimateLanguagePlanResult{}, platform.NewError(platform.CodeInternal, false, "语言计划快照编码失败")
	}
	quoteHash := platform.HashBytes(snapshotRaw)
	expiresAt := platform.Timestamp(time.Now().UTC().Add(languagePlanTTL).Format(time.RFC3339Nano))
	result := EstimateLanguagePlanResult{
		PlanId:            gen.TypesIdentifier(platform.NewIdentifier()),
		DocumentCount:     checkedInt(snapshot.DocumentCount),
		UnitCount:         checkedInt(snapshot.UnitCount),
		InterfaceKeyCount: checkedInt(snapshot.InterfaceKeyCount),
		Quote:             snapshot.Quote,
		QuoteHash:         gen.Hash(quoteHash),
		ExpiresAt:         gen.Timestamp(expiresAt),
	}
	err = saveLanguagePlan(ctx, op, snapshot, snapshotRaw, quoteHash, expiresAt, &result)
	return result, err
}

// ApproveLanguagePlan 保持原估价 revision 不变，并在全部 jobs 接受后原子启用自动翻译。
func ApproveLanguagePlan(ctx context.Context, op OperationContext, input ApproveLanguagePlanInput) (ApproveLanguagePlanResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	if op.CostClass != platform.CostClassBackground {
		return ApproveLanguagePlanResult{}, platform.NewError(platform.CodeForbidden, false, "语言计划批准只接受后台费用类别")
	}
	plan, approved, err := checkLanguagePlanForApproval(ctx, op, input)
	if err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	if approved != nil {
		return *approved, nil
	}
	approvalOp, workCtx, cancel, err := selectApprovalOperation(ctx, op, plan)
	if err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	defer cancel()
	if err = checkLanguagePlanRevisions(workCtx, approvalOp, plan); err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	if err = ensureLanguagePlanJobs(workCtx, approvalOp, &plan); err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	jobs, err := loadLanguagePlanJobs(workCtx, approvalOp, plan.PlanID)
	if err != nil {
		return ApproveLanguagePlanResult{}, err
	}
	accepted := map[string]gen.AcceptedJob{}
	for _, job := range jobs {
		if job.State == "accepted" {
			accepted[job.Kind+"\x00"+job.DocumentID] = gen.AcceptedJob{JobId: gen.Identifier(job.JobID), OperationId: gen.Identifier(job.OperationID), State: "queued"}
			continue
		}
		acceptedJob, createErr := localizationruntime.CreatePlanJob(workCtx, approvalOp, struct {
			Owner     string             `json:"owner"`
			Type      string             `json:"type"`
			InputRef  platform.ObjectRef `json:"inputRef"`
			DedupeKey string             `json:"dedupeKey"`
			DataEpoch string             `json:"dataEpoch"`
		}{"localization", planJobType(job.Kind), job.InputRef, job.DedupeKey, string(approvalOp.DataEpoch)}, planJobIdempotencyKey(plan.PlanID, job.Kind, job.DocumentID))
		if createErr != nil {
			return ApproveLanguagePlanResult{}, createErr
		}
		accepted[job.Kind+"\x00"+job.DocumentID] = acceptedJob
	}
	var result ApproveLanguagePlanResult
	idempotencyKey := "approve." + plan.PlanID
	err = language.Mutate(workCtx, approvalOp, "localization.ApproveLanguagePlan", idempotencyKey, input, &result, func(tx *sql.Tx) error {
		current, lookupErr := loadLanguagePlan(workCtx, tx, approvalOp, plan.PlanID)
		if lookupErr != nil {
			return lookupErr
		}
		if current.QuoteHash != platform.Hash(input.QuoteHash) || current.Snapshot.Language != plan.Snapshot.Language {
			return platform.NewError(platform.CodeValidation, false, "语言计划hash或语言不一致")
		}
		if current.State == "approved" && current.ApprovedResult != nil {
			result = *current.ApprovedResult
			return nil
		}
		if current.State != "pending" || current.ApprovalStartedAt == "" {
			return platform.NewError(platform.CodeConflict, false, "语言计划不处于可批准状态")
		}
		if current.ApprovalStartedAt == "" || !sameApprovalOperation(current.ApprovalOperation, approvalOp) {
			return platform.NewError(platform.CodeConflict, false, "语言计划批准scope已变化")
		}
		if _, updateErr := language.EnableLanguageForPlan(workCtx, tx, approvalOp, current.Snapshot.Language, current.Snapshot.LanguageVersion); updateErr != nil {
			return updateErr
		}
		documentJobs := 0
		var interfaceJob *gen.AcceptedJob
		for _, job := range jobs {
			jobResult := accepted[job.Kind+"\x00"+job.DocumentID]
			updated, updateErr := tx.ExecContext(workCtx, `UPDATE language_plan_jobs SET
				state = 'accepted', job_id = ?, operation_id = ?, accepted_at = ?, updated_at = ?
				WHERE plan_id = ? AND kind = ? AND document_id = ? AND input_hash = ?`,
				string(jobResult.JobId), string(jobResult.OperationId), string(platform.Now()), string(platform.Now()),
				current.PlanID, job.Kind, job.DocumentID, string(job.InputHash))
			if updateErr != nil {
				return language.StorageError(updateErr)
			}
			if changed, rowsErr := updated.RowsAffected(); rowsErr != nil || changed != 1 {
				return platform.NewError(platform.CodeIntegrityError, false, "语言计划任务接受回执未完整保存")
			}
			if job.Kind == "ui" {
				value := jobResult
				interfaceJob = &value
			} else {
				documentJobs++
			}
		}
		if interfaceJob == nil {
			return platform.NewError(platform.CodeIntegrityError, false, "语言计划缺少界面翻译任务")
		}
		result = ApproveLanguagePlanResult{DocumentJobs: documentJobs, InterfaceJob: *interfaceJob}
		approvedRaw, encodeErr := platform.CanonicalJSON(result)
		if encodeErr != nil {
			return platform.NewError(platform.CodeInternal, false, "语言计划批准结果编码失败")
		}
		now := string(platform.Now())
		updated, updateErr := tx.ExecContext(workCtx, `UPDATE language_plans SET state = 'approved',
			approved_at = ?, approved_by = ?, approved_result_json = ?, updated_at = ?
			WHERE plan_id = ? AND site_id = ? AND data_epoch = ? AND state = 'pending'
				AND quote_hash = ? AND approval_started_at IS NOT NULL`,
			now, string(approvalOp.Actor.SubjectID), string(approvedRaw), now, current.PlanID,
			string(approvalOp.Actor.SiteID), string(approvalOp.DataEpoch), string(current.QuoteHash))
		if updateErr != nil {
			return language.StorageError(updateErr)
		}
		if changed, rowsErr := updated.RowsAffected(); rowsErr != nil || changed != 1 {
			return platform.NewError(platform.CodeConflict, false, "语言计划已被并发批准")
		}
		return nil
	})
	return result, err
}

func estimateDocumentPlan(ctx context.Context, op OperationContext, target string, document localizationruntime.PublicTreeDocument, modelID string, minimumCharacters int) (languagePlanDocument, error) {
	source, err := localizationruntime.FetchPublishedSnapshot(ctx, op, document.Reference)
	if err != nil {
		return languagePlanDocument{}, err
	}
	sourceLanguage, err := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: source.SourceLanguage})
	if err != nil {
		return languagePlanDocument{}, err
	}
	result := languagePlanDocument{DocumentID: document.DocumentID, Reference: source.Reference}
	if sourceLanguage.Tag == target {
		return result, nil
	}
	split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{
		Document:       source.Reference,
		Source:         string(source.Body),
		TargetLanguage: target,
	})
	if err != nil {
		return languagePlanDocument{}, err
	}
	texts := []string{}
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		for _, unit := range split.Units {
			if unit.SourceText == nil {
				return platform.NewError(platform.CodeDependencyUnavailable, false, "语言计划缺少待估单元原文")
			}
			reused, reuseErr := ReuseInSnapshot(ctx, tx, op, ReuseTranslationMemoryInput{
				SourceHash:        unit.SourceHash,
				SourceText:        *unit.SourceText,
				TargetLanguage:    unit.TargetLanguage,
				MinimumCharacters: minimumCharacters,
				Candidate:         unit,
			})
			if reuseErr != nil {
				return reuseErr
			}
			if !reused.Reused {
				texts = append(texts, *unit.SourceText)
				result.UnitCount++
			}
		}
		return nil
	})
	if err != nil {
		return languagePlanDocument{}, err
	}
	result.InputTokens, err = localizationruntime.EstimatePlanTextTokens(ctx, op, modelID, texts)
	return result, err
}

func readInterfacePlan(ctx context.Context, tx *sql.Tx, op OperationContext, target string) (interfacePlanData, error) {
	var result interfacePlanData
	var sourceCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM interface_sources`).Scan(&sourceCount); err != nil {
		return result, language.StorageError(err)
	}
	source := map[string]gen.InterfaceString{}
	if sourceCount > 0 {
		var err error
		source, err = interfacecatalog.SourceCatalog(ctx, tx)
		if err != nil {
			return result, err
		}
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]gen.InterfaceString, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, source[key])
	}
	sourceRaw, err := platform.CanonicalJSON(ordered)
	if err != nil {
		return result, platform.NewError(platform.CodeInternal, false, "界面源目录编码失败")
	}
	result.SourceHash = platform.HashBytes(sourceRaw)
	if err = tx.QueryRowContext(ctx, `SELECT catalog_generation FROM domain_versions WHERE singleton=1`).Scan(&result.CatalogGeneration); err != nil {
		return result, language.StorageError(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT key,source_hash,protected FROM interface_catalog WHERE language=? AND data_epoch=?`, target, string(op.DataEpoch))
	if err != nil {
		return result, language.StorageError(err)
	}
	defer rows.Close()
	stored := map[string]struct {
		sourceHash string
		protected  bool
	}{}
	for rows.Next() {
		var key, sourceHash string
		var protected bool
		if err = rows.Scan(&key, &sourceHash, &protected); err != nil {
			return result, language.StorageError(err)
		}
		stored[key] = struct {
			sourceHash string
			protected  bool
		}{sourceHash, protected}
	}
	if err = rows.Err(); err != nil {
		return result, language.StorageError(err)
	}
	for _, key := range keys {
		item, found := stored[key]
		if found && item.sourceHash == source[key].SourceHash {
			continue
		}
		if found && item.protected {
			continue
		}
		result.Texts = append(result.Texts, source[key].Value)
		result.KeyCount++
	}
	return result, nil
}

func conservativeOutputTokens(count, perBatch int64) (int64, error) {
	if count == 0 || perBatch == 0 {
		return 0, nil
	}
	if perBatch < 0 || count > math.MaxInt64/perBatch {
		return 0, platform.NewError(platform.CodeValidation, false, "语言计划最大输出token超过整数预算")
	}
	return count * perBatch, nil
}

func combineLanguagePlanQuotes(documentQuote, interfaceQuote gen.QuoteCostResult) (gen.QuoteCostResult, error) {
	documentSnapshot, err := platform.CanonicalJSON(documentQuote.PriceSnapshot)
	if err != nil {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeInternal, false, "文档任务报价快照编码失败")
	}
	interfaceSnapshot, err := platform.CanonicalJSON(interfaceQuote.PriceSnapshot)
	if err != nil {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeInternal, false, "界面任务报价快照编码失败")
	}
	if platform.HashBytes(documentSnapshot) != platform.HashBytes(interfaceSnapshot) {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "文档与界面任务没有同一保守价目快照")
	}
	result := documentQuote
	if !documentQuote.PricingKnown || !interfaceQuote.PricingKnown {
		result.PricingKnown = false
		result.MaximumCost = nil
		warning := "price_unknown"
		result.Warning = &warning
		return result, nil
	}
	var total int64
	for _, quote := range []gen.QuoteCostResult{documentQuote, interfaceQuote} {
		if quote.MaximumCost == nil || quote.MaximumCost.Micros < 0 {
			return gen.QuoteCostResult{}, platform.NewError(platform.CodeIntegrityError, false, "已知价目报价缺少合法金额")
		}
		if int64(quote.MaximumCost.Micros) > math.MaxInt64-total {
			return gen.QuoteCostResult{}, platform.NewError(platform.CodeValidation, false, "语言计划费用超过整数预算")
		}
		total += int64(quote.MaximumCost.Micros)
	}
	maxInt := int64(^uint(0) >> 1)
	if total > maxInt {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeValidation, false, "语言计划费用超过平台整数范围")
	}
	result.MaximumCost = &gen.Money{Currency: "USD", Micros: int(total)}
	result.Warning = nil
	return result, nil
}

func planRouteFromRuntime(route localizationruntime.PlanTaskRoute) languagePlanRoute {
	return languagePlanRoute{
		TaskID:            route.TaskID,
		ProfileID:         route.ProfileID,
		CostClass:         route.CostClass,
		RouteVersion:      route.RouteVersion,
		ModelID:           route.ModelID,
		ModelSnapshotHash: route.ModelSnapshotHash,
	}
}

func checkedInt(value int64) int {
	maxInt := int64(^uint(0) >> 1)
	if value < 0 || value > maxInt {
		return int(maxInt)
	}
	return int(value)
}

func saveLanguagePlan(ctx context.Context, op OperationContext, snapshot languagePlanSnapshot, snapshotRaw []byte, quoteHash platform.Hash, expiresAt platform.Timestamp, result *EstimateLanguagePlanResult) error {
	return language.WriteData(ctx, op, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT plan_id,snapshot_hash,expires_at,approval_started_at
			FROM language_plans WHERE site_id=? AND data_epoch=? AND language=? AND state='pending'
			ORDER BY created_at DESC LIMIT 1`, string(op.Actor.SiteID), string(op.DataEpoch), snapshot.Language)
		var oldID, oldHash, oldExpiry string
		var approvalStarted sql.NullString
		err := row.Scan(&oldID, &oldHash, &oldExpiry, &approvalStarted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return language.StorageError(err)
		}
		if err == nil {
			if approvalStarted.Valid {
				return platform.NewError(platform.CodeConflict, false, "目标语言已有正在批准的计划")
			}
			oldExpires, parseErr := platform.ParseTimestamp(platform.Timestamp(oldExpiry))
			if parseErr != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "既有语言计划过期时间损坏")
			}
			if oldHash == string(quoteHash) && oldExpires.After(time.Now().UTC()) {
				return readPlanResult(ctx, tx, oldID, result)
			}
			if _, err = tx.ExecContext(ctx, `UPDATE language_plans SET state='superseded',updated_at=?
				WHERE plan_id=? AND state='pending' AND approval_started_at IS NULL`, string(platform.Now()), oldID); err != nil {
				return language.StorageError(err)
			}
		}
		resultRaw, encodeErr := platform.CanonicalJSON(result)
		if encodeErr != nil {
			return platform.NewError(platform.CodeInternal, false, "语言计划结果编码失败")
		}
		now := string(platform.Now())
		if _, err = tx.ExecContext(ctx, `INSERT INTO language_plans(
			plan_id,language,site_id,data_epoch,actor_id,operation_id,snapshot_json,snapshot_hash,
			result_json,quote_hash,expires_at,state,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,'pending',?,?)`,
			string(result.PlanId), snapshot.Language, string(op.Actor.SiteID), string(op.DataEpoch),
			string(op.Actor.SubjectID), string(op.OperationID), snapshotRaw, string(quoteHash),
			resultRaw, string(quoteHash), string(expiresAt), now, now); err != nil {
			return platform.NewError(platform.CodeConflict, false, "目标语言计划已被并发替换")
		}
		for _, document := range snapshot.Documents {
			referenceRaw, encodeErr := platform.CanonicalJSON(document.Reference)
			if encodeErr != nil {
				return platform.NewError(platform.CodeInternal, false, "语言计划文档引用编码失败")
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO language_plan_documents(
				plan_id,document_id,reference_json,reference_hash,unit_count,input_tokens)
				VALUES(?,?,?,?,?,?)`, string(result.PlanId), document.DocumentID, referenceRaw,
				string(platform.HashBytes(referenceRaw)), document.UnitCount, document.InputTokens); err != nil {
				return language.StorageError(err)
			}
		}
		return nil
	})
}

func readPlanResult(ctx context.Context, tx *sql.Tx, planID string, result *EstimateLanguagePlanResult) error {
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT result_json FROM language_plans WHERE plan_id=?`, planID).Scan(&raw); err != nil {
		return language.StorageError(err)
	}
	if json.Unmarshal(raw, result) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "既有语言计划结果损坏")
	}
	return nil
}

func checkLanguagePlanForApproval(ctx context.Context, op OperationContext, input ApproveLanguagePlanInput) (storedLanguagePlan, *ApproveLanguagePlanResult, error) {
	var plan storedLanguagePlan
	var approved *ApproveLanguagePlanResult
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		plan, err = loadLanguagePlan(ctx, tx, op, string(input.PlanId))
		if err != nil {
			return err
		}
		normalized, normalizeErr := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.Language})
		if normalizeErr != nil {
			return normalizeErr
		}
		if plan.Snapshot.Language != normalized.Tag || plan.QuoteHash != platform.Hash(input.QuoteHash) || plan.Result.QuoteHash != input.QuoteHash {
			return platform.NewError(platform.CodeValidation, false, "语言计划hash或语言不一致")
		}
		if plan.State == "approved" {
			if plan.ApprovedResult == nil {
				return platform.NewError(platform.CodeIntegrityError, false, "已批准语言计划缺少原结果")
			}
			approved = plan.ApprovedResult
			return nil
		}
		if plan.State != "pending" {
			return platform.NewError(platform.CodeConflict, false, "语言计划已被替换")
		}
		expires, parseErr := platform.ParseTimestamp(plan.ExpiresAt)
		if parseErr != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "语言计划过期时间损坏")
		}
		if plan.ApprovalStartedAt == "" && !expires.After(time.Now().UTC()) {
			return platform.NewError(platform.CodeValidation, false, "语言计划已过期")
		}
		config, configErr := language.LanguageForTag(ctx, tx, op, normalized.Tag)
		if configErr != nil {
			var problem *platform.Error
			if errors.As(configErr, &problem) && problem.Code == platform.CodeNotFound {
				return platform.NewError(platform.CodeValidation, false, "批准前目标语言配置已变化")
			}
			return configErr
		}
		if config.Version != plan.Snapshot.LanguageVersion || config.AutomaticTranslation ||
			config.ProfileId == nil || string(*config.ProfileId) != plan.Snapshot.ProfileID {
			return platform.NewError(platform.CodeValidation, false, "批准前语言配置revision已变化")
		}
		var glossaryGeneration, catalogGeneration int64
		if err = tx.QueryRowContext(ctx, `SELECT glossary_generation,catalog_generation FROM domain_versions WHERE singleton=1`).Scan(&glossaryGeneration, &catalogGeneration); err != nil {
			return language.StorageError(err)
		}
		interfaceData, interfaceErr := readInterfacePlan(ctx, tx, op, normalized.Tag)
		if interfaceErr != nil {
			return interfaceErr
		}
		if glossaryGeneration != plan.Snapshot.GlossaryGeneration ||
			catalogGeneration != plan.Snapshot.CatalogGeneration ||
			interfaceData.SourceHash != plan.Snapshot.InterfaceSourceHash ||
			interfaceData.KeyCount != plan.Snapshot.InterfaceKeyCount {
			return platform.NewError(platform.CodeValidation, false, "批准前术语或界面字典revision已变化")
		}
		return nil
	})
	return plan, approved, err
}

func selectApprovalOperation(ctx context.Context, current OperationContext, plan storedLanguagePlan) (OperationContext, context.Context, context.CancelFunc, error) {
	approval := current
	if plan.ApprovalStartedAt != "" {
		if plan.ApprovalOperation.RootOperationID == "" ||
			plan.ApprovalOperation.Actor.SubjectID != current.Actor.SubjectID ||
			plan.ApprovalOperation.Actor.SiteID != current.Actor.SiteID {
			return OperationContext{}, nil, nil, platform.NewError(platform.CodeForbidden, false, "语言计划只能沿原批准主体继续")
		}
		approval = plan.ApprovalOperation
	}
	if approval.RootOperationID == "" || approval.OperationID == "" || approval.CostClass != platform.CostClassBackground ||
		approval.Actor.Kind != "admin" || approval.DataEpoch != current.DataEpoch ||
		approval.Actor.SiteID != current.Actor.SiteID {
		return OperationContext{}, nil, nil, platform.NewError(platform.CodeConflict, false, "语言计划原批准scope不完整")
	}
	deadline, err := platform.ParseTimestamp(approval.Deadline)
	if err != nil {
		return OperationContext{}, nil, nil, platform.NewError(platform.CodeIntegrityError, false, "语言计划原批准deadline损坏")
	}
	if !deadline.After(time.Now().UTC()) {
		return OperationContext{}, nil, nil, platform.NewError(platform.CodeDeadlineExceeded, false, "语言计划原批准截止已过，不延长付费许可")
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline)
	return approval, workCtx, cancel, nil
}

func checkLanguagePlanRevisions(ctx context.Context, op OperationContext, plan storedLanguagePlan) error {
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return err
	}
	if !samePlanDocuments(plan.Snapshot, tree) {
		return platform.NewError(platform.CodeValidation, false, "批准前公开树revision已变化")
	}
	documentRoute, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.document")
	if err != nil {
		return err
	}
	interfaceRoute, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.ui")
	if err != nil {
		return err
	}
	if !samePlanRoute(plan.Snapshot.DocumentRoute, planRouteFromRuntime(documentRoute)) ||
		!samePlanRoute(plan.Snapshot.InterfaceRoute, planRouteFromRuntime(interfaceRoute)) {
		return platform.NewError(platform.CodeValidation, false, "批准前任务路由revision已变化")
	}
	documentQuote, err := localizationruntime.QuotePlanCost(ctx, op, documentRoute.ModelID, plan.Snapshot.DocumentInputTokens, plan.Snapshot.DocumentOutputTokens)
	if err != nil {
		return err
	}
	interfaceQuote, err := localizationruntime.QuotePlanCost(ctx, op, interfaceRoute.ModelID, plan.Snapshot.InterfaceInputTokens, plan.Snapshot.InterfaceOutputTokens)
	if err != nil {
		return err
	}
	currentQuote, err := combineLanguagePlanQuotes(documentQuote, interfaceQuote)
	if err != nil {
		return err
	}
	expectedRaw, err := platform.CanonicalJSON(plan.Snapshot.Quote)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "原语言计划报价编码失败")
	}
	currentRaw, err := platform.CanonicalJSON(currentQuote)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "当前语言计划报价编码失败")
	}
	if string(expectedRaw) != string(currentRaw) {
		return platform.NewError(platform.CodeValidation, false, "批准前价目快照已变化")
	}
	return nil
}

func loadLanguagePlan(ctx context.Context, tx *sql.Tx, op OperationContext, planID string) (storedLanguagePlan, error) {
	var result storedLanguagePlan
	var snapshotRaw, resultRaw, approvedRaw, approvalJSON []byte
	var approvalStarted, approvalRoot, approvalActor, approvalDeadline, approvedAt, approvedBy sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT plan_id,language,site_id,data_epoch,actor_id,operation_id,
		snapshot_json,snapshot_hash,result_json,quote_hash,expires_at,state,approval_started_at,
		approval_operation_json,approval_root_operation_id,approval_actor_id,approval_deadline,
		approved_at,approved_by,approved_result_json
		FROM language_plans WHERE plan_id=? AND site_id=? AND data_epoch=?`,
		planID, string(op.Actor.SiteID), string(op.DataEpoch)).Scan(
		&result.PlanID, &result.Language, &result.SiteID, &result.DataEpoch, &result.ActorID,
		&result.OperationID, &snapshotRaw, &result.SnapshotHash, &resultRaw, &result.QuoteHash,
		&result.ExpiresAt, &result.State, &approvalStarted, &approvalJSON, &approvalRoot,
		&approvalActor, &approvalDeadline, &approvedAt, &approvedBy, &approvedRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return storedLanguagePlan{}, platform.NewError(platform.CodeValidation, false, "语言计划不存在或不属于当前站代次")
	}
	if err != nil {
		return storedLanguagePlan{}, language.StorageError(err)
	}
	if json.Unmarshal(snapshotRaw, &result.Snapshot) != nil ||
		platform.HashBytes(snapshotRaw) != result.SnapshotHash ||
		json.Unmarshal(resultRaw, &result.Result) != nil ||
		result.Result.QuoteHash != gen.Hash(result.QuoteHash) {
		return storedLanguagePlan{}, platform.NewError(platform.CodeIntegrityError, false, "语言计划快照或结果损坏")
	}
	if approvalStarted.Valid {
		result.ApprovalStartedAt = approvalStarted.String
		if json.Unmarshal(approvalJSON, &result.ApprovalOperation) != nil ||
			!approvalRoot.Valid || !approvalActor.Valid || !approvalDeadline.Valid ||
			result.ApprovalOperation.RootOperationID != approvalRoot.String ||
			result.ApprovalOperation.Actor.SubjectID != approvalActor.String ||
			string(result.ApprovalOperation.Deadline) != approvalDeadline.String ||
			result.ApprovalOperation.Actor.SiteID != result.SiteID ||
			result.ApprovalOperation.DataEpoch != result.DataEpoch {
			return storedLanguagePlan{}, platform.NewError(platform.CodeIntegrityError, false, "语言计划原批准scope损坏")
		}
		result.ApprovalJSON = approvalJSON
	} else if len(approvalJSON) != 0 || approvalRoot.Valid || approvalActor.Valid || approvalDeadline.Valid {
		return storedLanguagePlan{}, platform.NewError(platform.CodeIntegrityError, false, "语言计划存在无启动时间的批准scope")
	}
	if approvedRaw != nil {
		var value ApproveLanguagePlanResult
		if json.Unmarshal(approvedRaw, &value) != nil || !approvedAt.Valid || !approvedBy.Valid {
			return storedLanguagePlan{}, platform.NewError(platform.CodeIntegrityError, false, "语言计划批准结果损坏")
		}
		result.ApprovedResult = &value
		result.ApprovedResultJSON = approvedRaw
	}
	return result, nil
}

func ensureLanguagePlanJobs(ctx context.Context, op OperationContext, plan *storedLanguagePlan) error {
	return language.WriteData(ctx, op, func(tx *sql.Tx) error {
		current, err := loadLanguagePlan(ctx, tx, op, plan.PlanID)
		if err != nil {
			return err
		}
		if current.State != "pending" {
			return platform.NewError(platform.CodeConflict, false, "语言计划已不能创建任务")
		}
		now := string(platform.Now())
		if current.ApprovalStartedAt == "" {
			operationRaw, encodeErr := platform.CanonicalJSON(op)
			if encodeErr != nil {
				return platform.NewError(platform.CodeInternal, false, "语言计划原批准scope编码失败")
			}
			updated, updateErr := tx.ExecContext(ctx, `UPDATE language_plans SET approval_started_at=?,
				approval_operation_json=?,approval_root_operation_id=?,approval_actor_id=?,approval_deadline=?,updated_at=?
				WHERE plan_id=? AND state='pending' AND approval_started_at IS NULL`,
				now, operationRaw, string(op.RootOperationID), string(op.Actor.SubjectID), string(op.Deadline), now, current.PlanID)
			if updateErr != nil {
				return language.StorageError(updateErr)
			}
			if changed, rowsErr := updated.RowsAffected(); rowsErr != nil || changed != 1 {
				return platform.NewError(platform.CodeConflict, false, "语言计划批准启动发生并发变化")
			}
			current.ApprovalStartedAt = now
			current.ApprovalOperation = op
			current.ApprovalJSON = operationRaw
		} else if !sameApprovalOperation(current.ApprovalOperation, op) {
			return platform.NewError(platform.CodeConflict, false, "语言计划批准必须沿原OperationContext重放")
		}
		for _, document := range current.Snapshot.Documents {
			if document.UnitCount == 0 {
				continue
			}
			reference := document.Reference
			if err = ensureLanguagePlanJob(ctx, tx, op, current, "document", document.DocumentID, &reference); err != nil {
				return err
			}
		}
		if err = ensureLanguagePlanJob(ctx, tx, op, current, "ui", "", nil); err != nil {
			return err
		}
		*plan = current
		return nil
	})
}

func ensureLanguagePlanJob(ctx context.Context, tx *sql.Tx, op OperationContext, plan storedLanguagePlan, kind, documentID string, document *gen.PublishedRevisionRef) error {
	route := plan.Snapshot.DocumentRoute
	unitCount := int64(0)
	interfaceKeyCount := int64(0)
	if kind == "ui" {
		route = plan.Snapshot.InterfaceRoute
		interfaceKeyCount = plan.Snapshot.InterfaceKeyCount
	} else {
		for _, item := range plan.Snapshot.Documents {
			if item.DocumentID == documentID {
				unitCount = item.UnitCount
				break
			}
		}
	}
	payload := languagePlanJobPayload{
		PlanID:                 plan.PlanID,
		Language:               plan.Snapshot.Language,
		Kind:                   kind,
		TreeRevision:           plan.Snapshot.TreeRevision,
		Document:               document,
		UnitCount:              unitCount,
		InterfaceKeyCount:      interfaceKeyCount,
		QuoteHash:              plan.QuoteHash,
		OriginalExpiresAt:      plan.ExpiresAt,
		BatchOutputTokens:      plan.Snapshot.BatchOutputTokens,
		Route:                  route,
		ExpectedPriceSnapshots: []platform.ExpectedPriceSnapshot{{ModelID: platform.Identifier(route.ModelID), PriceSnapshotID: platform.Identifier(plan.Snapshot.Quote.PriceSnapshot.PriceSnapshotId)}},
	}
	inputRaw, err := platform.CanonicalJSON(payload)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "语言计划任务输入编码失败")
	}
	keyPrefix := "localization.document:"
	if kind == "ui" {
		keyPrefix = "localization.ui:"
	}
	// jobs以dedupeKey+inputRef共同幂等；键必须绑定当前完整计划输入，不能跨计划复用。
	dedupeKey := keyPrefix + plan.PlanID + ":" + platform.HashBytes(inputRaw)
	var existingRefRaw, existingInputRaw []byte
	var existingHash, existingDedupe, existingState, existingJob, existingOperation, existingAccepted sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT input_ref_json,input_json,input_hash,dedupe_key,state,job_id,operation_id,accepted_at
		FROM language_plan_jobs WHERE plan_id=? AND kind=? AND document_id=?`,
		plan.PlanID, kind, documentID).Scan(&existingRefRaw, &existingInputRaw, &existingHash,
		&existingDedupe, &existingState, &existingJob, &existingOperation, &existingAccepted)
	if err == nil {
		var existingRef platform.ObjectRef
		if json.Unmarshal(existingRefRaw, &existingRef) != nil ||
			existingRef.Owner != "localization" || existingRef.ObjectID == "" ||
			existingRef.SHA256 != platform.HashBytes(existingInputRaw) || existingRef.ByteSize != int64(len(existingInputRaw)) ||
			existingHash.String != string(platform.HashBytes(inputRaw)) || existingDedupe.String != dedupeKey ||
			string(existingInputRaw) != string(inputRaw) {
			return platform.NewError(platform.CodeIntegrityError, false, "既有语言计划任务意图损坏")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return language.StorageError(err)
	}
	objectID := platform.NewIdentifier()
	ref := platform.ObjectRef{
		Owner:     "localization",
		ObjectID:  objectID,
		Version:   1,
		SHA256:    platform.HashBytes(inputRaw),
		MediaType: "application/json",
		ByteSize:  int64(len(inputRaw)),
	}
	refRaw, err := platform.CanonicalJSON(ref)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "语言计划任务引用编码失败")
	}
	now := string(platform.Now())
	_, err = tx.ExecContext(ctx, `INSERT INTO language_plan_jobs(
		plan_id,kind,document_id,input_ref_json,input_ref_hash,input_json,input_hash,
		dedupe_key,state,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,'pending',?,?)`,
		plan.PlanID, kind, documentID, refRaw, string(platform.HashBytes(refRaw)), inputRaw,
		string(platform.HashBytes(inputRaw)), dedupeKey, now, now)
	if err != nil {
		return language.StorageError(err)
	}
	return nil
}

func loadLanguagePlanJobs(ctx context.Context, op OperationContext, planID string) ([]storedLanguagePlanJob, error) {
	jobs := []storedLanguagePlanJob{}
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT kind,document_id,input_ref_json,input_json,input_hash,
			dedupe_key,state,COALESCE(job_id,''),COALESCE(operation_id,'')
			FROM language_plan_jobs WHERE plan_id=?
			ORDER BY CASE kind WHEN 'document' THEN 0 ELSE 1 END,document_id`, planID)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var item storedLanguagePlanJob
			var refRaw []byte
			if err = rows.Scan(&item.Kind, &item.DocumentID, &refRaw, &item.InputJSON, &item.InputHash,
				&item.DedupeKey, &item.State, &item.JobID, &item.OperationID); err != nil {
				return language.StorageError(err)
			}
			if json.Unmarshal(refRaw, &item.InputRef) != nil ||
				item.InputRef.Owner != "localization" || item.InputRef.SHA256 != platform.HashBytes(item.InputJSON) ||
				item.InputRef.ByteSize != int64(len(item.InputJSON)) {
				return platform.NewError(platform.CodeIntegrityError, false, "语言计划任务意图损坏")
			}
			jobs = append(jobs, item)
		}
		return language.StorageError(rows.Err())
	})
	return jobs, err
}

func samePlanDocuments(snapshot languagePlanSnapshot, tree localizationruntime.PublicTreeSnapshot) bool {
	if snapshot.TreeRevision != tree.TreeRevision || len(snapshot.Documents) != len(tree.Documents) {
		return false
	}
	current := append([]localizationruntime.PublicTreeDocument{}, tree.Documents...)
	sort.Slice(current, func(i, j int) bool { return current[i].DocumentID < current[j].DocumentID })
	for index, document := range snapshot.Documents {
		if document.DocumentID != current[index].DocumentID || document.Reference != current[index].Reference {
			return false
		}
	}
	return true
}

func samePlanRoute(expected, current languagePlanRoute) bool {
	return expected == current
}

func sameApprovalOperation(expected, current OperationContext) bool {
	expectedRaw, expectedErr := platform.CanonicalJSON(expected)
	currentRaw, currentErr := platform.CanonicalJSON(current)
	return expectedErr == nil && currentErr == nil && string(expectedRaw) == string(currentRaw)
}

func planJobType(kind string) string {
	if kind == "ui" {
		return "localization.ui"
	}
	return "localization.document"
}

func planJobIdempotencyKey(planID, kind, documentID string) string {
	return "lp." + platform.HashBytes([]byte(planID+"\x00"+kind+"\x00"+documentID))[:63]
}
