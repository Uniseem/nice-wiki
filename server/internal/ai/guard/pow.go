package guard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

type PoW struct{ secret []byte }

func NewPoW(secret []byte) *PoW { return &PoW{append([]byte{}, secret...)} }
func (p *PoW) Challenge(session string, difficulty int, now time.Time) string {
	if difficulty < 1 {
		difficulty = 1
	}
	expires := now.Add(5 * time.Minute).Unix()
	payload := session + ":" + strconv.FormatInt(expires, 10) + ":" + strconv.Itoa(difficulty)
	mac := hmac.New(sha256.New, p.secret)
	_, _ = mac.Write([]byte(payload))
	return payload + ":" + hex.EncodeToString(mac.Sum(nil))
}
func (p *PoW) Verify(challenge, nonce string, now time.Time) error {
	parts := strings.Split(challenge, ":")
	if len(parts) != 4 {
		return errors.New("工作量证明题目无效")
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > expires {
		return errors.New("工作量证明已过期")
	}
	difficulty, err := strconv.Atoi(parts[2])
	if err != nil || difficulty < 1 || difficulty > 8 {
		return errors.New("工作量证明难度无效")
	}
	payload := strings.Join(parts[:3], ":")
	mac := hmac.New(sha256.New, p.secret)
	_, _ = mac.Write([]byte(payload))
	expected, err := hex.DecodeString(parts[3])
	if err != nil || !hmac.Equal(expected, mac.Sum(nil)) {
		return errors.New("工作量证明签名无效")
	}
	sum := sha256.Sum256([]byte(challenge + ":" + nonce))
	for i := 0; i < difficulty; i++ {
		if sum[i] != 0 {
			return errors.New("工作量证明答案无效")
		}
	}
	return nil
}
