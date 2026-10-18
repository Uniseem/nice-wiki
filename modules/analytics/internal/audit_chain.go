package analytics

import (
 "context"
 "nice-wiki/libs/platform"
)

// ComputeAuditHash 的输入不包含链自身hash，避免自引用。
func ComputeAuditHash(ctx context.Context,op OperationContext,in ComputeAuditHashInput)(ComputeAuditHashResult,error){
 record:=in.AuditJSON
 if record.SchemaVersion<0||record.SchemaVersion>2{return ComputeAuditHashResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"审计链版本不支持")}
 fields:=map[string]any{"auditId":record.AuditID,"actorId":record.ActorID,"action":record.Action,"resourceId":record.ResourceID,"resultCode":record.ResultCode,"occurredAt":record.OccurredAt,"metadata":record.Metadata}
 if record.SchemaVersion>=2{fields["schemaVersion"]=2;fields["siteId"]=record.SiteID;fields["producer"]=record.Producer;fields["actorKind"]=record.ActorKind;fields["sourceOperationId"]=record.SourceOperationID;fields["originEpoch"]=record.OriginEpoch}
 raw,err:=platform.CanonicalJSON(fields);if err!=nil{return ComputeAuditHashResult{},err}
 return ComputeAuditHashResult{Hash:platform.HashBytes(append([]byte(in.PreviousHash),raw...))},nil
}
