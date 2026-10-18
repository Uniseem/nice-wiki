package index

import (
 "context"
 "encoding/json"
 "nice-wiki/libs/platform"
)

// ResolveTaskMemory 只读原任务完整记忆，空文字保留真实版本而不生成替代内容。
func ResolveTaskMemory(ctx context.Context,op OperationContext,taskID,language string)(platform.MemoryContext,error) {
 runtime,err:=RuntimeFromContext(ctx);if err!=nil {return platform.MemoryContext{},err}
 validate:=runtime.Validators["MemoryResolveContextResult"]
 if runtime.Memory==nil||validate==nil {return platform.MemoryContext{},platform.NewError(platform.CodeDependencyUnavailable,false,"任务记忆权威未装配")}
 service:=op;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:op.Actor.SiteID,ExpiresAt:op.Deadline,Grants:[]string{}}
 raw,err:=runtime.Memory.ResolveContext(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),struct{TaskID string `json:"taskId"`;Language string `json:"language"`}{taskID,language})
 if err!=nil {return platform.MemoryContext{},err};if err=validate(raw);err!=nil {return platform.MemoryContext{},err}
 var result platform.MemoryContext
 if json.Unmarshal(raw,&result)!=nil||result.TaskID!=taskID||result.Language!=language||result.ByteSize!=int64(len(result.Text)) {return platform.MemoryContext{},platform.NewError(platform.CodeIntegrityError,false,"完整任务记忆来源或字节数不一致")}
 return result,nil
}
