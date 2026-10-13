package guard

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PoW struct {
	secret []byte
	mu     sync.Mutex
	used   map[string]int64
}

func NewPoW(secret []byte) *PoW {
	return &PoW{secret: append([]byte{}, secret...), used: map[string]int64{}}
}
func (p *PoW) Challenge(session string, difficulty int, now time.Time) string {
	if difficulty > 8 {
		difficulty = 8
	}
	if difficulty < 1 {
		difficulty = 1
	}
	expires := now.Add(5 * time.Minute).Unix()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return ""
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(session+"\x00"+hex.EncodeToString(random))) + ":" + strconv.FormatInt(expires, 10) + ":" + strconv.Itoa(difficulty)
	mac := hmac.New(sha256.New, p.secret)
	_, _ = mac.Write([]byte(payload))
	return payload + ":" + hex.EncodeToString(mac.Sum(nil))
}
func (p *PoW) Verify(challenge, nonce string, now time.Time, subject ...string) error {
	parts := strings.Split(challenge, ":")
	if len(parts) != 4 {
		return errors.New("工作量证明题目无效")
	}
	identity, identityErr := base64.RawURLEncoding.DecodeString(parts[0])
	if len(subject) != 1 || identityErr != nil || !strings.HasPrefix(string(identity), subject[0]+"\x00") {
		return errors.New("工作量证明来源无效")
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
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, expiry := range p.used {
		if now.Unix() > expiry {
			delete(p.used, key)
		}
	}
	if _, ok := p.used[challenge]; ok {
		return errors.New("工作量证明已使用")
	}
	p.used[challenge] = expires
	return nil
}
