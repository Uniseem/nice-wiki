// Package media 实现资源媒体域的上传校验前置规则。
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// Hash 与公共 schema 的 SHA-256 标量对应。
type Hash = platform.Hash

// Identifier 与公共 schema 的不透明标识符对应。
type Identifier = platform.Identifier

// NormalizeOriginalNameInput 是展示文件名规范化输入。
type NormalizeOriginalNameInput struct {
	OriginalName string `json:"originalName"`
}

// NormalizeOriginalNameResult 是展示文件名规范化结果。
type NormalizeOriginalNameResult struct {
	SafeName string `json:"safeName"`
}

// HashUploadInput 是上传临时对象摘要计算输入。
type HashUploadInput struct {
	UploadID Identifier `json:"uploadId"`
}

// HashUploadResult 是上传临时对象摘要计算结果。
type HashUploadResult struct {
	SHA256   Hash `json:"sha256"`
	ByteSize int  `json:"byteSize"`
}

const (
	originalNameMaxRunes = 255
	maxUploadBytes       = int64(2147483648)
	hashBufferSize       = 64 * 1024
)

// NormalizeOriginalName 去掉路径组件、控制字符和双向控制符，仅保留安全
// 展示名。结果不参与任何文件路径拼接；空名、非法 UTF-8 和 . / .. 均拒绝。
func NormalizeOriginalName(input NormalizeOriginalNameInput) (NormalizeOriginalNameResult, error) {
	if !utf8.ValidString(input.OriginalName) || utf8.RuneCountInString(input.OriginalName) < 1 ||
		utf8.RuneCountInString(input.OriginalName) > originalNameMaxRunes {
		return NormalizeOriginalNameResult{}, platform.NewError(platform.CodeValidation, false, "文件名编码不合法")
	}

	components := strings.FieldsFunc(input.OriginalName, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(components) == 0 {
		return NormalizeOriginalNameResult{}, platform.NewError(platform.CodeValidation, false, "文件名不能为空")
	}

	component := components[len(components)-1]
	var builder strings.Builder
	builder.Grow(len(component))
	for _, r := range component {
		if unicode.IsControl(r) || isBidiControl(r) {
			continue
		}
		builder.WriteRune(r)
	}

	safeName := strings.TrimSpace(builder.String())
	if safeName == "" || safeName == "." || safeName == ".." ||
		!utf8.ValidString(safeName) || utf8.RuneCountInString(safeName) > originalNameMaxRunes {
		return NormalizeOriginalNameResult{}, platform.NewError(platform.CodeValidation, false, "文件名不合法")
	}

	return NormalizeOriginalNameResult{SafeName: safeName}, nil
}

// HashUpload 在服务生成的临时对象上以固定 64KiB 缓冲流式计算 SHA-256。
// 临时对象的打开和生命周期由本域私有存储边界提供；未装配时明确返回依赖
// 不可用，不能把未知对象状态当成空文件或成功。
func HashUpload(ctx context.Context, op OperationContext, input HashUploadInput) (HashUploadResult, error) {
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return HashUploadResult{}, err
	}
	defer cancel()

	if !validIdentifier(input.UploadID) {
		return HashUploadResult{}, platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	store := currentBlobStore()
	if store == nil {
		return HashUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储尚未装配")
	}

	reader, err := store.OpenUpload(workCtx, input.UploadID)
	if err != nil {
		return HashUploadResult{}, mapBlobStoreError(err, "读取上传临时对象失败")
	}
	defer func() { _ = reader.Close() }()

	hasher := sha256.New()
	buffer := make([]byte, hashBufferSize)
	var byteSize int64
	for {
		select {
		case <-workCtx.Done():
			return HashUploadResult{}, contextError(workCtx.Err())
		default:
		}

		readSize, readErr := reader.Read(buffer)
		if readSize < 0 || readSize > len(buffer) {
			return HashUploadResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象读取结果不合法")
		}
		if readSize > 0 {
			if byteSize > maxUploadBytes-int64(readSize) {
				return HashUploadResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象超过大小上限")
			}
			if _, err := hasher.Write(buffer[:readSize]); err != nil {
				return HashUploadResult{}, platform.NewError(platform.CodeIntegrityError, false, "计算上传对象摘要失败")
			}
			byteSize += int64(readSize)
		}
		if readSize == 0 && readErr == nil {
			return HashUploadResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象读取无进展")
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return HashUploadResult{}, mapBlobStoreError(readErr, "读取上传临时对象失败")
		}
	}

	if byteSize < 1 {
		return HashUploadResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象不能为空")
	}
	digest := hasher.Sum(nil)
	return HashUploadResult{
		SHA256:   Hash(hex.EncodeToString(digest)),
		ByteSize: int(byteSize),
	}, nil
}

func uploadOperationContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "操作上下文缺失")
	}
	if err := contextError(ctx.Err()); err != nil {
		return nil, nil, err
	}
	if op.Deadline == "" {
		return ctx, func() {}, nil
	}

	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return nil, nil, err
	}
	if !deadline.After(time.Now()) {
		return nil, nil, platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline)
	return workCtx, cancel, nil
}

func contextError(err error) error {
	switch err {
	case nil:
		return nil
	case context.Canceled:
		return platform.NewError(platform.CodeCancelled, true, "操作已取消")
	case context.DeadlineExceeded:
		return platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
	default:
		return platform.NewError(platform.CodeDependencyUnavailable, true, "操作上下文不可用")
	}
}

func validIdentifier(value Identifier) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == ':' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validHash(value Hash) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func validMediaType(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	if utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 128 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func isBidiControl(r rune) bool {
	switch {
	case r == '\u061c', r == '\u200e', r == '\u200f':
		return true
	case r >= '\u202a' && r <= '\u202e':
		return true
	case r >= '\u2066' && r <= '\u2069':
		return true
	default:
		return false
	}
}
