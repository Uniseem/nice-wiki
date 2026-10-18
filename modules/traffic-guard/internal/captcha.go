package trafficguard

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "os"
 "strings"
 "time"
 "nice-wiki/libs/platform"
)

// TurnstileConfig 仅可信装配提供；secret仅外部引用，不随业务快照导出。
type TurnstileConfig struct { SiteKey string;SecretRef string;ExpectedHostname string;Client *http.Client }
type VerifyExternalChallengeInput struct { ResponseToken string;ExpectedAction string;SubjectKey string;ChallengeID string }
type VerifyExternalChallengeResult struct { Valid bool }

// VerifyExternalChallenge 根据供应方公开协议编写，不包含第三方实现代码。
// 协议来源：https://developers.cloudflare.com/turnstile/get-started/server-side-validation/
func VerifyExternalChallenge(ctx context.Context,op OperationContext,in VerifyExternalChallengeInput)(VerifyExternalChallengeResult,error) {
 runtime,ok:=ctx.Value(guardRuntimeKey{}).(GuardRuntime)
 if !ok||runtime.Turnstile.SiteKey==""||runtime.Turnstile.SecretRef==""||runtime.Turnstile.ExpectedHostname=="" { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战配置未装配") }
 if len(in.ResponseToken)==0||len(in.ResponseToken)>2048 { return VerifyExternalChallengeResult{Valid:false},nil }
 secret,err:=os.ReadFile(runtime.Turnstile.SecretRef)
 if err!=nil||len(strings.TrimSpace(string(secret)))==0 { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战密钥引用不可用") }
 work,cancel:=context.WithTimeout(ctx,2500*time.Millisecond);defer cancel()
 // 固定HTTPS供应方地址，不接请求或配置中的任意回调URL；不传原IP。
 form:=url.Values{"secret":{strings.TrimSpace(string(secret))},"response":{in.ResponseToken}}
 request,err:=http.NewRequestWithContext(work,http.MethodPost,"https://challenges.cloudflare.com/turnstile/v0/siteverify",strings.NewReader(form.Encode()))
 if err!=nil { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeInternal,false,"外部挑战请求构造失败") }
 request.Header.Set("Content-Type","application/x-www-form-urlencoded")
 client:=http.Client{}
 if runtime.Turnstile.Client!=nil { client=*runtime.Turnstile.Client }
 client.Timeout=2500*time.Millisecond
 client.CheckRedirect=func(*http.Request,[]*http.Request)error { return http.ErrUseLastResponse }
 response,err:=client.Do(request)
 if err!=nil { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战验证暂不可用") }
 defer response.Body.Close()
 if response.StatusCode!=http.StatusOK { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战验证暂不可用") }
 raw,err:=io.ReadAll(io.LimitReader(response.Body,65537))
 if err!=nil||len(raw)>65536 { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战响应不可用") }
 var result struct { Success bool `json:"success"`;Hostname string `json:"hostname"`;Action string `json:"action"`;CData string `json:"cdata"`;ChallengeTS string `json:"challenge_ts"` }
 if json.Unmarshal(raw,&result)!=nil { return VerifyExternalChallengeResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"外部挑战响应不可用") }
 if !result.Success { return VerifyExternalChallengeResult{Valid:false},nil }
 issued,err:=time.Parse(time.RFC3339,result.ChallengeTS)
 now:=time.Now().UTC()
 valid:=err==nil&&!issued.After(now)&&now.Sub(issued)<=5*time.Minute&&result.Hostname==runtime.Turnstile.ExpectedHostname&&result.Action==in.ExpectedAction&&result.CData==in.ChallengeID
 return VerifyExternalChallengeResult{Valid:valid},nil
}
