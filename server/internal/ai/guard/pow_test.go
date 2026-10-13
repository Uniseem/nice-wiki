package guard

import (
	"crypto/sha256"
	"strconv"
	"testing"
	"time"
)

func solve(challenge string) string {
	for i := 0; ; i++ {
		nonce := strconv.Itoa(i)
		sum := sha256.Sum256([]byte(challenge + ":" + nonce))
		if sum[0] == 0 {
			return nonce
		}
	}
}
func TestPoWAddressBinding(t *testing.T) {
	for _, address := range []string{"192.0.2.1:1234", "[2001:db8::1]:443"} {
		p := NewPoW([]byte("test-secret"))
		now := time.Now()
		challenge := p.Challenge(address, 1, now)
		nonce := solve(challenge)
		if err := p.Verify(challenge, nonce, now, "other"); err == nil {
			t.Fatal("接受了其他来源")
		}
		if err := p.Verify(challenge, nonce, now, address); err != nil {
			t.Fatal(err)
		}
		if err := p.Verify(challenge, nonce, now, address); err == nil {
			t.Fatal("接受重放")
		}
		next := p.Challenge(address, 1, now)
		if next == challenge {
			t.Fatal("题目未随机化")
		}
		if err := p.Verify(next, solve(next), now.Add(6*time.Minute), address); err == nil {
			t.Fatal("接受过期题目")
		}
	}
}
