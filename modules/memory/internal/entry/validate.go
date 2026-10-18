// Package entry 实现人工记忆条目的权威 Markdown 修订。
package entry

import (
 "regexp"
 "strings"
 "unicode/utf8"
 "golang.org/x/text/language"
 "golang.org/x/text/unicode/norm"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type OperationContext=platform.OperationContext
type ValidateEntryInput struct {Entry gen.MemoryEntry `json:"entry"`}
type ValidateEntryResult struct {Valid bool `json:"valid"`;NormalizedBody string `json:"normalizedBody"`;BodyHash string `json:"bodyHash"`}
var credentialPattern=regexp.MustCompile(`(?i)(-----BEGIN (?:RSA |EC |OPENSSH |ENCRYPTED )?PRIVATE KEY-----|(?:api[_-]?key|access[_-]?token|client[_-]?secret|password|密码|密钥)\s*[:=]\s*\S+|\bsk-[A-Za-z0-9_-]{16,}|\bAKIA[A-Z0-9]{16}\b|\bgh[pousr]_[A-Za-z0-9]{20,}|内部客户信息|客户机密|confidential-client)`)

func ValidateEntry(e gen.MemoryEntry)(gen.MemoryEntry,error){
 if !platform.ValidIdentifier(e.EntryId)||e.EntryId=="."||e.EntryId==".."{return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆条目标识不是安全路径段")}
 e.Body=norm.NFC.String(memoryruntime.NormalizeLineEndings(e.Body));e.Title=norm.NFC.String(e.Title);e.Description=norm.NFC.String(memoryruntime.NormalizeLineEndings(e.Description))
 if !utf8.ValidString(e.Body)||e.Body==""||e.MaximumBytes<1||len([]byte(e.Body))>e.MaximumBytes||len([]byte(e.Body))>65536||utf8.RuneCountInString(e.Title)>256||strings.TrimSpace(e.Title)==""||e.Description==""{return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆正文、标题、说明或单条预算不合法")}
 if _,err:=language.Parse(e.Language);err!=nil{return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆语言标签不合法")}
 tags:=[]string{};seen:=map[string]bool{};for _,tag:=range e.Tags{tag=norm.NFC.String(strings.TrimSpace(tag));if tag==""{return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆标签为空")};if !seen[tag]{seen[tag]=true;tags=append(tags,tag)}}
 if len(tags)>16{return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆标签最多16个")};e.Tags=tags
 if credentialPattern.MatchString(e.Title+"\n"+e.Description+"\n"+e.Body+"\n"+strings.Join(e.Tags,"\n")){return gen.MemoryEntry{},platform.NewError(platform.CodeValidation,false,"记忆内容含禁止公开的凭证或敏感标记")}
 e.BodyHash=platform.HashBytes([]byte(e.Body));return e,nil
}

// ContainsPrivateMaterial 只实施登记的确定性凭证与敏感标记规则，不推断任意业务机密。
func ContainsPrivateMaterial(text string)bool{return credentialPattern.MatchString(text)}
