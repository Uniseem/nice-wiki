package extract

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"nice-wiki/modules/knowledge/internal/source"
)

type CreateExtractionInput struct {
	ApprovedPlan     *ExtractionPlan    `json:"-"`
	BuildID          string             `json:"buildId"`
	LeaseID          string             `json:"leaseId"`
	Asset            platform.AssetRef  `json:"asset"`
	Source           source.AssetSource `json:"source"`
	ExtractorVersion string             `json:"extractorVersion"`
	IDempotencyKey   string             `json:"idempotencyKey"`
}
type CreateExtractionResult struct {
	Extraction Extraction `json:"extraction"`
}
type ExtractionPlan struct {
	Route                  indexer.TaskRoute                `json:"route"`
	ExpectedPriceSnapshots []platform.ExpectedPriceSnapshot `json:"expectedPriceSnapshots,omitempty"`
	PriceQuotes            map[string]json.RawMessage       `json:"priceQuotes,omitempty"`
	TaskID                 string                           `json:"taskId"`
	Memory                 platform.MemoryContext           `json:"memory"`
	Profile                *indexer.ProfileDeclaration      `json:"profile,omitempty"`
	Assignment             *indexer.TaskAssignment          `json:"assignment,omitempty"`
	ModelSnapshots         []string                         `json:"modelSnapshots"`
	ExtractorVersion       string                           `json:"extractorVersion"`
}
type ExtractionJobInput struct {
	ExtractionID string             `json:"extractionId"`
	Source       source.AssetSource `json:"source"`
	Plan         ExtractionPlan     `json:"plan"`
}
type ExtractionArtifact struct {
	Extraction Extraction `json:"extraction"`
	Pages      []PDFPage  `json:"pages"`
}

// CreateExtraction 原构建批准下冻结提取任务，重放先读原关联，绝不因记忆变化另开付费owner。
func CreateExtraction(ctx context.Context, op OperationContext, input CreateExtractionInput) (CreateExtractionResult, error) {
	_, initial, build, closeInitial, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return CreateExtractionResult{}, err
	}
	originRaw, _ := platform.CanonicalJSON(input.Source)
	originHash := platform.HashBytes(originRaw)
	if !SameAsset(input.Asset, input.Source) || input.Source.Block.Document != build.Build.Document {
		initial.Rollback()
		closeInitial()
		return CreateExtractionResult{}, platform.NewError(platform.CodeIntegrityError, false, "提取资源不属于原构建授权")
	}
	var previous string
	err = initial.QueryRowContext(ctx, `SELECT extraction_id FROM extraction_origins WHERE build_id=? AND source_hash=?`, input.BuildID, originHash).Scan(&previous)
	_ = initial.Rollback()
	closeInitial()
	if err == nil {
		value, err := ReadExtraction(ctx, op, previous, input.Source)
		if err == nil {
			err = DispatchExtraction(ctx, op, previous)
		}
		return CreateExtractionResult{Extraction: value.Extraction}, err
	}
	if err != sql.ErrNoRows {
		return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取关联不可读")
	}
	plan, err := PrepareExtractionPlan(ctx, op, input.Source, input.ExtractorVersion, build.Language)
	if err != nil {
		return CreateExtractionResult{}, err
	}
	if input.ApprovedPlan != nil {
		approved := *input.ApprovedPlan
		plan.ExpectedPriceSnapshots = approved.ExpectedPriceSnapshots
		plan.PriceQuotes = approved.PriceQuotes
		expected, _ := platform.CanonicalJSON(approved)
		actual, _ := platform.CanonicalJSON(plan)
		if string(expected) != string(actual) {
			return CreateExtractionResult{}, platform.NewError(platform.CodeConflict, false, "资源提取当前任务计划与全量重建明批不一致")
		}
	}
	resourceRaw, _ := platform.CanonicalJSON([]any{input.Asset.SHA256, input.Asset.MediaType, input.Asset.ByteSize, plan.TaskID, plan.Memory.VersionHash, plan.Memory.MappingRevision, plan.ExtractorVersion})
	resourceKey := platform.HashBytes(resourceRaw)
	planRaw, _ := platform.CanonicalJSON(plan)
	cacheKey := platform.HashBytes(append(append([]byte{}, resourceRaw...), planRaw...))
	semanticRaw, _ := platform.CanonicalJSON([]any{input.Asset.SHA256, input.Asset.MediaType, input.Asset.ByteSize, plan.TaskID, plan.ExtractorVersion})
	intentKey := "semantic-v1:" + platform.HashBytes(semanticRaw)
	resolution, err := InspectFailedExtraction(ctx, op, input.BuildID, intentKey, input.Source, plan)
	if err != nil {
		return CreateExtractionResult{}, err
	}
	work, tx, _, cancel, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return CreateExtractionResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	err = tx.QueryRowContext(work, `SELECT extraction_id FROM extraction_origins WHERE build_id=? AND source_hash=?`, input.BuildID, originHash).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "并发提取关联不可读")
	}
	id := previous
	if err == sql.ErrNoRows {
		// 先免费恢复原paid完整结果；不冒充旧jobs完成，不改原模型或费用证据。
		if err = SaveRecoveredExtraction(work, tx, op, resolution); err != nil {
			return CreateExtractionResult{}, err
		}
		// 只复用当前模型安全声明覆盖的真实winning结果；从不把计划主模型冒充实际模型。
		rows, err := tx.QueryContext(work, `SELECT e.extraction_id,COALESCE(r.artifact_json,e.result_json),COALESCE(r.artifact_hash,e.result_hash) FROM extractions e LEFT JOIN extraction_recovered_results r ON r.extraction_id=e.extraction_id WHERE e.site_id=? AND e.data_epoch=? AND e.resource_key=? AND (e.state IN ('ready','unsupported') OR r.extraction_id IS NOT NULL) ORDER BY e.created_at,e.extraction_id`, op.Actor.SiteID, op.DataEpoch, resourceKey)
		if err != nil {
			return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取缓存不可读")
		}
		for rows.Next() {
			var candidate string
			var raw []byte
			var hash string
			if err = rows.Scan(&candidate, &raw, &hash); err != nil {
				rows.Close()
				return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取缓存不可读")
			}
			var saved ExtractionArtifact
			if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &saved) != nil {
				rows.Close()
				return CreateExtractionResult{}, platform.NewError(platform.CodeIntegrityError, false, "原提取结果损坏")
			}
			eligible := plan.TaskID == ""
			if saved.Extraction.ModelSnapshotHash != nil {
				for _, snapshot := range plan.ModelSnapshots {
					if *saved.Extraction.ModelSnapshotHash == snapshot {
						eligible = true
						break
					}
				}
			}
			if eligible {
				id = candidate
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取缓存读取失败")
		}
		if id == "" {
			if resolution.PreviousID != "" {
				if _, err = tx.ExecContext(work, `INSERT INTO extraction_cache_intents(site_id,data_epoch,cache_key,extraction_id) VALUES(?,?,?,?) ON CONFLICT(site_id,data_epoch,cache_key) DO NOTHING`, op.Actor.SiteID, op.DataEpoch, intentKey, resolution.PreviousID); err != nil {
					return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "旧资源语义意图保存失败")
				}
			}
			err = tx.QueryRowContext(work, `SELECT extraction_id FROM extraction_cache_intents WHERE site_id=? AND data_epoch=? AND cache_key=?`, op.Actor.SiteID, op.DataEpoch, intentKey).Scan(&id)
			if err != nil && err != sql.ErrNoRows {
				return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取意图不可读")
			}
			if id != "" && id != resolution.PreviousID {
				return CreateExtractionResult{}, platform.NewError(platform.CodeConflict, false, "资源语义拥有者并发变化，须重新读取原费用证明")
			}
			if id != "" && resolution.Blocked {
				return CreateExtractionResult{}, platform.NewError(platform.CodeConflict, false, "同资源原计划未终局，不能用记忆或模型变化绕过原owner")
			}
		}
		replacing := ""
		if resolution.Replace && id == resolution.PreviousID {
			replacing = id
			id = ""
		}
		if id == "" {
			id = platform.NewIdentifier()
			payload, _ := platform.CanonicalJSON(ExtractionJobInput{id, input.Source, plan})
			ref := platform.ObjectRef{Owner: "knowledge", ObjectID: id, Version: 1, SHA256: platform.HashBytes(payload), MediaType: "application/json", ByteSize: int64(len(payload))}
			refRaw, _ := platform.CanonicalJSON(ref)
			var buildOperation []byte
			if err = tx.QueryRowContext(work, `SELECT operation_json FROM build_dispatches WHERE build_id=?`, input.BuildID).Scan(&buildOperation); err != nil {
				return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原构建批准身份不可读")
			}
			// 提取任务以自己的幂等键作为 jobs 操作号，沿用构建批准身份时要换成这个号。
			var extractionOperation OperationContext
			if json.Unmarshal(buildOperation, &extractionOperation) != nil {
				return CreateExtractionResult{}, platform.NewError(platform.CodeIntegrityError, false, "原构建批准身份损坏")
			}
			extractionOperation.OperationID = "knowledge.extract." + id
			buildOperation, _ = json.Marshal(extractionOperation)
			_, err = tx.ExecContext(work, `INSERT INTO extractions(extraction_id,site_id,data_epoch,cache_key,resource_key,owner_build_id,source_json,plan_json,state,created_at) VALUES(?,?,?,?,?,?,?,?,'queued',?)`, id, op.Actor.SiteID, op.DataEpoch, cacheKey, resourceKey, input.BuildID, originRaw, planRaw, platform.Now())
			if err != nil {
				return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取任务保存失败")
			}
			if replacing == "" {
				if _, err = tx.ExecContext(work, `INSERT INTO extraction_cache_intents(site_id,data_epoch,cache_key,extraction_id) VALUES(?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, intentKey, id); err != nil {
					return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取意图保存失败")
				}
			} else {
				changed, err := tx.ExecContext(work, `UPDATE extraction_cache_intents SET extraction_id=? WHERE site_id=? AND data_epoch=? AND cache_key=? AND extraction_id=? AND EXISTS(SELECT 1 FROM extractions e WHERE e.extraction_id=? AND e.state IN ('failed','ready','unsupported') AND e.owner_build_id<>?)`, id, op.Actor.SiteID, op.DataEpoch, intentKey, replacing, replacing, input.BuildID)
				if err != nil {
					return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "新明确批准替换失败提取意图失败")
				}
				count, err := changed.RowsAffected()
				if err != nil {
					return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "失败提取意图CAS不可读")
				}
				if count != 1 {
					return CreateExtractionResult{}, platform.NewError(platform.CodeConflict, false, "失败提取意图已变化，保留原paid结果")
				}
			}
			if _, err = tx.ExecContext(work, `INSERT INTO extraction_dispatches(extraction_id,input_json,input_ref_json,operation_json,dedupe_key,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, payload, refRaw, buildOperation, "knowledge.extract."+id, platform.Now(), platform.Now()); err != nil {
				return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取派发保存失败")
			}
		}
		if _, err = tx.ExecContext(work, `INSERT INTO extraction_origins(extraction_id,build_id,source_hash,source_json) VALUES(?,?,?,?)`, id, input.BuildID, originHash, originRaw); err != nil {
			return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取授权关联保存失败")
		}
	}
	if err = tx.Commit(); err != nil {
		return CreateExtractionResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取接受提交失败")
	}
	value, err := ReadExtraction(ctx, op, id, input.Source)
	if err != nil {
		return CreateExtractionResult{}, err
	}
	// 原已提交意图可续派；只用其原input/root/key，不把未知派发换成新任务。
	if err = DispatchExtraction(ctx, op, id); err != nil {
		return CreateExtractionResult{}, err
	}
	return CreateExtractionResult{Extraction: value.Extraction}, nil
}

func PrepareExtractionPlan(ctx context.Context, op OperationContext, origin source.AssetSource, version, language string) (ExtractionPlan, error) {
	plan := ExtractionPlan{ExtractorVersion: version, ModelSnapshots: []string{}}
	if op.CostClass != platform.CostClassBackground {
		return plan, platform.NewError(platform.CodeForbidden, false, "资源提取只接受原构建批准费用类别")
	}
	switch origin.Asset.MediaType {
	case "application/pdf":
		if version != PDFExtractorVersion {
			return plan, platform.NewError(platform.CodeUnsupportedSchema, false, "PDF提取器版本不支持")
		}
		return plan, nil
	case "video/mp4", "video/webm":
		if version != "video-published-metadata-v1" {
			return plan, platform.NewError(platform.CodeUnsupportedSchema, false, "视频元数据版本不支持")
		}
		return plan, nil
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		plan.TaskID = "knowledge.describe-visual"
	case "text/plain", "text/markdown", "text/csv", "application/json":
		plan.TaskID = "knowledge.describe"
	default:
		return plan, nil
	}
	if version != "resource-description-v1" {
		return plan, platform.NewError(platform.CodeUnsupportedSchema, false, "资源描述提取器版本不支持")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return plan, err
	}
	memory, err := indexer.ResolveTaskMemory(ctx, op, plan.TaskID, language)
	if err != nil {
		return plan, err
	}
	plan.Memory = memory
	route, err := indexer.ReadTaskRoute(ctx, op, runtime, plan.TaskID)
	if err != nil {
		return plan, err
	}
	plan.Route = route
	plan.Profile = &route.Profile
	plan.Assignment = route.Assignment
	for _, model := range route.Models {
		plan.ModelSnapshots = append(plan.ModelSnapshots, model.ModelSnapshotHash)
	}
	if len(plan.ModelSnapshots) == 0 {
		return plan, platform.NewError(platform.CodeDependencyUnavailable, false, "资源模型计划缺真实snapshot证明")
	}
	return plan, nil
}

// ReadExtraction 原已付费文本只变更当前授权来源视图，不改原结果及原模型证明。
func ReadExtraction(ctx context.Context, op OperationContext, id string, origin source.AssetSource) (ExtractionArtifact, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ExtractionArtifact{}, err
	}
	var raw []byte
	var hash, site, epoch, state, version string
	err = runtime.DB.QueryRowContext(ctx, `SELECT COALESCE(r.artifact_json,e.result_json),COALESCE(r.artifact_hash,e.result_hash),e.site_id,e.data_epoch,e.state,e.plan_json FROM extractions e LEFT JOIN extraction_recovered_results r ON r.extraction_id=e.extraction_id WHERE e.extraction_id=?`, id).Scan(&raw, &hash, &site, &epoch, &state, &version)
	if err != nil {
		return ExtractionArtifact{}, platform.NormalizeIOError(err, platform.CodeNotFound, false, "原提取不存在")
	}
	if site != op.Actor.SiteID || epoch != op.DataEpoch {
		return ExtractionArtifact{}, platform.NewError(platform.CodeForbidden, false, "原提取不属于本站同代")
	}
	originRaw, _ := platform.CanonicalJSON(origin)
	var authorized int
	if err = runtime.DB.QueryRowContext(ctx, `SELECT count(*) FROM extraction_origins o JOIN builds b ON b.build_id=o.build_id WHERE o.extraction_id=? AND o.source_json=? AND b.site_id=? AND b.data_epoch=?`, id, originRaw, op.Actor.SiteID, op.DataEpoch).Scan(&authorized); err != nil {
		return ExtractionArtifact{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "原提取授权关联不可读")
	}
	if authorized == 0 {
		return ExtractionArtifact{}, platform.NewError(platform.CodeForbidden, false, "原提取没有该精确发布资源授权")
	}

	var plan ExtractionPlan
	if json.Unmarshal([]byte(version), &plan) != nil {
		return ExtractionArtifact{}, platform.NewError(platform.CodeIntegrityError, false, "原提取计划损坏")
	}
	result := ExtractionArtifact{Extraction: Extraction{ExtractionID: id, SourceBlock: &origin.Block, ExtractorVersion: plan.ExtractorVersion, State: state}, Pages: []PDFPage{}}
	asset := platform.AssetRef{AssetID: origin.Asset.AssetID, SHA256: origin.Asset.SHA256, MediaType: origin.Asset.MediaType, ByteSize: origin.Asset.ByteSize}
	result.Extraction.Asset = &asset
	if len(raw) > 0 {
		if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &result) != nil {
			return ExtractionArtifact{}, platform.NewError(platform.CodeIntegrityError, false, "原提取结果损坏")
		}
		result.Extraction.SourceBlock = &origin.Block
		result.Extraction.Asset = &asset
	}
	return result, nil
}
