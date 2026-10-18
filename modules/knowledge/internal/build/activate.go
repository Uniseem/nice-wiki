package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type ActivateBuildInput struct { BuildID string `json:"buildId"`;ExpectedActiveGeneration int `json:"expectedActiveGeneration"`;LeaseID string `json:"leaseId"` }
type ActivateBuildResult struct { Build gen.IndexBuild `json:"build"` }

// ActivateBuild 在新鲜权威证明和本域安全窗口内 CAS 指针，旧代数保留。
func ActivateBuild(ctx context.Context,op OperationContext,input ActivateBuildInput)(ActivateBuildResult,error){
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil{return ActivateBuildResult{},err}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建重放读取失败")}
 original,err:=indexer.ReadBuildRecord(ctx,read,op,input.BuildID);_=read.Rollback();if err!=nil{return ActivateBuildResult{},err}
 if string(original.Build.State)=="active" {return ActivateBuildResult{Build:original.Build},nil}
	work,tx,record,cancel,err:=indexer.BeginBuildWrite(ctx,op,input.BuildID,input.LeaseID);if err!=nil{return ActivateBuildResult{},err};defer cancel();defer tx.Rollback()
	var validatedAt string;var visibility int
	if err:=tx.QueryRowContext(work,`SELECT validated_at,visibility_revision FROM builds WHERE build_id=?`,input.BuildID).Scan(&validatedAt,&visibility);err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建激活证明读取失败")}
	observed,err:=platform.ParseTimestamp(validatedAt)
	if err!=nil||time.Since(observed)>time.Second||observed.After(time.Now())||record.ManifestHash==""||string(record.Build.State)!="validating"{return ActivateBuildResult{},platform.NewError(platform.CodeStaleResult,false,"构建缺少一秒内有效的权威证明")}
	_,hash,err:=indexer.ReadGeneration(work,tx,record);if err!=nil{return ActivateBuildResult{},err}
	if hash!=record.ManifestHash{return ActivateBuildResult{},platform.NewError(platform.CodeIntegrityError,false,"激活清单摘要不一致")}
	var previousID string;var generation int
	err=tx.QueryRowContext(work,`SELECT build_id,generation FROM active_indexes WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=?`,op.Actor.SiteID,op.DataEpoch,record.Build.Document.DocumentId,record.Build.Space.SpaceId).Scan(&previousID,&generation)
	if err!=nil&&err!=sql.ErrNoRows{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"活动代数读取失败")}
	if generation!=input.ExpectedActiveGeneration||record.Build.Generation<=generation {
		if _,err:=tx.ExecContext(work,`UPDATE builds SET state='superseded',generation_state='retired' WHERE build_id=?`,input.BuildID);err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"过期构建写入失败")}
		record.Build.State=gen.IndexBuildState("superseded")
	}else{
		if _,err:=tx.ExecContext(work,`INSERT INTO active_indexes(site_id,data_epoch,document_id,space_id,build_id,generation,visibility_revision) VALUES(?,?,?,?,?,?,?) ON CONFLICT(site_id,data_epoch,document_id,space_id) DO UPDATE SET build_id=excluded.build_id,generation=excluded.generation,visibility_revision=excluded.visibility_revision`,op.Actor.SiteID,op.DataEpoch,record.Build.Document.DocumentId,record.Build.Space.SpaceId,input.BuildID,record.Build.Generation,visibility);err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"活动指针写入失败")}
		if previousID!="" {if _,err:=tx.ExecContext(work,`UPDATE builds SET generation_state='retired' WHERE build_id=?`,previousID);err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"旧代数保留状态写入失败")}}
		if _,err:=tx.ExecContext(work,`UPDATE builds SET state='active',generation_state='active' WHERE build_id=?`,input.BuildID);err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"活动构建写入失败")}
		payload,_:=json.Marshal(struct{BuildID string `json:"buildId"`;Document gen.PublishedRevisionRef `json:"document"`;Space gen.EmbeddingSpace `json:"space"`;Hash string `json:"indexVersionHash"`;Epoch string `json:"dataEpoch"`}{input.BuildID,record.Build.Document,record.Build.Space,hash,op.DataEpoch})
		if err:=outbox.EnqueueEvent(tx,platform.EventEnvelope{EventID:"index.ready."+platform.HashBytes([]byte(input.BuildID)),EventType:"index.ready",EventVersion:1,Producer:"knowledge",SubjectID:record.Build.Document.DocumentId,Revision:int64(record.Build.Generation),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:payload});err!=nil{return ActivateBuildResult{},err}
		record.Build.State=gen.IndexBuildState("active")
	}
	if time.Since(observed)>time.Second{return ActivateBuildResult{},platform.NewError(platform.CodeStaleResult,false,"激活提交前权威证明已到期")}
	if err:=tx.Commit();err!=nil{return ActivateBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建激活提交失败")}
	return ActivateBuildResult{Build:record.Build},nil
}
