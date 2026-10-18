package model

import (
 "context"
 "encoding/json"
 "net/url"
 "time"
 "nice-wiki/libs/platform"
)

// RootFeesFinal 只消费MG完整原root收费事实；缺字段/缺价/旧epoch/超预算均不推零费。
func RootFeesFinal(ctx context.Context,op OperationContext,rootID string)(bool,error) {
 runtime,ok:=ctx.Value(modelRuntimeKey{}).(ModelRuntime)
 if !ok||runtime.Raw==nil||runtime.ValidateUsage==nil {return false,platform.NewError(platform.CodeDependencyUnavailable,false,"原根收费权威只读未装配")}
 service:=op;service.CallerService="knowledge";service.RootOperationID=rootID;service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:op.Actor.SiteID,ExpiresAt:op.Deadline,Grants:[]string{}}
 until:=time.Now().UTC().Format(time.RFC3339Nano)
 raw,err:=runtime.Raw.QueryUsage(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),url.Values{"rootOperationId":{rootID},"from":{"1970-01-01T00:00:00Z"},"until":{until}})
 if err!=nil {return false,err};if err=runtime.ValidateUsage(raw);err!=nil {return false,err}
 type money struct{Currency string `json:"currency"`;Micros int64 `json:"micros"`}
 var proof struct {WindowCoversRoot *bool `json:"windowCoversRoot"`;AllInvocationsFinal *bool `json:"allInvocationsFinal"`;HeldCost *money `json:"heldCost"`;UnknownInvocationIDs []string `json:"unknownInvocationIds"`;Entries []struct{PricingKnown bool `json:"pricingKnown"`;Status string `json:"status"`;ActualCost *money `json:"actualCost"`} `json:"entries"`}
 if json.Unmarshal(raw,&proof)!=nil {return false,platform.NewError(platform.CodeIntegrityError,false,"原根费用完整证明不可读")}
 if proof.WindowCoversRoot==nil||!*proof.WindowCoversRoot||proof.AllInvocationsFinal==nil||!*proof.AllInvocationsFinal||proof.HeldCost==nil||proof.HeldCost.Currency!="USD"||proof.HeldCost.Micros!=0||len(proof.UnknownInvocationIDs)!=0 {return false,nil}
 for _,entry:=range proof.Entries {if !entry.PricingKnown||entry.Status=="price_unknown"||entry.ActualCost==nil {return false,nil}}
 return true,nil
}

// OwnerFeesFinal 仅原任务及尚未完成孩子的接受调用需要费用终局；跨根已完成付费缓存免费复用不新授费。
// 同本任务原root的已完成孩子仍由完整RootFeesFinal涵盖，paidRefs始终另完整保留。
func OwnerFeesFinal(ctx context.Context,op OperationContext,objectID,rootID string)(bool,error) {
 runtime,ok:=ctx.Value(modelRuntimeKey{}).(ModelRuntime);if !ok||runtime.DB==nil {return false,platform.NewError(platform.CodeDependencyUnavailable,false,"owner费用来源未装配")}
 rows,err:=runtime.DB.QueryContext(ctx,`SELECT m.owner_id,COALESCE((SELECT operation_json FROM build_dispatches WHERE build_id=m.owner_id),(SELECT operation_json FROM extraction_dispatches WHERE extraction_id=m.owner_id)),(EXISTS(SELECT 1 FROM execution_records f WHERE f.object_id=m.owner_id AND f.job_type='knowledge.extract' AND f.state='completed' AND f.result_ref_json IS NOT NULL) OR EXISTS(SELECT 1 FROM extraction_recovered_results r WHERE r.extraction_id=m.owner_id)) FROM model_steps m WHERE m.site_id=? AND m.data_epoch=? AND (m.owner_id=? OR m.owner_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?)) AND m.invocation_id IS NOT NULL AND m.invocation_id<>''`,op.Actor.SiteID,op.DataEpoch,objectID,objectID)
 if err!=nil {return false,platform.NormalizeIOError(err,platform.CodeInternal,false,"原接受调用费用来源不可读")}
 roots:=map[string]bool{}
 for rows.Next(){var owner string;var raw []byte;var completed bool
  if err=rows.Scan(&owner,&raw,&completed);err!=nil {rows.Close();return false,platform.NormalizeIOError(err,platform.CodeInternal,false,"原接受调用费用来源不可读")}
  var original OperationContext
  if json.Unmarshal(raw,&original)!=nil||original.RootOperationID==""||original.Actor.SiteID!=op.Actor.SiteID||original.CostClass!=platform.CostClassBackground {rows.Close();return false,platform.NewError(platform.CodeIntegrityError,false,"接受调用缺真实原根来源")}
  if completed&&owner!=objectID&&original.RootOperationID!=rootID {continue}
  if original.DataEpoch!=op.DataEpoch {rows.Close();return false,nil}
  roots[original.RootOperationID]=true
 }
 err=rows.Err();rows.Close();if err!=nil {return false,platform.NormalizeIOError(err,platform.CodeInternal,false,"原接受调用费用来源不可读")}
 for root:=range roots {final,err:=RootFeesFinal(ctx,op,root);if err!=nil {return false,err};if !final {return false,nil}}
 // 无已接受调用时只承认上层已核的prepared或专用未接受观察；不在此构造零金额。
 return true,nil
}
