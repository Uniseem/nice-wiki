package stats

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

type Beacon struct {
	Path, Language, Referrer, Country, Visitor string
	AI                                         bool
	Created                                    time.Time
}
type Writer interface {
	WriteBeacon(context.Context, Beacon) error
}

func NormalizeReferrer(raw string) string {
	value, err := url.Parse(raw)
	if err != nil || value.Host == "" {
		return ""
	}
	return strings.ToLower(value.Hostname())
}
func Record(ctx context.Context, writer Writer, value Beacon) error {
	if writer == nil || value.Path == "" || len(value.Path) > 2048 {
		return errors.New("访问记录无效")
	}
	value.Referrer = NormalizeReferrer(value.Referrer)
	value.Created = time.Now()
	return writer.WriteBeacon(ctx, value)
}
