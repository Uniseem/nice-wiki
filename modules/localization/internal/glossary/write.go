package glossary

import (
 "context"
 "database/sql"
 "errors"
 "strings"
 "unicode/utf8"
 "golang.org/x/text/unicode/norm"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
)

type PutGlossaryTermInput=gen.PutGlossaryTermInput
type PutGlossaryTermResult=gen.PutGlossaryTermResult
type DeleteGlossaryTermInput struct{TermId string `json:"termId"`;ExpectedVersion int `json:"expectedVersion"`;IdempotencyKey string `json:"idempotencyKey"`}
type DeleteGlossaryTermResult=gen.DeleteGlossaryTermResult
func NormalizeTerm(ctx context.Context,op OperationContext,t gen.GlossaryTerm)(gen.GlossaryTerm,error){
 a,err:=language.NormalizeLanguageTag(ctx,op,language.NormalizeLanguageTagInput{Tag:t.SourceLanguage});if err!=nil{return t,err};b,err:=language.NormalizeLanguageTag(ctx,op,language.NormalizeLanguageTagInput{Tag:t.TargetLanguage});if err!=nil{return t,err};t.SourceLanguage=a.Tag;t.TargetLanguage=b.Tag;t.SourceTerm=norm.NFC.String(strings.TrimSpace(t.SourceTerm));t.TargetTerm=norm.NFC.String(strings.TrimSpace(t.TargetTerm))
 if t.SourceTerm==""||t.TargetTerm==""||utf8.RuneCountInString(t.SourceTerm)>256||utf8.RuneCountInString(t.TargetTerm)>256{return t,platform.NewError(platform.CodeValidation,false,"术语必须为1到256个字符")}
 if (t.Scope=="document"&&(t.DocumentId==nil||!platform.ValidIdentifier(*t.DocumentId)))||(t.Scope=="global"&&t.DocumentId!=nil){return t,platform.NewError(platform.CodeValidation,false,"术语范围与文档字段不匹配")};return t,nil
}
func PutGlossaryTerm(ctx context.Context,op OperationContext,input PutGlossaryTermInput)(PutGlossaryTermResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return PutGlossaryTermResult{},err};normalized,err:=NormalizeTerm(ctx,op,input.Term);if err!=nil{return PutGlossaryTermResult{},err};normalized.Source="manual";normalized.ExtractionBatchId=nil;var result PutGlossaryTermResult
 err=language.Mutate(ctx,op,"localization.PutGlossaryTerm",input.IdempotencyKey,input,&result,func(tx *sql.Tx)error{var err error;result.Term,result.Generation,err=StoreTerm(ctx,tx,op,normalized,input.ExpectedVersion);return err});return result,err
}
func StoreTerm(ctx context.Context,tx *sql.Tx,op OperationContext,t gen.GlossaryTerm,expected int)(gen.GlossaryTerm,int,error){
 var oldVersion int;var oldSource,oldTarget,oldEpoch string
 err:=tx.QueryRowContext(ctx,`SELECT version,source_language,target_language,data_epoch FROM glossary WHERE term_id=?`,t.TermId).Scan(&oldVersion,&oldSource,&oldTarget,&oldEpoch);if err!=nil&&!errors.Is(err,sql.ErrNoRows){return t,0,language.StorageError(err)};if err==nil&&oldEpoch!=op.DataEpoch{return t,0,platform.NewError(platform.CodeVersionMismatch,false,"术语数据代次已变化")};if oldVersion!=expected{return t,0,platform.NewError(platform.CodeVersionMismatch,false,"术语版本已变化")}
 document:="";if t.DocumentId!=nil{document=*t.DocumentId};var duplicate string
 err=tx.QueryRowContext(ctx,`SELECT term_id FROM glossary WHERE source_language=? AND target_language=? AND scope=? AND COALESCE(document_id,'')=? AND source_term=? AND deleted=0 AND term_id<>?`,t.SourceLanguage,t.TargetLanguage,t.Scope,document,t.SourceTerm,t.TermId).Scan(&duplicate);if err==nil{return t,0,platform.NewError(platform.CodeConflict,false,"同范围原词已存在术语")};if !errors.Is(err,sql.ErrNoRows){return t,0,language.StorageError(err)}
 generation,err:=AdvanceGeneration(ctx,tx);if err!=nil{return t,0,err};t.Version=oldVersion+1;t.Generation=generation
 _,err=tx.ExecContext(ctx,`INSERT INTO glossary(term_id,source_language,target_language,source_term,target_term,scope,document_id,version,generation,source,extraction_batch_id,data_epoch,created_at,updated_at,deleted) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,0) ON CONFLICT(term_id) DO UPDATE SET source_language=excluded.source_language,target_language=excluded.target_language,source_term=excluded.source_term,target_term=excluded.target_term,scope=excluded.scope,document_id=excluded.document_id,version=excluded.version,generation=excluded.generation,source=excluded.source,extraction_batch_id=excluded.extraction_batch_id,updated_at=excluded.updated_at,deleted=0`,t.TermId,t.SourceLanguage,t.TargetLanguage,t.SourceTerm,t.TargetTerm,t.Scope,t.DocumentId,t.Version,t.Generation,t.Source,t.ExtractionBatchId,op.DataEpoch,platform.Now(),platform.Now());if err!=nil{return t,0,language.StorageError(err)}
 if err=PublishChange(ctx,tx,op,t.SourceLanguage,t.TargetLanguage,generation,[]string{t.TermId},string(t.Source));err!=nil{return t,0,err}
 if oldSource!=""&&(oldSource!=t.SourceLanguage||oldTarget!=t.TargetLanguage){if err=PublishChange(ctx,tx,op,oldSource,oldTarget,generation,[]string{t.TermId},string(t.Source));err!=nil{return t,0,err}}
 return t,generation,nil
}
func AdvanceGeneration(ctx context.Context,tx *sql.Tx)(int,error){_,err:=tx.ExecContext(ctx,`UPDATE domain_versions SET glossary_generation=glossary_generation+1 WHERE singleton=1`);if err!=nil{return 0,language.StorageError(err)};return Generation(ctx,tx)}
func DeleteGlossaryTerm(ctx context.Context,op OperationContext,input DeleteGlossaryTermInput)(DeleteGlossaryTermResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return DeleteGlossaryTermResult{},err};var result DeleteGlossaryTermResult
 err:=language.Mutate(ctx,op,"localization.DeleteGlossaryTerm",input.IdempotencyKey,input,&result,func(tx *sql.Tx)error{
  t,err:=scanTerm(tx.QueryRowContext(ctx,`SELECT `+termColumns+` FROM glossary WHERE term_id=? AND deleted=0 AND data_epoch=?`,input.TermId,op.DataEpoch));if err!=nil{return err};if t.Version!=input.ExpectedVersion{return platform.NewError(platform.CodeVersionMismatch,false,"术语版本已变化")}
  generation,err:=AdvanceGeneration(ctx,tx);if err!=nil{return err};_,err=tx.ExecContext(ctx,`UPDATE glossary SET deleted=1,version=version+1,generation=?,updated_at=? WHERE term_id=? AND version=? AND data_epoch=? AND deleted=0`,generation,platform.Now(),t.TermId,t.Version,op.DataEpoch);if err!=nil{return language.StorageError(err)};result=DeleteGlossaryTermResult{Deleted:true,Generation:generation};return PublishChange(ctx,tx,op,t.SourceLanguage,t.TargetLanguage,generation,[]string{t.TermId},string(t.Source))
 });return result,err
}
func PublishChange(ctx context.Context,tx *sql.Tx,op OperationContext,source,target string,generation int,ids []string,origin string)error{
 if _,err:=HandleGlossaryChange(ctx,tx,op,source,target,generation);err!=nil{return err}
 return language.EmitReference(tx,op,"glossary.changed","glossary",generation,map[string]any{"generation":generation,"sourceLanguage":source,"targetLanguage":target,"termIds":ids,"source":origin,"dataEpoch":op.DataEpoch})
}
