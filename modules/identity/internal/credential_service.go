package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"nice-wiki/libs/platform"
)

// Password 与 types.json 的敏感口令标量对应；本域不记录其原值。
type Password = string

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// HashPasswordInput 是生成口令哈希的输入。
type HashPasswordInput struct {
	Password Password `json:"password"`
}

// HashPasswordResult 是生成口令哈希的结果。
type HashPasswordResult struct {
	EncodedHash string `json:"encodedHash"`
}

// VerifyPasswordInput 是核对口令的输入。
type VerifyPasswordInput struct {
	Password    string `json:"password"`
	EncodedHash string `json:"encodedHash"`
}

// VerifyPasswordResult 是核对口令的结果。
type VerifyPasswordResult struct {
	Valid       bool `json:"valid"`
	NeedsRehash bool `json:"needsRehash"`
}

// ValidatePasswordChangeInput 是改密三输入规则的输入。
type ValidatePasswordChangeInput struct {
	OldPassword  string   `json:"oldPassword"`
	NewPassword  Password `json:"newPassword"`
	Confirmation Password `json:"confirmation"`
}

// ValidatePasswordChangeResult 是改密三输入规则的结果。
type ValidatePasswordChangeResult struct {
	Valid bool `json:"valid"`
}

const (
	argonMemoryKiB     = uint32(65536)
	argonTime          = uint32(3)
	argonParallelism   = uint8(4)
	argonHashBytes     = uint32(32)
	argonSaltBytes     = 16
	argonMinSaltBytes  = 8
	argonMaxSaltBytes  = 64
	argonMaxHashBytes  = 64
	passwordMaxRunes   = 256
	passwordMinRunes   = 8
	verifyMinRunes     = 1
	passwordConcurrency = 4
)

// Argon2id 需要有界并发，避免多个登录/改密请求同时消耗过多内存。
var passwordSlots = make(chan struct{}, passwordConcurrency)

// HashPassword 使用合同固定参数生成 PHC Argon2id v19 编码哈希。
func HashPassword(ctx context.Context, op OperationContext, input HashPasswordInput) (HashPasswordResult, error) {
	if !validPassword(input.Password) {
		return HashPasswordResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}

	workCtx, cancel, err := credentialContext(ctx, op)
	if err != nil {
		return HashPasswordResult{}, err
	}
	defer cancel()
	if err := acquirePasswordSlot(workCtx); err != nil {
		return HashPasswordResult{}, err
	}
	defer func() { <-passwordSlots }()

	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return HashPasswordResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "随机源不可用")
	}
	defer clear(salt)

	passwordBytes := []byte(input.Password)
	defer clear(passwordBytes)
	derived := argon2.IDKey(passwordBytes, salt, argonTime, argonMemoryKiB, argonParallelism, argonHashBytes)
	if err := credentialContextError(workCtx.Err()); err != nil {
		clear(derived)
		return HashPasswordResult{}, err
	}
	encoded := encodeArgonHash(salt, derived)
	clear(derived)
	return HashPasswordResult{EncodedHash: encoded}, nil
}

// VerifyPassword 按 PHC 编码派生并以常量时间核对口令；超出当前成本上限
// 的编码或损坏编码一律隔离为完整性错误，不执行降级算法。
func VerifyPassword(input VerifyPasswordInput) (VerifyPasswordResult, error) {
	if !validVerifyPassword(input.Password) || len(input.EncodedHash) == 0 || len(input.EncodedHash) > 512 || !utf8.ValidString(input.EncodedHash) {
		return VerifyPasswordResult{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希不合法")
	}
	parsed, err := parseArgonHash(input.EncodedHash)
	if err != nil {
		return VerifyPasswordResult{}, err
	}

	if err := acquirePasswordSlot(context.Background()); err != nil {
		return VerifyPasswordResult{}, err
	}
	defer func() { <-passwordSlots }()

	passwordBytes := []byte(input.Password)
	defer clear(passwordBytes)
	expected := argon2.IDKey(passwordBytes, parsed.salt, parsed.time, parsed.memory, parsed.parallelism, uint32(len(parsed.digest)))
	defer clear(expected)
	valid := subtle.ConstantTimeCompare(expected, parsed.digest) == 1
	return VerifyPasswordResult{
		Valid:       valid,
		NeedsRehash: valid && (parsed.memory != argonMemoryKiB || parsed.time != argonTime || parsed.parallelism != argonParallelism || uint32(len(parsed.digest)) != argonHashBytes),
	}, nil
}

// ValidatePasswordChange 校验改密的长度、两次新口令一致且不同于旧口令。
// 语义不满足时返回 valid=false；输入超出 schema 时返回 VALIDATION，且不回显口令。
func ValidatePasswordChange(input ValidatePasswordChangeInput) (ValidatePasswordChangeResult, error) {
	if !validNonEmptyPassword(input.OldPassword) || !validPassword(input.NewPassword) || !validPassword(input.Confirmation) {
		return ValidatePasswordChangeResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}
	if subtle.ConstantTimeCompare([]byte(input.NewPassword), []byte(input.Confirmation)) != 1 {
		return ValidatePasswordChangeResult{Valid: false}, nil
	}
	if !passwordEqual(input.OldPassword, input.NewPassword) {
		return ValidatePasswordChangeResult{Valid: true}, nil
	}
	return ValidatePasswordChangeResult{Valid: false}, nil
}

type parsedArgonHash struct {
	memory      uint32
	time        uint32
	parallelism uint8
	salt        []byte
	digest      []byte
}

func validPassword(password string) bool {
	return validUTF8Runes(password, passwordMinRunes, passwordMaxRunes)
}

func validNonEmptyPassword(password string) bool {
	return validUTF8Runes(password, verifyMinRunes, passwordMaxRunes)
}

func validVerifyPassword(password string) bool {
	return validUTF8Runes(password, verifyMinRunes, passwordMaxRunes)
}

func validUTF8Runes(value string, min, max int) bool {
	if !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	return length >= min && length <= max
}

func passwordEqual(left, right string) bool {
	leftDigest := sha256.Sum256([]byte(left))
	rightDigest := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftDigest[:], rightDigest[:]) == 1
}

func encodeArgonHash(salt, digest []byte) string {
	encoding := base64.RawStdEncoding
	return "$argon2id$v=19$m=65536,t=3,p=4$" + encoding.EncodeToString(salt) + "$" + encoding.EncodeToString(digest)
}

func parseArgonHash(encoded string) (parsedArgonHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希编码不合法")
	}

	var parsed parsedArgonHash
	var seenMemory, seenTime, seenParallel bool
	for _, parameter := range strings.Split(parts[3], ",") {
		name, value, ok := strings.Cut(parameter, "=")
		if !ok || value == "" {
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
		}
		parsedValue, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsedValue == 0 {
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
		}
		switch name {
		case "m":
			if seenMemory {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数重复")
			}
			parsed.memory, seenMemory = uint32(parsedValue), true
		case "t":
			if seenTime {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数重复")
			}
			parsed.time, seenTime = uint32(parsedValue), true
		case "p":
			if seenParallel || parsedValue > uint64(^uint8(0)) {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
			}
			parsed.parallelism, seenParallel = uint8(parsedValue), true
		default:
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不支持")
		}
	}
	if !seenMemory || !seenTime || !seenParallel || parsed.memory > argonMemoryKiB || parsed.time > argonTime || parsed.parallelism > argonParallelism || parsed.memory < 8*uint32(parsed.parallelism) {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希成本不合法")
	}

	var err error
	parsed.salt, err = decodeArgonBase64(parts[4])
	if err != nil || len(parsed.salt) < argonMinSaltBytes || len(parsed.salt) > argonMaxSaltBytes {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希盐不合法")
	}
	parsed.digest, err = decodeArgonBase64(parts[5])
	if err != nil || len(parsed.digest) == 0 || len(parsed.digest) > argonMaxHashBytes {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希摘要不合法")
	}
	return parsed, nil
}

func decodeArgonBase64(value string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func credentialContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "口令计算上下文缺失")
	}
	if err := credentialContextError(ctx.Err()); err != nil {
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
		return nil, nil, platform.NewError(platform.CodeDeadlineExceeded, true, "口令计算已超时")
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline)
	return workCtx, cancel, nil
}

func acquirePasswordSlot(ctx context.Context) error {
	select {
	case passwordSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return credentialContextError(ctx.Err())
	}
}

func credentialContextError(err error) error {
	switch err {
	case nil:
		return nil
	case context.Canceled:
		return platform.NewError(platform.CodeCancelled, true, "口令计算已取消")
	case context.DeadlineExceeded:
		return platform.NewError(platform.CodeDeadlineExceeded, true, "口令计算已超时")
	default:
		return platform.NewError(platform.CodeDependencyUnavailable, true, "口令计算上下文不可用")
	}
}
