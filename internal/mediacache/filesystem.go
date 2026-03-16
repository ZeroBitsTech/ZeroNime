package mediacache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type FilesystemStore struct {
	baseDir string
}

func NewFilesystemStore(baseDir string) *FilesystemStore {
	return &FilesystemStore{baseDir: baseDir}
}

func (s *FilesystemStore) Put(_ context.Context, key string, body []byte) error {
	targetPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, body, 0o644)
}

func (s *FilesystemStore) Get(_ context.Context, key string) ([]byte, error) {
	targetPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	body, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", key, err)
	}
	return body, nil
}
