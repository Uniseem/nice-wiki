package translation

import (
 "context"
 "database/sql"
 "errors"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
)

type ReuseTranslationMemoryInput struct{SourceHash string `json:"sourceHash"`;SourceText string `json:"sourceText"`;TargetLanguage string `json:"targetLanguage"`;MinimumCharacters int `json:"minimumCharacters"`;Candidate gen.TranslationUnit `json:"candidate"`}
type ReuseTranslationMemoryResult struct{Unit *gen.TranslationUnit `json:"unit,omitempty"`;Reused bool `json:"reused"`}
func ReuseTranslationMemory(ctx context.Context,op OperationContext,input ReuseTranslationMemoryInput)(ReuseTranslationMemoryResult,error){
 var result ReuseTranslationMemoryResult
 if VisibleCharacterCount(input.SourceText)<input.MinimumCharacters{return result,nil}
 err:=language.ReadData(ctx,op,func(tx *sql.Tx)error{var err error;result,err=ReuseInSnapshot(ctx,tx,op,input);return err});return result,err
}
func ReuseInSnapshot(ctx context.Context,tx *sql.Tx,op OperationContext,input ReuseTranslationMemoryInput)(ReuseTranslationMemoryResult,error){
 if VisibleCharacterCount(input.SourceText)<input.MinimumCharacters{return ReuseTranslationMemoryResult{},nil}
 var translationID,unitID string
 err:=tx.QueryRowContext(ctx,`SELECT u.translation_id,u.unit_id FROM translation_units u JOIN translation_manifests m ON m.translation_id=u.translation_id CROSS JOIN domain_versions v WHERE v.singleton=1 AND u.source_hash=? AND u.source_text=? AND u.target_language=? AND u.status='normal' AND m.state='ready' AND m.glossary_generation=v.glossary_generation AND u.data_epoch=? AND m.data_epoch=? ORDER BY m.generation DESC,u.translation_id,u.unit_id LIMIT 1`,input.SourceHash,input.SourceText,input.TargetLanguage,op.DataEpoch,op.DataEpoch).Scan(&translationID,&unitID)
 if errors.Is(err,sql.ErrNoRows){return ReuseTranslationMemoryResult{},nil};if err!=nil{return ReuseTranslationMemoryResult{},language.StorageError(err)}
 object,err:=ReadObject(ctx,tx,op,translationID);if err!=nil{return ReuseTranslationMemoryResult{},err}
 for _,stored:=range object.Units{if stored.UnitId==unitID{if stored.SourceHash!=input.SourceHash||stored.SourceText==nil||*stored.SourceText!=input.SourceText||stored.Status!="normal"{return ReuseTranslationMemoryResult{},platform.NewError(platform.CodeIntegrityError,false,"翻译记忆索引与权威正文不一致")};unit:=input.Candidate;unit.TargetText=stored.TargetText;unit.TargetHash=stored.TargetHash;unit.Origin="machine";unit.Status="normal";return ReuseTranslationMemoryResult{Unit:&unit,Reused:true},nil}}
 return ReuseTranslationMemoryResult{},platform.NewError(platform.CodeIntegrityError,false,"翻译记忆权威对象缺少索引单元")
}
