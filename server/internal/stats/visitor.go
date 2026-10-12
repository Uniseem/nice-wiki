package stats

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Salt struct {
	Day   string
	Value []byte
}

func NewSalt(now time.Time) (Salt, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return Salt{}, err
	}
	return Salt{Day: now.UTC().Format("2006-01-02"), Value: value}, nil
}
func Visitor(salt Salt, ip, userAgent string) string {
	mac := hmac.New(sha256.New, salt.Value)
	_, _ = mac.Write([]byte(ip))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userAgent))
	return hex.EncodeToString(mac.Sum(nil))
}
