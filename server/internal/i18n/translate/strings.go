package translate

import (
	"context"
	"errors"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/textutil"
	"os"
)

const sourceHashPrefix = "__nw_source_hash__:"

type StringModel interface {
	TranslateStrings(context.Context, string, map[string]string) (map[string]string, error)
}

func TranslateStrings(ctx context.Context, model StringModel, language, path string, source map[string]string) error {
	existing := map[string]string{}
	if err := fsutil.ReadJSON(path, &existing); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pending := map[string]string{}
	dirty := false
	for key, value := range source {
		hash := textutil.Hash([]byte(value))
		if value == "" {
			if existing[key] != "" || existing[sourceHashPrefix+key] != hash {
				existing[key] = ""
				existing[sourceHashPrefix+key] = hash
				dirty = true
			}
			continue
		}
		if existing[key] == "" || existing[sourceHashPrefix+key] != hash {
			pending[key] = value
		}
	}
	for key := range existing {
		if len(key) >= len(sourceHashPrefix) && key[:len(sourceHashPrefix)] == sourceHashPrefix {
			continue
		}
		if _, ok := source[key]; !ok {
			delete(existing, key)
			delete(existing, sourceHashPrefix+key)
			dirty = true
		}
	}
	if len(pending) == 0 && !dirty {
		return nil
	}
	if len(pending) > 0 {
		translated, err := model.TranslateStrings(ctx, language, pending)
		if err != nil {
			return err
		}
		for key := range pending {
			value, ok := translated[key]
			if !ok || value == "" {
				return errors.New("站点字符串译文不完整")
			}
			existing[key] = value
			existing[sourceHashPrefix+key] = textutil.Hash([]byte(source[key]))
		}
	}
	return fsutil.WriteJSON(path, existing)
}
