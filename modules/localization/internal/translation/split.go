package translation

import (
 "context"
 "encoding/json"
 "errors"
 "html"
 "regexp"
 "strconv"
 "strings"
 "unicode/utf8"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
)

type InlineSlot struct{Index int `json:"index"`;Path []string `json:"path"`;Value string `json:"value"`;Fixed bool `json:"fixed"`}
type UnitTemplate struct{UnitId string `json:"unitId"`;ParentBlockId string `json:"parentBlockId"`;Slots []InlineSlot `json:"slots"`}
type SplitTranslationUnitsInput struct{Document gen.PublishedRevisionRef `json:"document"`;Source string `json:"source"`;TargetLanguage string `json:"targetLanguage"`}
type SplitTranslationUnitsResult struct{Units []gen.TranslationUnit `json:"units"`;Templates []UnitTemplate `json:"templates"`;SourceTree json.RawMessage `json:"sourceTree"`;ModelText string `json:"modelText"`;FallbackBlockIds []string `json:"fallbackBlockIds"`}
var errUnitFragmentTooLarge=errors.New("单元编号文本超过允许大小")
var protectedURLPattern=regexp.MustCompile(`(?i)(https?://|mailto:)[^\s<>]+`)

// SplitTranslationUnits 保留完整树和真实块/行身份；外部值只在本地模板中保存。
func SplitTranslationUnits(ctx context.Context,op OperationContext,input SplitTranslationUnitsInput)(SplitTranslationUnitsResult,error){
 if !utf8.ValidString(input.Source){return SplitTranslationUnitsResult{},platform.NewError(platform.CodeIntegrityError,false,"翻译源不是UTF8")}
 if len([]byte(input.Source))>16<<20{return SplitTranslationUnitsResult{},platform.NewError(platform.CodeBudgetExceeded,false,"翻译源超过content的16MiB正文上限")}
 var tree map[string]any;decoder:=json.NewDecoder(strings.NewReader(input.Source));decoder.UseNumber();if decoder.Decode(&tree)!=nil{return SplitTranslationUnitsResult{},platform.NewError(platform.CodeIntegrityError,false,"发布正文不是完整JSON树")}
 version,versionOK:=tree["schemaVersion"].(json.Number);if !versionOK||string(version)!=strconv.Itoa(input.Document.SchemaVersion){return SplitTranslationUnitsResult{},platform.NewError(platform.CodeIntegrityError,false,"源正文schemaVersion与发布引用不匹配")}
 canonical,err:=platform.CanonicalJSON(tree);if err!=nil||platform.HashBytes(canonical)!=input.Document.ContentHash{return SplitTranslationUnitsResult{},platform.NewError(platform.CodeIntegrityError,false,"源树摘要与发布引用不一致")}
 target,err:=language.NormalizeLanguageTag(ctx,op,language.NormalizeLanguageTagInput{Tag:input.TargetLanguage});if err!=nil{return SplitTranslationUnitsResult{},err}
 result:=SplitTranslationUnitsResult{Units:[]gen.TranslationUnit{},Templates:[]UnitTemplate{},SourceTree:canonical,FallbackBlockIds:[]string{}};ids:=map[string]bool{};fallback:=map[string]bool{}
 addFallback:=func(id string){if !fallback[id]{fallback[id]=true;result.FallbackBlockIds=append(result.FallbackBlockIds,id)}}
 addUnit:=func(node map[string]any,path []string,id,parent string)(bool,error){
  template:=UnitTemplate{UnitId:id,ParentBlockId:parent,Slots:[]InlineSlot{}};var model strings.Builder
  if err:=CollectSlots(node,path,&template.Slots,&model);err!=nil{if errors.Is(err,errUnitFragmentTooLarge){addFallback(parent);return false,nil};return false,err}
  text:=model.String();translatable:=false;for _,slot:=range template.Slots{if !slot.Fixed&&strings.TrimSpace(slot.Value)!=""{translatable=true;break}};if text==""||!translatable{return true,nil}
  if utf8.RuneCountInString(text)>65536{addFallback(parent);return false,nil}
  if ids[id]{return false,platform.NewError(platform.CodeIntegrityError,false,"翻译源块或表格行ID重复")};ids[id]=true
  raw,_:=platform.CanonicalJSON(WithoutSourceIDs(node));sourceHash:=platform.HashBytes(raw)
  unit:=gen.TranslationUnit{UnitId:id,Document:input.Document,TargetLanguage:target.Tag,SourceHash:sourceHash,SourceText:&text,TargetText:text,TargetHash:platform.HashBytes([]byte(text)),Origin:"machine",Protected:false,Version:0,HumanGeneration:0,Status:"fallback",ParentBlockId:&parent}
  result.Units=append(result.Units,unit);result.Templates=append(result.Templates,template);return true,nil
 }
 var walk func(map[string]any,[]string)error
 walk=func(node map[string]any,path []string)error{
  kind,_:=node["type"].(string);attrs,_:=node["attrs"].(map[string]any);blockID,_:=attrs["blockId"].(string)
  switch kind{
  case "codeBlock","mathBlock","horizontalRule","attachment","drawing","video":return nil
  case "paragraph","heading","image":_,err:=addUnit(node,path,blockID,blockID);return err
  case "table":
   // 只对超大表格按生产方真实稳定rowId分组；其他节点从不编造子ID。
   probe:=[]InlineSlot{};var text strings.Builder;probeErr:=CollectSlots(node,path,&probe,&text)
   if probeErr==nil&&utf8.RuneCountInString(text.String())<=65536{_,err:=addUnit(node,path,blockID,blockID);return err}
   rows,_:=node["content"].([]any)
   for i,value:=range rows{row:=value.(map[string]any);rowAttrs,_:=row["attrs"].(map[string]any);rowID,_:=rowAttrs["rowId"].(string);if rowID==""{return platform.NewError(platform.CodeIntegrityError,false,"超大表格缺少稳定行标识")};if _,err:=addUnit(row,AppendPath(path,"content",strconv.Itoa(i)),rowID,blockID);err!=nil{return err}};return nil
  }
  children,_:=node["content"].([]any);for index,value:=range children{if err:=walk(value.(map[string]any),AppendPath(path,"content",strconv.Itoa(index)));err!=nil{return err}};return nil
 }
 if err=walk(tree,[]string{});err!=nil{return SplitTranslationUnitsResult{},err}
 var contextText strings.Builder;for _,u:=range result.Units{contextText.WriteString(u.UnitId);contextText.WriteString(":\n");contextText.WriteString(*u.SourceText);contextText.WriteByte('\n')};result.ModelText=contextText.String()
 if len(canonical)+len([]byte(result.ModelText))*3>64<<20{return SplitTranslationUnitsResult{},platform.NewError(platform.CodeBudgetExceeded,false,"翻译源与候选聚合对象超过64MiB预算")}
 return result,nil
}
func AppendPath(path []string,parts ...string)[]string{result:=append([]string{},path...);return append(result,parts...)}
// CollectSlots 对每个文本节点编号；代码、公式、URL与换行只留下值标签。
func CollectSlots(node map[string]any,path []string,slots *[]InlineSlot,text *strings.Builder)error{
 kind,_:=node["type"].(string)
 if kind=="codeBlock"||kind=="mathBlock"||kind=="video"||kind=="attachment"||kind=="drawing"{return nil}
 if kind=="inlineMath"||kind=="hardBreak"{return AppendSlot(slots,text,nil,"",true)}
 if kind=="text"{
  value,_:=node["text"].(string);fixed:=false;marks,_:=node["marks"].([]any);for _,mark:=range marks{if mark.(map[string]any)["type"]=="code"{fixed=true}}
  if fixed{return AppendSlot(slots,text,AppendPath(path,"text"),value,true)}
  return AppendTextSlots(slots,text,AppendPath(path,"text"),value)
 }
 if kind=="image"{
  attrs,_:=node["attrs"].(map[string]any);if alt,ok:=attrs["alt"].(string);ok&&alt!=""{if err:=AppendTextSlots(slots,text,AppendPath(path,"attrs","alt"),alt);err!=nil{return err}}
  caption,_:=attrs["caption"].([]any);for i,n:=range caption{if err:=CollectSlots(n.(map[string]any),AppendPath(path,"attrs","caption",strconv.Itoa(i)),slots,text);err!=nil{return err}};return nil
 }
 children,_:=node["content"].([]any);for i,n:=range children{if err:=CollectSlots(n.(map[string]any),AppendPath(path,"content",strconv.Itoa(i)),slots,text);err!=nil{return err}};return nil
}
func AppendTextSlots(slots *[]InlineSlot,text *strings.Builder,path []string,value string)error{
 matches:=protectedURLPattern.FindAllStringIndex(value,-1);position:=0
 for _,at:=range matches{if at[0]>position{if err:=AppendSlot(slots,text,path,value[position:at[0]],false);err!=nil{return err}};if err:=AppendSlot(slots,text,path,value[at[0]:at[1]],true);err!=nil{return err};position=at[1]};if position<len(value){return AppendSlot(slots,text,path,value[position:],false)};return nil
}
func AppendSlot(slots *[]InlineSlot,text *strings.Builder,path []string,value string,fixed bool)error{
 index:=len(*slots)+1;number:=strconv.Itoa(index);encoded:="<v"+number+"/>"
 if !fixed{encoded="<i"+number+">"+html.EscapeString(value)+"</i"+number+">"}
 // 最多四字节一个Unicode字符；先限制收集内存，再做精确65536字符判断。
 if text.Len()+len(encoded)>65536*4{return errUnitFragmentTooLarge}
 *slots=append(*slots,InlineSlot{Index:index,Path:append([]string{},path...),Value:value,Fixed:fixed});text.WriteString(encoded);return nil
}
// WithoutSourceIDs 让相同内容跨新块身份命中TM，URL/代码/结构仍参与源hash。
func WithoutSourceIDs(value any)any{
 switch v:=value.(type){
 case map[string]any:
  result:=map[string]any{}
  for key,item:=range v{
   if key=="attrs"{if attrs,ok:=item.(map[string]any);ok{clean:=map[string]any{};for attr,datum:=range attrs{if attr=="blockId"||attr=="rowId"||attr=="cellId"||attr=="itemId"||attr=="columnId"{continue};clean[attr]=WithoutSourceIDs(datum)};result[key]=clean;continue}}
   result[key]=WithoutSourceIDs(item)
  };return result
 case []any:result:=make([]any,0,len(v));for _,item:=range v{result=append(result,WithoutSourceIDs(item))};return result
 default:return value
 }
}
