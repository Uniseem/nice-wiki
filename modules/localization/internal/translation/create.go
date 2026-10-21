package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type CreateTranslationInput = gen.CreateTranslationInput
type CreateTranslationResult = gen.CreateTranslationResult

// CreateTranslation 为一个发布修订申请某语言的译文：登记 queued 译文与全部翻译单元，随后在本进程里异步翻译。
func CreateTranslation(ctx context.Context, op OperationContext, input CreateTranslationInput) (CreateTranslationResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return CreateTranslationResult{}, err
	}
	return createTranslation(ctx, op, input)
}

func createTranslation(ctx context.Context, op OperationContext, input CreateTranslationInput) (CreateTranslationResult, error) {
	if input.Document.DataEpoch != op.DataEpoch {
		return CreateTranslationResult{}, platform.NewError(platform.CodeStaleResult, false, "发布引用不属于当前数据代次")
	}
	source, err := localizationruntime.FetchPublishedSnapshot(ctx, op, input.Document)
	if err != nil {
		return CreateTranslationResult{}, err
	}
	split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{Document: input.Document, Source: string(source.Body), TargetLanguage: input.TargetLanguage})
	if err != nil {
		return CreateTranslationResult{}, err
	}
	var result CreateTranslationResult
	err = language.Mutate(ctx, op, "localization.CreateTranslation", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		target, err := interfacecatalog.EnabledLanguage(ctx, tx, input.TargetLanguage)
		if err != nil {
			return err
		}
		if target == source.SourceLanguage {
			return platform.NewError(platform.CodeValidation, false, "目标语言与源语言相同，不需要翻译")
		}
		id := platform.NewIdentifier()
		now := time.Now().UTC().Format(time.RFC3339Nano)
		ref, _ := json.Marshal(input.Document)
		if _, err := tx.ExecContext(ctx, `INSERT INTO translation_manifests(translation_id,publication_ref,target_language,generation,memory_hash,glossary_generation,state,data_epoch,updated_at) VALUES(?,?,?,0,?,0,'queued',?,?)`, id, string(ref), target, platform.HashBytes(nil), op.DataEpoch, now); err != nil {
			return language.StorageError(err)
		}
		for _, unit := range split.Units {
			if unit.SourceText == nil {
				return platform.NewError(platform.CodeIntegrityError, false, "翻译单元缺少固定原文")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO translation_units(unit_id,translation_id,document_id,source_hash,source_text,target_language,target_file_ref,target_text,target_hash,origin,protected,version,human_generation,status,previous_target_text,data_epoch,updated_at) VALUES(?,?,?,?,?,?,NULL,'',?,'machine',0,0,0,'normal',NULL,?,?)`, unit.UnitId, id, input.Document.DocumentId, unit.SourceHash, *unit.SourceText, target, platform.HashBytes(nil), op.DataEpoch, now); err != nil {
				return language.StorageError(err)
			}
		}
		result = CreateTranslationResult{Translation: gen.Translation{TranslationId: id, Document: input.Document, TargetLanguage: target, State: gen.TranslationState("queued"), Total: len(split.Units), MemoryVersionHash: platform.HashBytes(nil)}}
		return nil
	})
	if err != nil {
		return CreateTranslationResult{}, err
	}
	go ExecuteTranslation(context.Background(), executionOperation(op, result.Translation.TranslationId), result.Translation.TranslationId)
	return result, nil
}

// executionOperation 是后台翻译用的操作上下文：按后台费用类别，给足 15 分钟。
// 根操作固定为译文 ID：模型网关按“根操作 + 幂等键”认同一次调用，进程重启后接着翻才能复用已完成的批次。
func executionOperation(op OperationContext, translationID string) OperationContext {
	execution := op
	execution.RootOperationID = translationID
	execution.CostClass = platform.CostClassBackground
	execution.Deadline = time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339Nano)
	execution.Actor.ExpiresAt = execution.Deadline
	return execution
}
