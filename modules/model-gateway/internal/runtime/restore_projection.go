package runtime

import (
 "context"
 "database/sql"
 "encoding/json"
 "strconv"
 "nice-wiki/modules/model-gateway/internal/gen"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
)

// IsRestoredEpoch只认证域迁移记录的完整原清单；不重基付费JSON/执行hash，也不授权新派发。
func IsRestoredEpoch(ctx context.Context,tx *sql.Tx,source,target string)(bool,error){
 if source==target{return true,nil};var raw []byte;var hash,binding string
 err:=tx.QueryRowContext(ctx,`SELECT m.manifest_json,m.manifest_hash,s.proof_hash FROM restored_epoch_sources s JOIN restored_epoch_manifests m ON m.manifest_hash=s.manifest_hash WHERE s.source_epoch=? AND s.target_epoch=?`,source,target).Scan(&raw,&hash,&binding);if err==sql.ErrNoRows{return false,nil};if err!=nil{return false,ExecutionStorageError(err)};var manifest lifecycle.DomainSnapshotManifest
 if platform.HashBytes(raw)!=hash||json.Unmarshal(raw,&manifest)!=nil||manifest.Owner!="model-gateway"||manifest.SchemaVersion!=2||manifest.Invocations==nil{return false,platform.NewError(platform.CodeIntegrityError,false,"恢复来源完整清单证明损坏")};proof,_:=platform.CanonicalJSON([]string{source,target,hash});if !platform.ValidIdentifier(source)||!platform.ValidIdentifier(target)||platform.HashBytes(proof)!=binding{return false,platform.NewError(platform.CodeIntegrityError,false,"恢复来源映射原source/target/manifest绑定损坏")};return true,nil
}

func IsRedactedMaterial(ctx context.Context,tx *sql.Tx,kind,key string)(bool,error){
 var databaseHash,materialHash string;err:=tx.QueryRowContext(ctx,`SELECT source_database_hash,source_material_hash FROM snapshot_material_redactions WHERE kind=? AND record_key=?`,kind,key).Scan(&databaseHash,&materialHash);if err==sql.ErrNoRows{return false,nil};if err!=nil{return false,ExecutionStorageError(err)};if !platform.ValidHash(databaseHash)||!platform.ValidHash(materialHash){return false,platform.NewError(platform.CodeIntegrityError,false,"明确省略密钥material的原来源摘要损坏")};return true,nil
}

// CheckProviderMaterial只在新接受事务核真实当前加密材料；明确脱密要求管理员新版本配置，不按空密钥派发。
func CheckProviderMaterial(ctx context.Context,tx *sql.Tx,op OperationContext,provider gen.Provider)error{
 var epoch string;var nonce,ciphertext int;err:=tx.QueryRowContext(ctx,`SELECT data_epoch,length(nonce),length(ciphertext) FROM credentials WHERE provider_id=? AND credential_version=?`,provider.ProviderId,provider.CredentialVersion).Scan(&epoch,&nonce,&ciphertext);if err==sql.ErrNoRows{return platform.NewError(platform.CodeDependencyUnavailable,false,"渠道没有当前凭据material")};if err!=nil{return ExecutionStorageError(err)};if epoch!=op.DataEpoch{return platform.NewError(platform.CodeStaleResult,false,"当前凭据代际不同")};if nonce!=12||ciphertext<=16{redacted,e:=IsRedactedMaterial(ctx,tx,"credential",provider.ProviderId+"|"+strconv.Itoa(provider.CredentialVersion));if e!=nil{return e};if redacted&&nonce==0&&ciphertext==0{return platform.NewError(platform.CodeDependencyUnavailable,false,"备份未包含渠道密钥，需管理员配置新版本")};return platform.NewError(platform.CodeIntegrityError,false,"渠道加密材料损坏")}
 for i,ref:=range provider.ExtraHeaderSecretRefs{var owner,name string;err=tx.QueryRowContext(ctx,`SELECT provider_id,header_name,data_epoch,length(nonce),length(ciphertext) FROM header_secrets WHERE ref_id=?`,ref).Scan(&owner,&name,&epoch,&nonce,&ciphertext);if err!=nil{return ExecutionStorageError(err)};projected,e:=IsRestoredEpoch(ctx,tx,epoch,op.DataEpoch);if e!=nil{return e};if owner!=provider.ProviderId||i>=len(provider.ExtraHeaderNames)||name!=provider.ExtraHeaderNames[i]||!projected{return platform.NewError(platform.CodeIntegrityError,false,"渠道额外头来源不属于本次配置")};if nonce!=12||ciphertext<=16{redacted,e:=IsRedactedMaterial(ctx,tx,"header",ref);if e!=nil{return e};if redacted&&nonce==0&&ciphertext==0{return platform.NewError(platform.CodeDependencyUnavailable,false,"备份未包含额外头密钥，需管理员重新配置")};return platform.NewError(platform.CodeIntegrityError,false,"渠道额外头加密材料损坏")}}
 return nil
}
