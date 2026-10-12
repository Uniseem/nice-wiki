package llm

import (
	"encoding/json"
	"errors"
)

func DecodeStructured[T any](text string) (T, error) {
	var value T
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return value, errors.New("模型结构化输出无效")
	}
	return value, nil
}
