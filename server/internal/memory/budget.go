package memory

import (
	"errors"
	"golang.org/x/text/language"
	"nice-wiki/internal/aitask"
	"unicode/utf8"
)

const TaskLimit = 5 << 10

func Validate(item Item) error {
	if item.ID == "" || item.Title == "" || item.Source != SourceManual && item.Source != SourceAssistant && item.Source != SourceAutomatic {
		return errors.New("记忆身份或标题不能为空")
	}
	limit := item.Limit
	if limit <= 0 {
		limit = 1536
	}
	if len(item.Title)+len(item.Description)+len(item.Content) > limit || !utf8.ValidString(item.Title) || !utf8.ValidString(item.Description) || !utf8.ValidString(item.Content) {
		return errors.New("记忆内容超过上限或编码无效")
	}
	for _, code := range item.Languages {
		if _, err := language.Parse(code); err != nil {
			return errors.New("记忆适用语言无效")
		}
	}
	return nil
}
func ValidateMappings(store *Store, mapping *Mapping, overrides map[string]Item) error {
	for _, ids := range mapping.All() {
		values := []Item{}
		for _, id := range ids {
			value, ok := overrides[id]
			if !ok {
				var err error
				value, err = store.Get(id)
				if err != nil {
					return err
				}
			}
			values = append(values, value)
		}
		if err := InjectionSize(values); err != nil {
			return err
		}
	}
	return nil
}

func ValidateMappingValues(store *Store, values map[string][]string) error {
	for task, ids := range values {
		if _, ok := aitask.Lookup(task); !ok {
			return errors.New("未知 AI 任务")
		}
		items := []Item{}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return errors.New("任务记忆映射重复")
			}
			seen[id] = true
			item, err := store.Get(id)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := InjectionSize(items); err != nil {
			return err
		}
	}
	return nil
}
func InjectionSize(values []Item) error {
	size := 0
	for _, v := range values {
		size += len(v.Title) + len(v.Description) + len(v.Content)
	}
	if size > TaskLimit {
		return errors.New("任务注入记忆超过 5KB")
	}
	return nil
}
