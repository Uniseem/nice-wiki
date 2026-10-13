package guard

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Sessions struct{ secret []byte }

func NewSessions(secret []byte) *Sessions { return &Sessions{secret: append([]byte{}, secret...)} }
func (s *Sessions) New() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(raw)
	return s.Renew(id)
}
func (s *Sessions) Renew(id string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != 24 {
		return "", errors.New("问答会话无效")
	}
	payload := id + "." + strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Sessions) Verify(value string) (string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", errors.New("问答会话无效")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("问答会话无效")
	}
	mac := hmac.New(sha256.New, s.secret)
	payload := parts[0] + "." + parts[1]
	_, _ = mac.Write([]byte(payload))
	expires, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil || time.Now().Unix() > expires || !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errors.New("问答会话无效")
	}
	return parts[0], nil
}
