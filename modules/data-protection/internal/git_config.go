package protection

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "math"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/data-protection/internal/gen"
)

type ConfigureRemoteInput struct{URL string `json:"url"`;Branch string `json:"branch"`;CredentialRef string `json:"credentialRef"`;ExpectedRevision int64 `json:"expectedRevision"`;Actor platform.AuthContext `json:"-"`}
type ConfigureRemoteResult=gen.ConfigureRemoteResult

// ConfigureRemote 权限已由HTTP边界即时核验；保存可信凭据ID，不读取请求指定的系统路径。
func ConfigureRemote(ctx context.Context,op OperationContext,in ConfigureRemoteInput)(ConfigureRemoteResult,error){
 if err:=requireAdministrator(op);err!=nil{return ConfigureRemoteResult{},err};store,err:=Catalog(op);if err!=nil{return ConfigureRemoteResult{},err};if _,err=ValidateRemote(ctx,op,ValidateRemoteInput{URL:in.URL,Branch:in.Branch});err!=nil{return ConfigureRemoteResult{},err};if in.CredentialRef!=""&&store.GitCredentialRefs[in.CredentialRef]==""{return ConfigureRemoteResult{},platform.NewError(platform.CodeForbidden,false,"凭据引用未在部署登记")}
 raw,err:=platform.CanonicalJSON(in);if err!=nil{return ConfigureRemoteResult{},err};inputHash:=platform.HashBytes(raw);tx,err:=store.DB.BeginTx(ctx,nil);if err!=nil{return ConfigureRemoteResult{},storageError(err)};defer tx.Rollback();var priorHash,priorResult string
 err=tx.QueryRowContext(ctx,`SELECT input_hash,result_json FROM operation_receipts WHERE subject_id=? AND function_id='data-protection.ConfigureRemote' AND operation_id=?`,op.Actor.SubjectID,op.OperationID).Scan(&priorHash,&priorResult);if err==nil{if priorHash!=inputHash{return ConfigureRemoteResult{},platform.NewError(platform.CodeConflict,false,"同操作原输入不一致")};var result ConfigureRemoteResult;if json.Unmarshal([]byte(priorResult),&result)!=nil{return ConfigureRemoteResult{},platform.NewError(platform.CodeIntegrityError,false,"远程配置原回执损坏")};return result,nil};if !errors.Is(err,sql.ErrNoRows){return ConfigureRemoteResult{},storageError(err)}
 var revision int64;err=tx.QueryRowContext(ctx,`SELECT revision FROM remote_config WHERE config_id=1`).Scan(&revision);if err!=nil&&!errors.Is(err,sql.ErrNoRows){return ConfigureRemoteResult{},storageError(err)};if revision!=in.ExpectedRevision{return ConfigureRemoteResult{},platform.NewError(platform.CodeConflict,false,"远程配置已被其他操作更新")};if revision==math.MaxInt64{return ConfigureRemoteResult{},platform.NewError(platform.CodeBudgetExceeded,false,"远程配置修订号已达上限")};revision++
 _,err=tx.ExecContext(ctx,`INSERT INTO remote_config(config_id,url_without_credentials,branch,credential_ref,revision,approved_by,updated_at) VALUES(1,?,?,?,?,?,?) ON CONFLICT(config_id) DO UPDATE SET url_without_credentials=excluded.url_without_credentials,branch=excluded.branch,credential_ref=excluded.credential_ref,revision=excluded.revision,approved_by=excluded.approved_by,updated_at=excluded.updated_at`,in.URL,in.Branch,in.CredentialRef,revision,op.Actor.SubjectID,platform.Now());if err!=nil{return ConfigureRemoteResult{},storageError(err)};result:=ConfigureRemoteResult{Revision:revision};resultJSON,err:=platform.CanonicalJSON(result);if err!=nil{return ConfigureRemoteResult{},err};_,err=tx.ExecContext(ctx,`INSERT INTO operation_receipts(subject_id,function_id,operation_id,input_hash,result_json,created_at) VALUES(?,'data-protection.ConfigureRemote',?,?,?,?)`,op.Actor.SubjectID,op.OperationID,inputHash,string(resultJSON),platform.Now());if err!=nil{return ConfigureRemoteResult{},storageError(err)};if err=tx.Commit();err!=nil{return ConfigureRemoteResult{},storageError(err)};return result,nil
}
