package translation

import (
 "context"
 "encoding/json"
 "html"
 "strconv"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
)

type protectedField struct{Values []string;Fixed []bool}
type slotField struct{Key string;Indices []int}
type ProtectedProjection struct{Tree any;Fields map[string]protectedField;Units map[string]bool;Whole map[string]bool;Rows map[string]bool}

// StableSlotKey 只使用最深真实节点身份及该节点内路径；祖先数组位置不参与对应。
func StableSlotKey(tree any,path []string)(string,error){
 cursor:=tree;anchor:=[]string{};relative:=[]string{}
 for index:=0;index<=len(path);index++{
  if node,ok:=cursor.(map[string]any);ok{if attrs,ok:=node["attrs"].(map[string]any);ok{
   identity:=[]string{};kind,_:=node["type"].(string)
   for _,name:=range []string{"blockId","rowId","cellId","itemId","columnId"}{if id,ok:=attrs[name].(string);ok&&id!=""{identity=append(identity,name,id)}}
   if len(identity)>0{anchor=append([]string{kind},identity...);relative=[]string{}}
  }}
  if index==len(path){break};segment:=path[index];relative=append(relative,segment)
  switch node:=cursor.(type){case map[string]any:cursor=node[segment];case []any:position,err:=strconv.Atoi(segment);if err!=nil||position<0||position>=len(node){return "",platform.NewError(platform.CodeIntegrityError,false,"保护模板路径不属于原树")};cursor=node[position];default:return "",platform.NewError(platform.CodeIntegrityError,false,"保护模板路径无法定位")}
 }
 if len(anchor)==0{return "",platform.NewError(platform.CodeIntegrityError,false,"保护文本没有真实稳定节点身份")}
 raw,_:=platform.CanonicalJSON(struct{Anchor,Path []string}{anchor,relative});return string(raw),nil
}

// CollectSlotFields 把同一text/alt的URL分段放在一起，不能用段号跨字段猜译文。
func CollectSlotFields(tree any,slots []InlineSlot)([]slotField,error){
 result:=[]slotField{};byPath:=map[string]int{};byStable:=map[string]string{}
 for index,slot:=range slots{if len(slot.Path)==0{continue};raw,_:=json.Marshal(slot.Path);path:=string(raw)
  if position,found:=byPath[path];found{result[position].Indices=append(result[position].Indices,index);continue}
  key,err:=StableSlotKey(tree,slot.Path);if err!=nil{return nil,err};if prior,found:=byStable[key];found&&prior!=path{return nil,platform.NewError(platform.CodeIntegrityError,false,"真实节点身份不能唯一定位保护字段")}
  byStable[key]=path;byPath[path]=len(result);result=append(result,slotField{Key:key,Indices:[]int{index}})
 };return result,nil
}

// BuildProtectedProjection 当前对象优先于上一完整代；只读取真实人工/保护单元。
func BuildProtectedProjection(ctx context.Context,op OperationContext,current json.RawMessage,objects []TranslationObject)(ProtectedProjection,error){
 result:=ProtectedProjection{Fields:map[string]protectedField{},Units:map[string]bool{},Whole:map[string]bool{},Rows:map[string]bool{}}
 decoder:=json.NewDecoder(strings.NewReader(string(current)));decoder.UseNumber();if decoder.Decode(&result.Tree)!=nil{return ProtectedProjection{},platform.NewError(platform.CodeIntegrityError,false,"当前保护投影缺少完整源树")}
 for _,object:=range objects{if object.Translation.TranslationId==""{continue};protected:=map[string]gen.TranslationUnit{}
  for _,unit:=range object.Units{if !unit.Protected&&unit.Origin!="human"{continue};protected[unit.UnitId]=unit;result.Units[unit.UnitId]=true;parent:=unit.UnitId;if unit.ParentBlockId!=nil{parent=*unit.ParentBlockId};if parent==unit.UnitId{result.Whole[parent]=true}else{result.Rows[parent]=true}}
  if len(protected)==0{continue}
  split,err:=SplitTranslationUnits(ctx,op,SplitTranslationUnitsInput{Document:object.Translation.Document,Source:string(object.SourceTree),TargetLanguage:object.Translation.TargetLanguage});if err!=nil{return ProtectedProjection{},err}
  var tree any;decoder=json.NewDecoder(strings.NewReader(string(object.SourceTree)));decoder.UseNumber();if decoder.Decode(&tree)!=nil{return ProtectedProjection{},platform.NewError(platform.CodeIntegrityError,false,"人工译文原树损坏")}
  for _,template:=range split.Templates{unit,found:=protected[template.UnitId];if !found{continue};if unit.Status=="fallback"{return ProtectedProjection{},platform.NewError(platform.CodeDependencyUnavailable,false,"人工或受保护旧译缺少可恢复结果")}
   values,err:=ParseSlotValues(template.Slots,unit.TargetText);if err!=nil{return ProtectedProjection{},err};fields,err:=CollectSlotFields(tree,template.Slots);if err!=nil{return ProtectedProjection{},err}
   for _,field:=range fields{if _,newer:=result.Fields[field.Key];newer{continue};value:=protectedField{Values:[]string{},Fixed:[]bool{}}
    for _,index:=range field.Indices{value.Values=append(value.Values,values[index]);value.Fixed=append(value.Fixed,template.Slots[index].Fixed)};result.Fields[field.Key]=value
   }
  }
 };return result,nil
}

// ProjectProtectedUnit 只复用证明同归属且段形状一致的译值，固定值永远取当前源。
func ProjectProtectedUnit(projection ProtectedProjection,template UnitTemplate,candidate gen.TranslationUnit)(gen.TranslationUnit,bool,bool,error){
 fields,err:=CollectSlotFields(projection.Tree,template.Slots);if err!=nil{return gen.TranslationUnit{},false,false,err}
 parent:=template.ParentBlockId;inherited:=projection.Whole[parent]||(template.UnitId==parent&&projection.Rows[parent])
 values:=make([]string,len(template.Slots));for index,slot:=range template.Slots{values[index]=slot.Value}
 reused:=false;partial:=false
 for _,field:=range fields{old,found:=projection.Fields[field.Key];translatable:=false;for _,index:=range field.Indices{if !template.Slots[index].Fixed{translatable=true}}
  if !translatable{continue};if !found{partial=true;continue}
  if len(old.Values)!=len(field.Indices){return gen.TranslationUnit{},false,false,platform.NewError(platform.CodeDependencyUnavailable,false,"人工字段的URL或代码分段形状已改变")}
  for position,index:=range field.Indices{slot:=template.Slots[index];if old.Fixed[position]!=slot.Fixed{return gen.TranslationUnit{},false,false,platform.NewError(platform.CodeDependencyUnavailable,false,"人工字段的固定值归属无法证明")};if !slot.Fixed{values[index]=old.Values[position];reused=true}}
 }
 if !reused&&!inherited{
  if projection.Units[candidate.UnitId]{return gen.TranslationUnit{},false,false,platform.NewError(platform.CodeDependencyUnavailable,false,"人工单元仍在当前源但原文本归属无法证明")};return candidate,false,false,nil
 }
 var target strings.Builder
 for index,slot:=range template.Slots{number:=strconv.Itoa(slot.Index);if slot.Fixed{target.WriteString("<v"+number+"/>")}else{target.WriteString("<i"+number+">"+html.EscapeString(values[index])+"</i"+number+">")}}
 candidate.TargetText=target.String();candidate.TargetHash=platform.HashBytes([]byte(candidate.TargetText));candidate.Protected=true;candidate.Status="review";if reused{candidate.Origin="human"}
 return candidate,true,partial,nil
}
