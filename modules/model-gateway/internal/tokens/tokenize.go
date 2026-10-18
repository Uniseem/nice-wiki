package tokens

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "net/http"

 "github.com/tiktoken-go/tokenizer"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/problem"
 "nice-wiki/modules/model-gateway/internal/catalog"
)

type TokenizeTextInput struct {
 ModelID string `json:"modelId"`
 Text string `json:"text"`
}

type TokenizeTextResult struct {
 EmbeddingSpace catalog.EmbeddingSpace `json:"embeddingSpace"`
 TokenizerVersion string `json:"tokenizerVersion"`
 TokenCount int `json:"tokenCount"`
 ByteTokenEnds []int `json:"byteTokenEnds"`
 TextHash platform.Hash `json:"textHash"`
}

// TokenizeText 使用模型不可变声明中的离线BPE；不创建调用或费用记录。
func TokenizeText(ctx context.Context, op OperationContext, input TokenizeTextInput) (TokenizeTextResult, error) {
 if op.CallerService != "knowledge" || op.Actor.Kind != "service" || op.Actor.SubjectID != "knowledge" {
  return TokenizeTextResult{}, platform.NewError(platform.CodeForbidden, false, "分词只允许知识纯服务")
 }
 if len(input.Text) > 4<<20 { return TokenizeTextResult{}, platform.NewError(platform.CodeValidation, false, "分词正文超过4MiB字节预算") }
 model, err := catalog.ReadTokenizerModel(ctx, op, input.ModelID)
 if err != nil { return TokenizeTextResult{}, err }
 space, err := catalog.ModelEmbeddingSpace(model)
 if err != nil { return TokenizeTextResult{}, err }
 var encoding tokenizer.Encoding
 switch model.PreprocessorVersion {
 case "none;tiktoken-go@v0.7.0;cl100k_base": encoding=tokenizer.Cl100kBase
 case "none;tiktoken-go@v0.7.0;o200k_base": encoding=tokenizer.O200kBase
 default: return TokenizeTextResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型没有受支持的固定分词声明")
 }
 codec, err := tokenizer.Get(encoding)
 if err != nil { return TokenizeTextResult{}, platform.NewError(platform.CodeInternal, false, "固定分词器无法装配") }
 ids, pieces, err := codec.Encode(input.Text)
 if err != nil { return TokenizeTextResult{}, platform.NewError(platform.CodeIntegrityError, false, "文本无法按固定分词器编码") }
 // 真实BPE可在一个Unicode字符中间结束；调用方据原始token序号选择合法字符边界。
 ends := make([]int, 0, len(pieces))
 offset := 0
 for _, piece := range pieces {
  end:=offset+len(piece)
  if len(piece)==0 || end>len(input.Text) || input.Text[offset:end]!=piece { return TokenizeTextResult{}, platform.NewError(platform.CodeIntegrityError, false, "分词字节片与输入不一致") }
  ends=append(ends,end)
  offset=end
 }
 if offset!=len(input.Text) || len(ids)!=len(ends) { return TokenizeTextResult{}, platform.NewError(platform.CodeIntegrityError, false, "分词结果没有完整覆盖输入") }
 sum:=sha256.Sum256([]byte(input.Text))
 return TokenizeTextResult{EmbeddingSpace:space,TokenizerVersion:model.PreprocessorVersion,TokenCount:len(ids),ByteTokenEnds:ends,TextHash:hex.EncodeToString(sum[:])},nil
}

// NewTokenizerHandlers 由公共严格校验入口绑定免费分词操作。
func NewTokenizerHandlers() map[string]http.Handler {
 return map[string]http.Handler{"tokenizeText":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Cache-Control","no-store")
  op,_:=platform.OperationContextFrom(r.Context())
  var input TokenizeTextInput
  if json.NewDecoder(r.Body).Decode(&input)!=nil { _=problem.WriteProblem(w,problem.FromError(platform.NewError(platform.CodeValidation,false,"分词输入不可读"),op.RequestID));return }
  result,err:=TokenizeText(r.Context(),op,input)
  if err!=nil { _=problem.WriteProblem(w,problem.FromError(err,op.RequestID));return }
  w.Header().Set("Content-Type","application/json")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _=json.NewEncoder(w).Encode(result)
 })}
}
