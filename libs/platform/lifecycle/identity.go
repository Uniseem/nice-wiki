package lifecycle

import (
 "bytes"
 "crypto/ed25519"
 "encoding/base64"
 "encoding/json"
 "io"
 "os"
 "path/filepath"

 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/serviceauth"
)

// LoadServiceIdentity 是各域共用的启动身份加载边界，不读取业务配置或模型密钥。
func LoadServiceIdentity(config platform.ServiceConfig) (platform.TrustAnchors, platform.SigningKey, error) {
 var anchors platform.TrustAnchors
 var key platform.SigningKey
 if err:=readIdentityConfig(config.TrustAnchorRef,&anchors);err!=nil{return anchors,key,err}
 if err:=readIdentityConfig(config.SigningKeyRef,&key);err!=nil{return anchors,key,err}
 if len(anchors.Services)==0 || key.KeyID=="" || !filepath.IsAbs(key.PrivateKeyRef) {return platform.TrustAnchors{},platform.SigningKey{},platform.NewError(platform.CodeIntegrityError,false,"服务身份材料不完整")}
 private,err:=serviceauth.LoadSigningKey(key)
 if err!=nil{return platform.TrustAnchors{},platform.SigningKey{},err}
 public,err:=base64.RawURLEncoding.DecodeString(anchors.ServiceKeyID(config.Module,key.KeyID))
 if err!=nil || !bytes.Equal(public,private.Public().(ed25519.PublicKey)) {return platform.TrustAnchors{},platform.SigningKey{},platform.NewError(platform.CodeIntegrityError,false,"服务签名身份与本站信任锚不一致")}
 return anchors,key,nil
}

func readIdentityConfig(ref string,target any) error {
 file,err:=os.Open(ref)
 if err!=nil{return platform.NewError(platform.CodeDependencyUnavailable,false,"服务身份材料不可读")}
 defer file.Close()
 raw,err:=io.ReadAll(io.LimitReader(file,(1<<20)+1))
 if err!=nil || len(raw)>1<<20{return platform.NewError(platform.CodeIntegrityError,false,"服务身份材料超过预算")}
 decoder:=json.NewDecoder(bytes.NewReader(raw));decoder.DisallowUnknownFields()
 if err:=decoder.Decode(target);err!=nil{return platform.NewError(platform.CodeIntegrityError,false,"服务身份材料结构不合法")}
 if decoder.Decode(new(any))!=io.EOF{return platform.NewError(platform.CodeIntegrityError,false,"服务身份材料含额外数据")}
 return nil
}
