package translate

import (
	"context"
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
)

type StringModel interface {
	TranslateStrings(context.Context, string, map[string]string) (map[string]string, error)
}

func TranslateStrings(ctx context.Context, model StringModel, language, path string, source map[string]string) error {
	existing := map[string]string{}
	if err := fsutil.ReadJSON(path, &existing); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pending := map[string]string{}
	for key, value := range source {
		if existing[key] == "" {
			pending[key] = value
		}
	}
	if len(pending) == 0 {
		return nil
	}
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
	}
	return fsutil.WriteJSON(path, existing)
}
