package translation

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
)

type EditTranslationUnitInput=gen.EditTranslationUnitInput
type EditTranslationUnitResult=gen.EditTranslationUnitResult
type SetUnitProtectionInput=gen.SetUnitProtectionInput
type SetUnitProtectionResult=gen.SetUnitProtectionResult
func EditTranslationUnit(ctx context.Context,op OperationContext,input EditTranslationUnitInput)(EditTranslationUnitResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return EditTranslationUnitResult{},err};var result EditTranslationUnitResult
 err:=language.Mutate(ctx,op,"localization.EditTranslationUnit",input.IdempotencyKey,input,&result,func(tx *sql.Tx)error{
  object,err:=ReadObject(ctx,tx,op,input.TranslationId);if err!=nil{return err};found:=-1
  for i:=range object.Units{if object.Units[i].UnitId==input.UnitId{found=i;break}};if found<0{return platform.NewError(platform.CodeNotFound,false,"译文单元不存在")};unit:=&object.Units[found]
  if unit.Version!=input.ExpectedVersion{return platform.NewError(platform.CodeVersionMismatch,false,"译文单元版本已变化")};if unit.SourceText==nil{return platform.NewError(platform.CodeDependencyUnavailable,false,"译文单元缺少来源，不能核验插值")};if err=CheckInlineTokens(*unit.SourceText,input.TargetText);err!=nil{return err}
  unit.TargetText=input.TargetText;unit.TargetHash=platform.HashBytes([]byte(input.TargetText));unit.Version++;unit.HumanGeneration++;unit.Origin="human";unit.Protected=true;unit.Status="normal"
  saved,err:=WriteObject(ctx,tx,op,object,object.Translation.Generation);if err!=nil{return err};result.Unit=saved.Units[found]
  _,err=tx.ExecContext(ctx,`UPDATE translation_jobs SET human_generation=human_generation+1,state=CASE WHEN state IN('queued','running') THEN 'superseded' ELSE state END,updated_at=? WHERE job_id=? AND data_epoch=?`,platform.Now(),input.TranslationId,op.DataEpoch);return language.StorageError(err)
 });return result,err
}
func SetUnitProtection(ctx context.Context,op OperationContext,input SetUnitProtectionInput)(SetUnitProtectionResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return SetUnitProtectionResult{},err};var result SetUnitProtectionResult
 err:=language.Mutate(ctx,op,"localization.SetUnitProtection",input.IdempotencyKey,input,&result,func(tx *sql.Tx)error{
  object,err:=ReadObject(ctx,tx,op,input.TranslationId);if err!=nil{return err};found:=-1;for i:=range object.Units{if object.Units[i].UnitId==input.UnitId{found=i;break}};if found<0{return platform.NewError(platform.CodeNotFound,false,"译文单元不存在")};unit:=&object.Units[found]
  if unit.Version!=input.ExpectedVersion{return platform.NewError(platform.CodeVersionMismatch,false,"译文单元版本已变化")};unit.Protected=input.Protected;unit.Version++;unit.HumanGeneration++
  saved,err:=WriteObject(ctx,tx,op,object,object.Translation.Generation);if err!=nil{return err};result.Unit=saved.Units[found]
  _,err=tx.ExecContext(ctx,`UPDATE translation_jobs SET human_generation=human_generation+1,state=CASE WHEN state IN('queued','running') THEN 'superseded' ELSE state END,updated_at=? WHERE job_id=? AND data_epoch=?`,platform.Now(),input.TranslationId,op.DataEpoch);return language.StorageError(err)
 });return result,err
}
