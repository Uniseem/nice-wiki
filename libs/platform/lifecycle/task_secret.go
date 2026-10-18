package lifecycle

import (
 "crypto/aes"
 "crypto/cipher"
 "crypto/rand"
 "io"
 "os"
 "path/filepath"
 "nice-wiki/libs/platform"
)

const maxTaskSecretBytes = 64 << 10
// TaskSecretBinding只取原持久任务身份；绝不把恢复target epoch重写进原secret绑定。
type TaskSecretBinding struct{Owner string `json:"owner"`;SiteID string `json:"siteId"`;DataEpoch string `json:"dataEpoch"`;RootOperationID string `json:"rootOperationId"`;Purpose string `json:"purpose"`}
type TaskSecret struct{Nonce []byte `json:"nonce"`;Ciphertext []byte `json:"ciphertext"`}
// SealTaskSecret返回随机nonce和认证密文；持久位置及访问授权仍由本域决定。
func SealTaskSecret(raw []byte,keyRef string,binding TaskSecretBinding)(TaskSecret,error){
 if len(raw)>maxTaskSecretBytes{return TaskSecret{},platform.NewError(platform.CodeBudgetExceeded,false,"任务秘密超过64KiB")};aad,err:=taskSecretAAD(binding);if err!=nil{return TaskSecret{},err};aead,err:=taskSecretCipher(keyRef);if err!=nil{return TaskSecret{},err};nonce:=make([]byte,aead.NonceSize());if _,err=io.ReadFull(rand.Reader,nonce);err!=nil{return TaskSecret{},platform.NewError(platform.CodeInternal,false,"任务秘密随机源不可用")};return TaskSecret{Nonce:nonce,Ciphertext:aead.Seal(nil,nonce,raw,aad)},nil
}
// OpenTaskSecret只认证相同owner/site/原epoch/root/purpose；不验证或延长其中permit的有效期。
func OpenTaskSecret(secret TaskSecret,keyRef string,binding TaskSecretBinding)([]byte,error){
 aad,err:=taskSecretAAD(binding);if err!=nil{return nil,err};aead,err:=taskSecretCipher(keyRef);if err!=nil{return nil,err};if len(secret.Nonce)!=aead.NonceSize()||len(secret.Ciphertext)<aead.Overhead()||len(secret.Ciphertext)>maxTaskSecretBytes+aead.Overhead(){return nil,platform.NewError(platform.CodeIntegrityError,false,"任务秘密密文边界不合法")};raw,err:=aead.Open(nil,secret.Nonce,secret.Ciphertext,aad);if err!=nil{return nil,platform.NewError(platform.CodeIntegrityError,false,"任务秘密认证失败")};return raw,nil
}
func taskSecretAAD(binding TaskSecretBinding)([]byte,error){if binding.Owner==""||len(binding.Owner)>64||!platform.ValidIdentifier(binding.SiteID)||!platform.ValidIdentifier(binding.DataEpoch)||!platform.ValidIdentifier(binding.RootOperationID)||binding.Purpose==""||len(binding.Purpose)>256{return nil,platform.NewError(platform.CodeValidation,false,"任务秘密缺真实原任务作用域")};return platform.CanonicalJSON(struct{Version int `json:"version"`;Binding TaskSecretBinding `json:"binding"`}{Version:1,Binding:binding})}
func taskSecretCipher(keyRef string)(cipher.AEAD,error){
 if keyRef==""||!filepath.IsAbs(keyRef){return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密私有key引用未配置")};if err:=ValidatePrivatePath(keyRef,"regular-file",false);err!=nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密私有key不可读取")};file,err:=os.Open(keyRef);if err!=nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密私有key不可读取")};defer file.Close();info,err:=file.Stat();if err!=nil||info.Mode().Perm()&0077!=0||info.Size()!=32{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密key必须为私有原始32字节")};key,err:=io.ReadAll(io.LimitReader(file,33));if err!=nil||len(key)!=32{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密私有key长度或读取不完整")};defer func(){for i:=range key{key[i]=0}}();block,err:=aes.NewCipher(key);if err!=nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"任务秘密AES256 key不可用")};aead,err:=cipher.NewGCM(block);if err!=nil{return nil,platform.NewError(platform.CodeInternal,false,"任务秘密认证算法不可用")};return aead,nil
}
