// Package secretbox 用部署主密钥（32 字节文件）以 AES-256-GCM 加密需要落库的密钥材料：
// 第三方登录密钥、用户自带的模型密钥、DNS 令牌、备份目标凭据等。
// 附加数据绑定用途，密文不能挪到别的用途下解开。
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"os"
)

// Box 持有主密钥；零值不可用。
type Box struct{ aead cipher.AEAD }

// Load 读取主密钥文件（恰好 32 字节原始内容）。
func Load(ref string) (*Box, error) {
	raw, err := os.ReadFile(ref)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("secretbox: 主密钥文件不可用")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal 返回随机 nonce 与密文；purpose 是附加数据（如 "oauth-secret:github"）。
func (b *Box) Seal(plain []byte, purpose string) (nonce, ciphertext []byte, err error) {
	nonce = make([]byte, b.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, b.aead.Seal(nil, nonce, plain, []byte(purpose)), nil
}

// Open 解密；用途不符或被篡改时返回错误。
func (b *Box) Open(nonce, ciphertext []byte, purpose string) ([]byte, error) {
	if len(nonce) != b.aead.NonceSize() {
		return nil, errors.New("secretbox: nonce 长度不符")
	}
	return b.aead.Open(nil, nonce, ciphertext, []byte(purpose))
}
