package assets

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/store/sqlc"
	"os"
	"sync"
)

type Source interface {
	WithDocuments(func([]ReferenceDocument) error) error
}
type Store struct {
	logger *slog.Logger
	mu     sync.Mutex
	layout datadir.Layout
	db     *sql.DB
	source Source
}

func Open(layout datadir.Layout, db *sql.DB, source Source, logger *slog.Logger) *Store {
	return &Store{layout: layout, db: db, source: source, logger: logger}
}
func (s *Store) Get(ctx context.Context, hash string) (Metadata, error) {
	value, err := sqlc.New(s.db).GetAsset(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Metadata{}, ErrNotFound
	}
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Hash: value.Hash, Kind: value.Kind, Extension: value.Extension, MIME: value.MimeType, Size: value.Size, Name: value.OriginalName, CreatedAt: value.CreatedAt}, nil
}
func (s *Store) OpenFile(ctx context.Context, hash string) (Metadata, io.ReadCloser, error) {
	value, err := s.Get(ctx, hash)
	if err != nil {
		return Metadata{}, nil, err
	}
	if err := s.exists(value); err != nil {
		return Metadata{}, nil, err
	}
	path, err := s.path(value)
	if err != nil {
		return Metadata{}, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, nil, err
	}
	return value, file, nil
}
func (s *Store) path(value Metadata) (string, error) {
	return s.layout.Asset(value.Kind, value.Hash, value.Extension)
}
func (s *Store) exists(value Metadata) error {
	path, err := s.path(value)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != value.Size {
		return errors.New("资源文件类型或大小与索引不一致")
	}
	return nil
}
