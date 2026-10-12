package memory

import (
	"errors"
	"unicode/utf8"
)

const TaskLimit = 5 << 10

func Validate(item Item) error {
	if item.ID == "" || item.Title == "" {
		return errors.New("记忆身份或标题不能为空")
	}
	limit := item.Limit
	if limit <= 0 {
		limit = 1536
	}
	if len(item.Content) > limit || !utf8.ValidString(item.Content) {
		return errors.New("记忆内容超过上限或编码无效")
	}
	return nil
}
func InjectionSize(values []Item) error {
	size := 0
	for _, v := range values {
		size += len(v.Description) + len(v.Content)
	}
	if size > TaskLimit {
		return errors.New("任务注入记忆超过 5KB")
	}
	return nil
}
