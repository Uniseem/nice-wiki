package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 全局限制 Argon2 同时运行数量，避免并发登录把内存耗尽。
var passwordSlots = make(chan struct{}, 2)

func reservePasswordSlot() func() { passwordSlots <- struct{}{}; return func() { <-passwordSlots } }

const passwordMemory = 64 * 1024
const passwordIterations = 3
const passwordParallelism = 2

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 8 || len(password) > 1024 {
		return invalid("密码至少 8 位，最多 1024 字节")
	}
	return nil
}
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	release := reservePasswordSlot()
	defer release()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, 32)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordIterations, passwordParallelism, encoding.EncodeToString(salt), encoding.EncodeToString(hash)), nil
}
func VerifyPassword(password, encoded string) bool {
	if len(password) > 1024 {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}
	values := make([]uint64, 3)
	for index, prefix := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(params[index], prefix) {
			return false
		}
		value, err := strconv.ParseUint(strings.TrimPrefix(params[index], prefix), 10, 32)
		if err != nil {
			return false
		}
		values[index] = value
	}
	if values[0] < 8*1024 || values[0] > 128*1024 || values[1] < 1 || values[1] > 6 || values[2] < 1 || values[2] > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	release := reservePasswordSlot()
	defer release()
	actual := argon2.IDKey([]byte(password), salt, uint32(values[1]), uint32(values[0]), uint8(values[2]), 32)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}
