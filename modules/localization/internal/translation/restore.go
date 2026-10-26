package translation

import (
	"encoding/json"
	"html"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"regexp"
	"strconv"
	"strings"
)

// RestoreSourceTree 只替换模板指向的text/alt，原链接、代码、公式与全部结构属性保留。
func RestoreSourceTree(source json.RawMessage, templates []UnitTemplate, translated []gen.TranslationUnit) (json.RawMessage, error) {
	var tree any
	d := json.NewDecoder(strings.NewReader(string(source)))
	d.UseNumber()
	if d.Decode(&tree) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "译文还原缺少完整源树")
	}
	byID := map[string]gen.TranslationUnit{}
	known := map[string]bool{}
	for _, template := range templates {
		known[template.UnitId] = true
	}
	for _, unit := range translated {
		if !known[unit.UnitId] {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "译文还原包含未知块或表格行")
		}
		if _, exists := byID[unit.UnitId]; exists {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "译文还原存在重复单元")
		}
		byID[unit.UnitId] = unit
	}
	for _, template := range templates {
		unit, exists := byID[template.UnitId]
		if !exists {
			continue
		}
		values, err := ParseSlotValues(template.Slots, unit.TargetText)
		if err != nil {
			return nil, err
		}
		paths := map[string][]string{}
		originals := map[string]string{}
		replacement := map[string]string{}
		order := []string{}
		for i, slot := range template.Slots {
			if len(slot.Path) == 0 {
				continue
			}
			keyBytes, _ := json.Marshal(slot.Path)
			key := string(keyBytes)
			if _, seen := paths[key]; !seen {
				paths[key] = slot.Path
				order = append(order, key)
			}
			originals[key] += slot.Value
			replacement[key] += values[i]
		}
		for _, key := range order {
			if err = ReplaceSourceText(tree, paths[key], originals[key], replacement[key]); err != nil {
				return nil, err
			}
		}
	}
	raw, err := platform.CanonicalJSON(tree)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "译文树还原编码失败")
	}
	if err = localizationruntime.ValidateSourcePayload("ContentDocumentBody", raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func ParseSlotValues(slots []InlineSlot, target string) ([]string, error) {
	result := make([]string, 0, len(slots))
	rest := target
	for _, slot := range slots {
		number := strconv.Itoa(slot.Index)
		if slot.Fixed {
			token := "<v" + number + "/>"
			if !strings.HasPrefix(rest, token) {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "译文固定值标签不一致")
			}
			rest = strings.TrimPrefix(rest, token)
			result = append(result, slot.Value)
			continue
		}
		open, close := "<i"+number+">", "</i"+number+">"
		if !strings.HasPrefix(rest, open) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "译文文本标签边界不一致")
		}
		rest = strings.TrimPrefix(rest, open)
		end := strings.Index(rest, close)
		if end < 0 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "译文文本标签缺少结束")
		}
		value := rest[:end]
		if strings.Contains(value, "<") {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "译文文本值含未转义标签")
		}
		result = append(result, html.UnescapeString(value))
		rest = rest[end+len(close):]
	}
	if rest != "" {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "译文标签之外含未归属文本")
	}
	return result, nil
}

var textTagPattern = regexp.MustCompile(`</?i\d+/?>`)

// RepairSlotText 在译文标签套不回原文时补救：语序和中文不同的语言（如德语）常把行内代码挪进句中，
// 模型随之把文本标签嵌套错或写成空标签。只要每个固定值标签按原顺序恰好出现一次，就丢掉写乱的文本标签，
// 以固定值为界把文字切回原来的文本段，重写成严格格式。两个固定值之间有多个文本段（例如先加粗再普通）
// 分不出边界，固定值之外出现文本段装不下的文字，或文字里还剩别的标签，都不修，返回 false。
func RepairSlotText(slots []InlineSlot, target string) (string, bool) {
	rest := textTagPattern.ReplaceAllString(target, "")
	var out strings.Builder
	for i := 0; i < len(slots); i++ {
		slot := slots[i]
		number := strconv.Itoa(slot.Index)
		if !slot.Fixed {
			if i+1 < len(slots) && !slots[i+1].Fixed {
				return "", false
			}
			end := len(rest)
			if i+1 < len(slots) {
				end = strings.Index(rest, "<v"+strconv.Itoa(slots[i+1].Index)+"/>")
				if end < 0 {
					return "", false
				}
			}
			out.WriteString("<i" + number + ">" + rest[:end] + "</i" + number + ">")
			rest = rest[end:]
			continue
		}
		token := "<v" + number + "/>"
		at := strings.Index(rest, token)
		// 固定值前只允许空白（原文这里没有文本段可放）。
		if at < 0 || strings.TrimSpace(rest[:at]) != "" {
			return "", false
		}
		out.WriteString(token)
		rest = rest[at+len(token):]
	}
	if strings.TrimSpace(rest) != "" {
		return "", false
	}
	repaired := out.String()
	if _, err := ParseSlotValues(slots, repaired); err != nil {
		return "", false
	}
	return repaired, true
}

func ReplaceSourceText(root any, path []string, expected, replacement string) error {
	if len(path) == 0 || (path[len(path)-1] != "text" && path[len(path)-1] != "alt") {
		return platform.NewError(platform.CodeIntegrityError, false, "译文模板只能定位text或alt")
	}
	cursor := root
	for _, segment := range path[:len(path)-1] {
		switch current := cursor.(type) {
		case map[string]any:
			cursor = current[segment]
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(current) {
				return platform.NewError(platform.CodeIntegrityError, false, "译文模板数组路径不合法")
			}
			cursor = current[index]
		default:
			return platform.NewError(platform.CodeIntegrityError, false, "译文模板路径不是完整源树")
		}
	}
	object, ok := cursor.(map[string]any)
	if !ok {
		return platform.NewError(platform.CodeIntegrityError, false, "译文模板文本节点不合法")
	}
	field := path[len(path)-1]
	value, ok := object[field].(string)
	if !ok || value != expected {
		return platform.NewError(platform.CodeIntegrityError, false, "译文模板与原始文本不一致")
	}
	object[field] = replacement
	return nil
}
