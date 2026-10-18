package protection

import (
 "context"
 "path/filepath"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
)

type ValidateBackupRequestInput struct{IncludeSecrets bool `json:"includeSecrets"`;Actor platform.AuthContext `json:"-"`;DataEpoch string `json:"dataEpoch"`}
type ValidateBackupRequestResult struct{Valid bool `json:"valid"`;CapacityVerified bool `json:"capacityVerified"`}

// ValidateBackupRequest 只证明基础前置条件；真实容量待14域清单齐全后核预算，不用未知零字节冒充分配成功。
func ValidateBackupRequest(ctx context.Context,op OperationContext,in ValidateBackupRequestInput)(ValidateBackupRequestResult,error){
 if err:=requireAdministrator(op);err!=nil{return ValidateBackupRequestResult{},err};store,err:=Catalog(op);if err!=nil{return ValidateBackupRequestResult{},err};if in.DataEpoch!=op.DataEpoch{return ValidateBackupRequestResult{},platform.NewError(platform.CodeVersionMismatch,false,"备份请求属于其他数据代次")}
 if in.IncludeSecrets{if store.Authorize==nil{return ValidateBackupRequestResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"敏感备份授权端口未装配")};if err=store.Authorize(ctx,op,"superadmin","createBackup");err!=nil{return ValidateBackupRequestResult{},err}}
 if store.Runtime==nil{return ValidateBackupRequestResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"备份运行权威未装配")};view,err:=store.Runtime(ctx);if err!=nil{return ValidateBackupRequestResult{},err};if view.DataEpoch!=op.DataEpoch{return ValidateBackupRequestResult{},platform.NewError(platform.CodeVersionMismatch,false,"运行代次已经变化")};if view.Phase!=platform.PhaseRunning||view.Maintenance||view.RestoreID!=""{return ValidateBackupRequestResult{},platform.NewError(platform.CodeLocked,true,"站点当前维护或恢复中，不能创建新备份")}
 if len(store.Keyring)!=1{return ValidateBackupRequestResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"备份本机密钥未配置")};for id:=range store.Keyring{key,keyErr:=configuredKEK(store,id);if keyErr!=nil{return ValidateBackupRequestResult{},keyErr};clear(key)}
 for _,name:=range []string{"control","vault","active/incoming"}{if err=lifecycle.ValidatePrivatePath(filepath.Join(store.Root,name),"directory",false);err!=nil{return ValidateBackupRequestResult{},err}}
 return ValidateBackupRequestResult{Valid:true,CapacityVerified:false},nil
}
