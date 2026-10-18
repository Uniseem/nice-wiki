package provider

import (
 "context"
 "encoding/json"
 "math"
 "sort"
 "nice-wiki/libs/platform"
)
type ParseRerankResponseInput struct{CandidateIDs []string `json:"candidateIds"`;ResponseBody string `json:"responseBody"`}
type ParseRerankResponseResult struct{OrderedIDs []string `json:"orderedIds"`}
func ParseRerankResponse(ctx context.Context,op OperationContext,input ParseRerankResponseInput)(ParseRerankResponseResult,error){
 type ranked struct{Index *int `json:"index"`;Score *float64 `json:"relevance_score"`};var response struct{Results []ranked `json:"results"`}
 if json.Unmarshal([]byte(input.ResponseBody),&response)!=nil||response.Results==nil{return ParseRerankResponseResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"重排响应缺少results")}
 seen:=make(map[int]bool);for _,item:=range response.Results{if item.Index==nil||item.Score==nil||*item.Index<0||*item.Index>=len(input.CandidateIDs)||seen[*item.Index]||math.IsNaN(*item.Score)||math.IsInf(*item.Score,0){return ParseRerankResponseResult{},platform.NewError(platform.CodeIntegrityError,false,"重排响应index重复、越界或分数非有限")};seen[*item.Index]=true}
 sort.SliceStable(response.Results,func(i,j int)bool{return *response.Results[i].Score>*response.Results[j].Score});result:=ParseRerankResponseResult{OrderedIDs:make([]string,0,len(input.CandidateIDs))};for _,item:=range response.Results{result.OrderedIDs=append(result.OrderedIDs,input.CandidateIDs[*item.Index])};for index,id:=range input.CandidateIDs{if !seen[index]{result.OrderedIDs=append(result.OrderedIDs,id)}};return result,nil
}
