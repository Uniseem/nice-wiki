package translation

import (
	"context"
	"encoding/json"
	"html"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"strconv"
	"strings"
	"unicode/utf8"
)

type protectedField struct {
	Values []string
	Fixed  []bool
	Human  bool
}
type slotField struct {
	Key     string
	Anchor  string
	Shape   string
	Indices []int
}
type ProtectedProjection struct {
	Tree   any
	Fields map[string]protectedField
	Shapes map[string]string
	Units  map[string]bool
	Whole  map[string]bool
	Rows   map[string]bool
}

var sourceNodeIDAttributes = map[string]string{"paragraph": "blockId", "heading": "blockId", "blockquote": "blockId", "horizontalRule": "blockId", "bulletList": "blockId", "orderedList": "blockId", "taskList": "blockId", "table": "blockId", "codeBlock": "blockId", "mathBlock": "blockId", "callout": "blockId", "image": "blockId", "attachment": "blockId", "drawing": "blockId", "video": "blockId", "columns": "blockId", "listItem": "itemId", "taskItem": "itemId", "tableRow": "rowId", "tableCell": "cellId", "tableHeader": "cellId", "column": "columnId"}

// ProtectedAnchorShape 保留无ID子树节点类型/字段/数组形状；真实稳定子节点独立定锚。
func ProtectedAnchorShape(root any) string {
	var shape func(any, bool) any
	shape = func(value any, anchored bool) any {
		switch node := value.(type) {
		case map[string]any:
			kind, _ := node["type"].(string)
			if !anchored && sourceNodeIDAttributes[kind] != "" {
				return nil
			}
			result := map[string]any{}
			for key, item := range node {
				if key == "type" {
					result[key] = item
				} else {
					result[key] = shape(item, false)
				}
			}
			return result
		case []any:
			result := []any{}
			for _, item := range node {
				if child := shape(item, false); child != nil {
					result = append(result, child)
				}
			}
			return result
		case string:
			return "string"
		case json.Number:
			return "number"
		case bool:
			return "boolean"
		default:
			return "null"
		}
	}
	raw, _ := platform.CanonicalJSON(shape(root, true))
	return platform.HashBytes(raw)
}

// StableSlotKey 只使用最深真实节点身份及该节点内路径；祖先数组位置不参与对应。
func StableSlotKey(tree any, path []string) (string, any, error) {
	cursor := tree
	anchor := []string{}
	relative := []string{}
	var anchored any
	for index := 0; index <= len(path); index++ {
		if node, ok := cursor.(map[string]any); ok {
			if attrs, ok := node["attrs"].(map[string]any); ok {
				identity := []string{}
				kind, _ := node["type"].(string)
				name := sourceNodeIDAttributes[kind]
				if id, ok := attrs[name].(string); name != "" && ok && id != "" {
					identity = append(identity, name, id)
				}
				if len(identity) > 0 {
					anchor = append([]string{kind}, identity...)
					relative = []string{}
					anchored = node
				}
			}
		}
		if index == len(path) {
			break
		}
		segment := path[index]
		relative = append(relative, segment)
		switch node := cursor.(type) {
		case map[string]any:
			cursor = node[segment]
		case []any:
			position, err := strconv.Atoi(segment)
			if err != nil || position < 0 || position >= len(node) {
				return "", nil, platform.NewError(platform.CodeIntegrityError, false, "保护模板路径不属于原树")
			}
			cursor = node[position]
		default:
			return "", nil, platform.NewError(platform.CodeIntegrityError, false, "保护模板路径无法定位")
		}
	}
	if len(anchor) == 0 {
		return "", nil, platform.NewError(platform.CodeIntegrityError, false, "保护文本没有真实稳定节点身份")
	}
	raw, _ := platform.CanonicalJSON(struct{ Anchor, Path []string }{anchor, relative})
	return string(raw), anchored, nil
}

// CollectSlotFields 把同一text/alt的URL分段放在一起，不能用段号跨字段猜译文。
func CollectSlotFields(tree any, slots []InlineSlot) ([]slotField, error) {
	result := []slotField{}
	byPath := map[string]int{}
	byStable := map[string]string{}
	shapes := map[string]string{}
	for index, slot := range slots {
		if len(slot.Path) == 0 {
			continue
		}
		raw, _ := json.Marshal(slot.Path)
		path := string(raw)
		if position, found := byPath[path]; found {
			result[position].Indices = append(result[position].Indices, index)
			continue
		}
		key, anchored, err := StableSlotKey(tree, slot.Path)
		if err != nil {
			return nil, err
		}
		var identity struct{ Anchor, Path []string }
		if json.Unmarshal([]byte(key), &identity) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "稳定字段身份无法编码")
		}
		anchorBytes, _ := platform.CanonicalJSON(identity.Anchor)
		anchor := string(anchorBytes)
		signature, known := shapes[anchor]
		if !known {
			signature = ProtectedAnchorShape(anchored)
			shapes[anchor] = signature
		}
		if prior, found := byStable[key]; found && prior != path {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "真实节点身份不能唯一定位保护字段")
		}
		byStable[key] = path
		byPath[path] = len(result)
		result = append(result, slotField{Key: key, Anchor: anchor, Shape: signature, Indices: []int{index}})
	}
	return result, nil
}

// BuildProtectedProjection 当前对象优先于上一完整代；只读取真实人工/保护单元。
func BuildProtectedProjection(ctx context.Context, op OperationContext, current json.RawMessage, objects []TranslationObject) (ProtectedProjection, error) {
	result := ProtectedProjection{Fields: map[string]protectedField{}, Shapes: map[string]string{}, Units: map[string]bool{}, Whole: map[string]bool{}, Rows: map[string]bool{}}
	decoder := json.NewDecoder(strings.NewReader(string(current)))
	decoder.UseNumber()
	if decoder.Decode(&result.Tree) != nil {
		return ProtectedProjection{}, platform.NewError(platform.CodeIntegrityError, false, "当前保护投影缺少完整源树")
	}
	for _, object := range objects {
		if object.Translation.TranslationId == "" {
			continue
		}
		protected := map[string]gen.TranslationUnit{}
		for _, unit := range object.Units {
			if !unit.Protected && unit.Origin != "human" {
				continue
			}
			protected[unit.UnitId] = unit
			result.Units[unit.UnitId] = true
			parent := unit.UnitId
			if unit.ParentBlockId != nil {
				parent = *unit.ParentBlockId
			}
			if parent == unit.UnitId {
				result.Whole[parent] = true
			} else {
				result.Rows[parent] = true
			}
		}
		if len(protected) == 0 {
			continue
		}
		split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{Document: object.Translation.Document, Source: string(object.SourceTree), TargetLanguage: object.Translation.TargetLanguage})
		if err != nil {
			return ProtectedProjection{}, err
		}
		var tree any
		decoder = json.NewDecoder(strings.NewReader(string(object.SourceTree)))
		decoder.UseNumber()
		if decoder.Decode(&tree) != nil {
			return ProtectedProjection{}, platform.NewError(platform.CodeIntegrityError, false, "人工译文原树损坏")
		}
		priorAnchors := map[string]bool{}
		for anchor := range result.Shapes {
			priorAnchors[anchor] = true
		}
		for _, template := range split.Templates {
			unit, found := protected[template.UnitId]
			if !found {
				continue
			}
			if unit.Status == "fallback" {
				return ProtectedProjection{}, platform.NewError(platform.CodeDependencyUnavailable, false, "人工或受保护旧译缺少可恢复结果")
			}
			values, err := ParseSlotValues(template.Slots, unit.TargetText)
			if err != nil {
				return ProtectedProjection{}, err
			}
			fields, err := CollectSlotFields(tree, template.Slots)
			if err != nil {
				return ProtectedProjection{}, err
			}
			for _, field := range fields {
				if priorAnchors[field.Anchor] {
					continue
				}
				result.Shapes[field.Anchor] = field.Shape
				if _, newer := result.Fields[field.Key]; newer {
					continue
				}
				value := protectedField{Values: []string{}, Fixed: []bool{}, Human: unit.Origin == "human"}
				for _, index := range field.Indices {
					value.Values = append(value.Values, values[index])
					value.Fixed = append(value.Fixed, template.Slots[index].Fixed)
				}
				result.Fields[field.Key] = value
			}
		}
	}
	// 当前节点即使已没有任何可译text/alt也核形状，不能跳过整个旧人工单元。
	known := map[string]string{}
	for anchor := range result.Shapes {
		var identity []string
		if json.Unmarshal([]byte(anchor), &identity) != nil || len(identity) != 3 {
			return ProtectedProjection{}, platform.NewError(platform.CodeIntegrityError, false, "保护节点身份损坏")
		}
		known[identity[1]+":"+identity[2]] = anchor
	}
	var inspect func(any) error
	inspect = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			kind, _ := node["type"].(string)
			name := sourceNodeIDAttributes[kind]
			if attrs, ok := node["attrs"].(map[string]any); ok && name != "" {
				if id, ok := attrs[name].(string); ok {
					if anchor, found := known[name+":"+id]; found {
						raw, _ := platform.CanonicalJSON([]string{kind, name, id})
						if string(raw) != anchor || ProtectedAnchorShape(node) != result.Shapes[anchor] {
							return platform.NewError(platform.CodeDependencyUnavailable, false, "同一稳定节点的无ID结构已变化，旧人工文本归属不能证明")
						}
					}
				}
			}
			for _, child := range node {
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := inspect(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := inspect(result.Tree); err != nil {
		return ProtectedProjection{}, err
	}
	return result, nil
}

// ProjectProtectedUnit 只复用证明同归属且段形状一致的译值，固定值永远取当前源。
func ProjectProtectedUnit(projection ProtectedProjection, template UnitTemplate, candidate gen.TranslationUnit) (gen.TranslationUnit, bool, bool, error) {
	fields, err := CollectSlotFields(projection.Tree, template.Slots)
	if err != nil {
		return gen.TranslationUnit{}, false, false, err
	}
	parent := template.ParentBlockId
	inherited := projection.Whole[parent] || (template.UnitId == parent && projection.Rows[parent])
	values := make([]string, len(template.Slots))
	for index, slot := range template.Slots {
		values[index] = slot.Value
	}
	reused := false
	human := false
	partial := false
	for _, field := range fields {
		if oldShape, known := projection.Shapes[field.Anchor]; known && oldShape != field.Shape {
			return gen.TranslationUnit{}, false, false, platform.NewError(platform.CodeDependencyUnavailable, false, "同一稳定节点内的无ID字段形状已改变，不能按数组位置归属人工译值")
		}
		old, found := projection.Fields[field.Key]
		translatable := false
		for _, index := range field.Indices {
			if !template.Slots[index].Fixed {
				translatable = true
			}
		}
		if !translatable {
			continue
		}
		if !found {
			partial = true
			continue
		}
		if len(old.Values) != len(field.Indices) {
			return gen.TranslationUnit{}, false, false, platform.NewError(platform.CodeDependencyUnavailable, false, "人工字段的URL或代码分段形状已改变")
		}
		for position, index := range field.Indices {
			slot := template.Slots[index]
			if old.Fixed[position] != slot.Fixed {
				return gen.TranslationUnit{}, false, false, platform.NewError(platform.CodeDependencyUnavailable, false, "人工字段的固定值归属无法证明")
			}
			if !slot.Fixed {
				values[index] = old.Values[position]
				reused = true
				human = human || old.Human
			}
		}
	}
	if !reused && !inherited {
		if projection.Units[candidate.UnitId] {
			return gen.TranslationUnit{}, false, false, platform.NewError(platform.CodeDependencyUnavailable, false, "人工单元仍在当前源但原文本归属无法证明")
		}
		return candidate, false, false, nil
	}
	var target strings.Builder
	characters := 0
	for index, slot := range template.Slots {
		number := strconv.Itoa(slot.Index)
		encoded := "<v" + number + "/>"
		if !slot.Fixed {
			encoded = "<i" + number + ">" + html.EscapeString(values[index]) + "</i" + number + ">"
		}
		characters += utf8.RuneCountInString(encoded)
		if characters > 65536 {
			return gen.TranslationUnit{}, false, false, platform.NewError(platform.CodeBudgetExceeded, false, "保护译文投影超过单元65536字符预算")
		}
		target.WriteString(encoded)
	}
	candidate.TargetText = target.String()
	candidate.TargetHash = platform.HashBytes([]byte(candidate.TargetText))
	candidate.Protected = true
	candidate.Status = "review"
	if human {
		candidate.Origin = "human"
	}
	return candidate, true, partial, nil
}
