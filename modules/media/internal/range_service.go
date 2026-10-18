package media

import (
	"nice-wiki/libs/platform"
	"strconv"
	"strings"
)

type ParseByteRangeInput struct {
	Header  *string `json:"header"`
	Total   int64   `json:"total"`
	IfRange *string `json:"ifRange"`
	Etag    string  `json:"etag"`
}
type ByteRange struct {
 Start int64 `json:"start"`
 End int64 `json:"end"`
 Total int64 `json:"total"`
 Partial bool `json:"partial"`
}
type ParseByteRangeResult struct {
	Range ByteRange `json:"range"`
}

// ParseByteRange只规范一个bytes区间；日期If-Range没有已证明修改时间，因此回全量。
func ParseByteRange(input ParseByteRangeInput) (ParseByteRangeResult, error) {
	full := ParseByteRangeResult{Range: ByteRange{Start: 0, End: input.Total - 1, Total: input.Total, Partial: false}}
	if input.Header == nil || (input.IfRange != nil && *input.IfRange != input.Etag) {
		return full, nil
	}
	invalid := func() (ParseByteRangeResult, error) {
		return ParseByteRangeResult{}, platform.NewError(platform.CodeValidation, false, "请求的单字节范围不可满足")
	}
	header := *input.Header
	if !strings.HasPrefix(header, "bytes=") || strings.ContainsAny(header, ", \t\r\n") {
		return invalid()
	}
	left, right, ok := strings.Cut(header[len("bytes="):], "-")
	if !ok || strings.Contains(right, "-") || (left == "" && right == "") {
		return invalid()
	}
	number := func(value string) (int64, bool) {
		if value == "" {
			return 0, false
		}
		for _, digit := range value {
			if digit < '0' || digit > '9' {
				return 0, false
			}
		}
		v, err := strconv.ParseInt(value, 10, 64)
		return v, err == nil
	}
	start, end := int64(0), input.Total-1
	if left == "" {
		suffix, ok := number(right)
		if !ok || suffix == 0 {
			return invalid()
		}
		if suffix < input.Total {
			start = input.Total - suffix
		}
	} else {
		var ok bool
		start, ok = number(left)
		if !ok || start >= input.Total {
			return invalid()
		}
		if right != "" {
			end, ok = number(right)
			if !ok || end < start {
				return invalid()
			}
			if end >= input.Total {
				end = input.Total - 1
			}
		}
	}
	return ParseByteRangeResult{Range: ByteRange{Start: start, End: end, Total: input.Total, Partial: true}}, nil
}
