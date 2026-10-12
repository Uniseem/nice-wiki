package ingest

import (
	"context"
	"errors"
	"io"
	"mime"
	"path/filepath"
	"strings"
)

type FileModel interface {
	ExtractFile(context.Context, string, io.Reader) (string, error)
}

func ExtractAttachment(ctx context.Context, name, contentType string, reader io.Reader, model FileModel) (string, error) {
	extension := strings.ToLower(filepath.Ext(name))
	if strings.HasPrefix(contentType, "text/") || extension == ".md" || extension == ".csv" {
		data, err := io.ReadAll(io.LimitReader(reader, 8<<20))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if model == nil {
		return "", errors.New("附件提取模型尚未配置")
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(extension)
	}
	return model.ExtractFile(ctx, contentType, io.LimitReader(reader, 64<<20))
}
