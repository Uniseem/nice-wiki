package publicsite

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"nice-wiki/libs/platform"
	"strconv"
)

type BuildCacheKeyInput struct {
	DocumentID          string
	PublicationRevision int64
	MetadataRevision    int64
	Locale              string
	ThemeRevision       int64
	DataEpoch           string
}
type BuildCacheKeyResult struct{ Key string }

// BuildCacheKey 对每字段长度编码，避免分隔符碰撞；渲染版本升级自动隔离旧结果。
func BuildCacheKey(ctx context.Context, op platform.OperationContext, in BuildCacheKeyInput) (BuildCacheKeyResult, error) {
	hash := sha256.New()
	var length [8]byte
	for _, field := range []string{"public-site-renderer-v2:" + assetVersion, in.DataEpoch, in.DocumentID, strconv.FormatInt(in.PublicationRevision, 10), strconv.FormatInt(in.MetadataRevision, 10), in.Locale, strconv.FormatInt(in.ThemeRevision, 10)} {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		hash.Write(length[:])
		hash.Write([]byte(field))
	}
	return BuildCacheKeyResult{hex.EncodeToString(hash.Sum(nil))}, nil
}
