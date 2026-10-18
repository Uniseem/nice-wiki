package trafficguard

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/serviceauth"
)

type SignPermitInput struct { AdmissionID string;ExpiresAt *string }
type SignPermitResult struct { Token string;Permit platform.AdmissionPermit }
type ValidatePermitInput struct { Token string;ExpectedAction string;CallerService string;DataEpoch string;RequestID string }
type ValidatePermitResult struct { AdmissionID string;Valid bool;ExpiresAt string }

// SignPermit 只有同根消费后的QA持久行能生成模型凭证；分配阶段不签名。
func SignPermit(ctx context.Context,op OperationContext,in SignPermitInput)(SignPermitResult,error) {
 t,err:=beginGuardTransaction(ctx,op,false)
 if err!=nil { return SignPermitResult{},err }
 defer t.close()
 row,err:=readPermitRecord(t,in.AdmissionID)
 if err!=nil { return SignPermitResult{},err }
 if op.CallerService!="qa"||row.owner!="qa"||row.consumed!=row.requestID||row.requestID!=op.RootOperationID { return SignPermitResult{},platform.NewError(platform.CodeForbidden,false,"只有原根已消费问答许可可签模型凭证") }
 if row.state!="active"||row.expires<=t.now.UnixNano() { return SignPermitResult{},platform.NewError(platform.CodeStaleResult,false,"已终止许可不得重签") }
 permit:=permitMetadata(row,op.DataEpoch)
 if in.ExpiresAt!=nil {
  expiry,err:=platform.ParseTimestamp(*in.ExpiresAt)
  if err!=nil { return SignPermitResult{},err }
  if expiry.UnixNano()>row.expires||expiry.UnixNano()>row.hardDeadline||!expiry.After(t.now) { return SignPermitResult{},platform.NewError(platform.CodeStaleResult,false,"原回执期限不在当前已获准租约内") }
  permit.ExpiresAt=expiry.UTC().Format(time.RFC3339Nano)
 }
 signed,err:=serviceauth.SignAdmissionPermit(t.runtime.SigningKey,permit)
 if err!=nil { return SignPermitResult{},err }
 return SignPermitResult{Token:signed.Token,Permit:signed},nil
}

// ValidatePermit 验原签名后逐持久字段核对，不把二跳qa服务主体替代匿名原主体。
func ValidatePermit(ctx context.Context,op OperationContext,in ValidatePermitInput)(ValidatePermitResult,error) {
 if op.CallerService!="qa"||in.CallerService!=op.CallerService||in.RequestID!=op.RootOperationID { return ValidatePermitResult{},platform.NewError(platform.CodeForbidden,false,"许可验证必须由原QA根调用") }
 runtime,ok:=ctx.Value(guardRuntimeKey{}).(GuardRuntime)
 if !ok { return ValidatePermitResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"风控验签配置未装配") }
 signed,err:=serviceauth.VerifyAdmissionPermit(runtime.Anchors,in.Token)
 if err!=nil { return ValidatePermitResult{},err }
 t,err:=beginGuardTransaction(ctx,op,false)
 if err!=nil { return ValidatePermitResult{},err }
 defer t.close()
 row,err:=readPermitRecord(t,signed.AdmissionID)
 if err!=nil { return ValidatePermitResult{},err }
 if row.owner!="qa"||row.consumed!=row.requestID||row.requestID!=in.RequestID||signed.RequestID!=in.RequestID||signed.DataEpoch!=op.DataEpoch||(in.ExpectedAction!=""&&signed.Action!=in.ExpectedAction) { return ValidatePermitResult{},platform.NewError(platform.CodeForbidden,false,"签名许可不属于原QA主体和根") }
 expiry,err:=platform.ParseTimestamp(signed.ExpiresAt)
 if err!=nil { return ValidatePermitResult{},err }
 expected:=permitMetadata(row,op.DataEpoch)
 expected.ExpiresAt=signed.ExpiresAt
 expected.Token=signed.Token
 if expected!=signed||expiry.UnixNano()>row.expires||expiry.UnixNano()>row.hardDeadline { return ValidatePermitResult{},platform.NewError(platform.CodeForbidden,false,"签名许可与持久绑定不一致") }
 if row.state!="active"||row.expires<=t.now.UnixNano() { return ValidatePermitResult{AdmissionID:row.admissionID,Valid:false,ExpiresAt:signed.ExpiresAt},nil }
 banned,_,err:=CheckBan(t,row.networks)
 if err!=nil { return ValidatePermitResult{},err }
 return ValidatePermitResult{AdmissionID:row.admissionID,Valid:!banned,ExpiresAt:signed.ExpiresAt},nil
}
