package httpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/mediacache"
)

type predictiveEpisode struct {
	EpisodeID string
	Candidate domain.StreamCandidate
}

type predictiveMetadataStore struct {
	mu      sync.RWMutex
	entries map[string]domain.StartupMediaCache
}

func newPredictiveMetadataStore() *predictiveMetadataStore {
	return &predictiveMetadataStore{entries: map[string]domain.StartupMediaCache{}}
}

func (s *predictiveMetadataStore) GetStartupMediaCache(episodeID string) (*domain.StartupMediaCache, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[episodeID]
	if !ok {
		return nil, false, nil
	}
	copyEntry := entry
	return &copyEntry, true, nil
}

func (s *predictiveMetadataStore) UpsertStartupMediaCache(entry domain.StartupMediaCache) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.EpisodeID] = entry
	return nil
}

func (s *predictiveMetadataStore) DeleteStartupMediaCache(episodeID string) (*domain.StartupMediaCache, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[episodeID]
	if !ok {
		return nil, false
	}
	delete(s.entries, episodeID)
	copyEntry := entry
	return &copyEntry, true
}

func (s *predictiveMetadataStore) Snapshot() map[string]domain.StartupMediaCache {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]domain.StartupMediaCache, len(s.entries))
	for key, entry := range s.entries {
		snapshot[key] = entry
	}
	return snapshot
}

type predictiveFilesystemStore struct {
	baseDir string
}

func newPredictiveFilesystemStore(baseDir string) *predictiveFilesystemStore {
	return &predictiveFilesystemStore{baseDir: baseDir}
}

func (s *predictiveFilesystemStore) Reset() error {
	if err := os.RemoveAll(s.baseDir); err != nil {
		return err
	}
	return os.MkdirAll(s.baseDir, 0o755)
}

func (s *predictiveFilesystemStore) Put(_ context.Context, key string, body []byte) error {
	targetPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, body, 0o644)
}

func (s *predictiveFilesystemStore) Get(_ context.Context, key string) ([]byte, error) {
	targetPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	body, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read predictive blob %s: %w", key, err)
	}
	return body, nil
}

func (s *predictiveFilesystemStore) Delete(key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	targetPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type predictiveStartupCache struct {
	service *mediacache.Service
	meta    *predictiveMetadataStore
	blobs   *predictiveFilesystemStore

	mu      sync.Mutex
	windows map[uint]map[string]struct{}
}

func newPredictiveStartupCache(baseDir string, fetcher mediacache.Fetcher, headBytes, tailBytes int64, timeout time.Duration) *predictiveStartupCache {
	meta := newPredictiveMetadataStore()
	blobs := newPredictiveFilesystemStore(baseDir)
	_ = blobs.Reset()

	return &predictiveStartupCache{
		service: mediacache.New(meta, blobs, fetcher, headBytes, tailBytes, timeout),
		meta:    meta,
		blobs:   blobs,
		windows: map[uint]map[string]struct{}{},
	}
}

func (c *predictiveStartupCache) TryServeRange(ctx context.Context, episodeID, sourceURL, rangeHeader string) (*mediacache.CachedResponse, bool, error) {
	if c == nil || c.service == nil {
		return nil, false, nil
	}
	return c.service.TryServeRange(ctx, episodeID, sourceURL, rangeHeader)
}

func (c *predictiveStartupCache) UpdateWindow(ctx context.Context, clientID uint, episodes []predictiveEpisode) error {
	if c == nil || c.service == nil {
		return nil
	}

	normalized := make([]predictiveEpisode, 0, len(episodes))
	activeSet := make(map[string]struct{}, len(episodes))
	for _, episode := range episodes {
		episodeID := strings.TrimSpace(episode.EpisodeID)
		if episodeID == "" || strings.TrimSpace(episode.Candidate.URL) == "" {
			continue
		}
		if _, exists := activeSet[episodeID]; exists {
			continue
		}
		activeSet[episodeID] = struct{}{}
		normalized = append(normalized, predictiveEpisode{
			EpisodeID: episodeID,
			Candidate: episode.Candidate,
		})
	}

	c.mu.Lock()
	if len(activeSet) == 0 {
		delete(c.windows, clientID)
	} else {
		c.windows[clientID] = activeSet
	}
	c.mu.Unlock()

	var firstErr error
	var errMu sync.Mutex
	var wg sync.WaitGroup
	for _, episode := range normalized {
		episode := episode
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.service.Prewarm(ctx, episode.EpisodeID, episode.Candidate); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()

	c.pruneInactive()
	return firstErr
}

func (c *predictiveStartupCache) pruneInactive() {
	c.mu.Lock()
	activeEpisodes := make(map[string]struct{})
	for _, window := range c.windows {
		for episodeID := range window {
			activeEpisodes[episodeID] = struct{}{}
		}
	}
	c.mu.Unlock()

	for episodeID := range c.meta.Snapshot() {
		if _, ok := activeEpisodes[episodeID]; ok {
			continue
		}
		entry, deleted := c.meta.DeleteStartupMediaCache(episodeID)
		if !deleted || entry == nil {
			continue
		}
		_ = c.blobs.Delete(entry.HeadKey)
		_ = c.blobs.Delete(entry.TailKey)
	}
}
