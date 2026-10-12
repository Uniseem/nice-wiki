package assets

import (
	"context"
	"errors"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/store/sqlc"
	"os"
)

func (s *Store) List(ctx context.Context) (result []Metadata, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.source == nil {
		return nil, errors.New("资源引用来源尚未接入")
	}
	err = s.source.WithDocuments(func(documents []ReferenceDocument) error {
		if err := s.refreshReferences(ctx, documents); err != nil {
			return err
		}
		values, err := sqlc.New(s.db).ListAssets(ctx)
		if err != nil {
			return err
		}
		result = make([]Metadata, len(values))
		for i, value := range values {
			result[i] = Metadata{Hash: value.Hash, Kind: value.Kind, Extension: value.Extension, MIME: value.MimeType, Size: value.Size, Name: value.OriginalName, CreatedAt: value.CreatedAt, References: value.ReferenceCount}
		}
		return nil
	})
	return result, err
}

func (s *Store) Delete(ctx context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.source == nil {
		return errors.New("资源引用来源尚未接入，不能清理")
	}
	// 快照持锁到文件与索引均删除，避免检查之后被正文再次引用。
	return s.source.WithDocuments(func(documents []ReferenceDocument) error {
		if err := s.refreshReferences(ctx, documents); err != nil {
			return err
		}
		values, err := sqlc.New(s.db).ListAssets(ctx)
		if err != nil {
			return err
		}
		var value Metadata
		found := false
		for _, entry := range values {
			if entry.Hash != hash {
				continue
			}
			if entry.ReferenceCount > 0 {
				return ErrReferenced
			}
			value = Metadata{Hash: entry.Hash, Kind: entry.Kind, Extension: entry.Extension, Size: entry.Size}
			found = true
			break
		}
		if !found {
			return ErrNotFound
		}
		path, err := s.path(value)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := sqlc.New(s.db).DeleteAsset(ctx, hash); err != nil {
			return errors.Join(err, fsutil.WriteFile(path, data, 0600))
		}
		return nil
	})
}
