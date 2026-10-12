package textutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Hash(data []byte) string { hash := sha256.Sum256(data); return hex.EncodeToString(hash[:]) }
func HashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return Hash(data), nil
}
