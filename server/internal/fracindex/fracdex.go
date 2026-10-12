package fracindex

import (
	"errors"
	"roci.dev/fracdex"
)

// 使用已确认的 rocicorp/fracdex CC0 依赖，不复制或修改其源代码。
func Between(before, after string) (string, error) {
	if before != "" && after != "" && before >= after {
		return "", errors.New("排序边界顺序无效")
	}
	return fracdex.KeyBetween(before, after)
}
