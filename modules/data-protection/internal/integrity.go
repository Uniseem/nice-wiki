package protection

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"nice-wiki/libs/platform"
	"os"
)

type VerifyObjectHashInput struct {
	ObjectPath    string `json:"objectPath"`
	ExpectedHash  string `json:"expectedHash"`
	ExpectedBytes int64  `json:"expectedBytes"`
}
type VerifyObjectHashResult struct {
	Valid bool `json:"valid"`
}

func VerifyObjectHash(ctx context.Context, op OperationContext, in VerifyObjectHashInput) (VerifyObjectHashResult, error) {
	store, err := Catalog(op)
	if err != nil {
		return VerifyObjectHashResult{}, err
	}
	path, err := PrivateObjectPath(store, in.ObjectPath)
	if err != nil {
		return VerifyObjectHashResult{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return VerifyObjectHashResult{}, storageError(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return VerifyObjectHashResult{}, storageError(err)
	}
	if info.Size() != in.ExpectedBytes {
		return VerifyObjectHashResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象长度与引用不符")
	}
	expected, err := hex.DecodeString(in.ExpectedHash)
	if err != nil || len(expected) != sha256.Size {
		return VerifyObjectHashResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象摘要格式不合法")
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, in.ExpectedBytes+1))
	if err != nil {
		return VerifyObjectHashResult{}, storageError(err)
	}
	if count != in.ExpectedBytes || subtle.ConstantTimeCompare(digest.Sum(nil), expected) != 1 {
		return VerifyObjectHashResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象完整性校验失败")
	}
	return VerifyObjectHashResult{Valid: true}, nil
}
