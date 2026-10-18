package protection

import (
 "context"
 "net/url"
 "strings"
 "nice-wiki/libs/platform"
)

type ValidateRemoteInput struct{URL string `json:"url"`;Branch string `json:"branch"`}
type ValidateRemoteResult struct{Valid bool `json:"valid"`}

// ValidateRemote 只解析配置，不执行网络访问或Git命令；allowlist只能由受信部署装配。
func ValidateRemote(ctx context.Context,op OperationContext,in ValidateRemoteInput)(ValidateRemoteResult,error){
 store,err:=Catalog(op);if err!=nil{return ValidateRemoteResult{},err};target,err:=url.Parse(in.URL);if err!=nil{return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程地址格式不合法")}
 if (target.Scheme!="https"&&target.Scheme!="ssh")||target.Opaque!=""||target.User!=nil||target.RawQuery!=""||target.ForceQuery||target.Fragment!=""||target.RawFragment!=""||target.RawPath!=""||strings.ContainsAny(in.URL,"%\\\r\n\t "){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程仅允许无凭证与参数的HTTPS或SSH地址")}
 host:=target.Hostname();if host==""||host!=strings.ToLower(host)||strings.HasPrefix(host,".")||strings.HasSuffix(host,".")||strings.Contains(host,".."){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程主机格式不合法")};for _,ch:=range host{if !(ch>='a'&&ch<='z'||ch>='0'&&ch<='9'||ch=='.'||ch=='-'){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程主机必须使用部署登记的ASCII名称")}}
 port:=target.Port();if port!=""&&!(target.Scheme=="https"&&port=="443"||target.Scheme=="ssh"&&port=="22"){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程只允许协议标准端口")};allowed:=false;for _,configured:=range store.AllowedGitHosts{if host==configured{allowed=true;break}};if !allowed{return ValidateRemoteResult{},platform.NewError(platform.CodeForbidden,false,"远程主机不在部署白名单")}
 if target.Path==""||target.Path=="/"||!strings.HasPrefix(target.Path,"/")||strings.HasSuffix(target.Path,"/"){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程仓库路径不合法")};for _,segment:=range strings.Split(strings.TrimPrefix(target.Path,"/"),"/"){if segment==""||segment=="."||segment==".."||strings.HasPrefix(segment,"-"){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程仓库路径含不安全段")};for _,ch:=range segment{if !(ch>='a'&&ch<='z'||ch>='A'&&ch<='Z'||ch>='0'&&ch<='9'||ch=='.'||ch=='_'||ch=='-'){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"远程仓库路径含不支持字符")}}}
 branch:=in.Branch;if !strings.HasPrefix(branch,"refs/heads/"){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"分支必须是refs/heads下完整引用")};name:=strings.TrimPrefix(branch,"refs/heads/");if name==""||strings.Contains(name,"..")||strings.Contains(name,"@{")||strings.HasSuffix(name,"."){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"分支引用不合法")};for _,segment:=range strings.Split(name,"/"){if segment==""||strings.HasPrefix(segment,".")||strings.HasPrefix(segment,"-")||strings.HasSuffix(segment,".lock"){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"分支引用段不合法")};for _,ch:=range segment{if !(ch>='a'&&ch<='z'||ch>='A'&&ch<='Z'||ch>='0'&&ch<='9'||ch=='.'||ch=='_'||ch=='-'){return ValidateRemoteResult{},platform.NewError(platform.CodeValidation,false,"分支引用含不支持字符")}}};return ValidateRemoteResult{Valid:true},nil
}
