// Package bundled保存models.dev公开目录数据；MIT，Copyright (c) 2025 models.dev。
// 来源https://models.dev/api.json；内容版本见source.json；原许可见同目录LICENSE。
package bundled

import (
 _ "embed"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
)

//go:embed models.dev.json
var catalogJSON string
//go:embed source.json
var sourceJSON string

const contentHash="80a19a3c78de1a7b0eb0a72e13e44df21180f0681d6cdf1dbd4c4c345fae3fb6"
type Source struct{Source string `json:"source"`;SourceVersion string `json:"sourceVersion"`;SHA256 string `json:"sha256"`;ByteSize int `json:"byteSize"`;RetrievedAt string `json:"retrievedAt"`;License string `json:"license"`}

// ReadSnapshot返回真实固定数据副本；版本取内容hash，不把仓库分支当API数据版本。
func ReadSnapshot()([]byte,Source,error){
 raw:=[]byte(catalogJSON);var source Source
 if json.Unmarshal([]byte(sourceJSON),&source)!=nil||source.Source!="https://models.dev/api.json"||source.SHA256!=contentHash||source.SourceVersion!="sha256:"+contentHash||source.ByteSize!=len(raw)||source.License!="MIT"||platform.HashBytes(raw)!=contentHash{return nil,Source{},platform.NewError(platform.CodeIntegrityError,false,"编译模型目录数据与真实来源摘要不一致")}
 if _,err:=time.Parse(time.RFC3339Nano,source.RetrievedAt);err!=nil{return nil,Source{},platform.NewError(platform.CodeIntegrityError,false,"编译模型目录取得时间损坏")}
 return raw,source,nil
}
