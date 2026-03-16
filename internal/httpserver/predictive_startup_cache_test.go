package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"anime/develop/backend/internal/domain"
)

type predictiveTestFetcher struct {
	contentByURL map[string][]byte
	calls        []string
	sleep        time.Duration
	inFlight     atomic.Int32
	maxInFlight  atomic.Int32
}

func (f *predictiveTestFetcher) Fetch(_ context.Context, rawURL, rangeHeader string) (*http.Response, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s|%s", rawURL, rangeHeader))
	current := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		seen := f.maxInFlight.Load()
		if current <= seen || f.maxInFlight.CompareAndSwap(seen, current) {
			break
		}
	}
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
	source, ok := f.contentByURL[rawURL]
	if !ok {
		return nil, fmt.Errorf("missing source for %s", rawURL)
	}
	body, contentRange, err := predictiveSliceContent(source, rangeHeader)
	if err != nil {
		return nil, err
	}
	resp := &http.Response{
		StatusCode: http.StatusPartialContent,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	resp.Header.Set("Content-Type", "video/mp4")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	resp.Header.Set("Content-Range", contentRange)
	resp.Header.Set("Accept-Ranges", "bytes")
	return resp, nil
}

func TestPredictiveStartupCacheServesNextEpisodeAndEvictsOldWindow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fetcher := &predictiveTestFetcher{
		contentByURL: map[string][]byte{
			"https://example.com/ep2.mp4": []byte("0123456789abcdefghijklmnopqrstuvwxyz"),
			"https://example.com/ep3.mp4": []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"),
			"https://example.com/ep4.mp4": []byte("ep4-content-abcdefghijklmnopqrstuvwxyz"),
		},
	}
	cache := newPredictiveStartupCache(dir, fetcher, 10, 4, 0)

	err := cache.UpdateWindow(context.Background(), 7, []predictiveEpisode{
		{
			EpisodeID: "episode-2",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep2.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
		{
			EpisodeID: "episode-3",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep3.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWindow() error = %v", err)
	}

	cached, ok, err := cache.TryServeRange(context.Background(), "episode-2", "https://example.com/ep2.mp4", "bytes=0-4")
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if !ok {
		t.Fatalf("TryServeRange() cache miss for episode-2")
	}
	if string(cached.Body) != "01234" {
		t.Fatalf("body = %q, want %q", string(cached.Body), "01234")
	}

	err = cache.UpdateWindow(context.Background(), 7, []predictiveEpisode{
		{
			EpisodeID: "episode-3",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep3.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
		{
			EpisodeID: "episode-4",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep4.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWindow() second error = %v", err)
	}

	if _, ok, err := cache.TryServeRange(context.Background(), "episode-2", "https://example.com/ep2.mp4", "bytes=0-4"); err != nil {
		t.Fatalf("TryServeRange() second error = %v", err)
	} else if ok {
		t.Fatalf("episode-2 should have been evicted")
	}

	if _, ok, err := cache.TryServeRange(context.Background(), "episode-3", "https://example.com/ep3.mp4", "bytes=0-4"); err != nil {
		t.Fatalf("TryServeRange() third error = %v", err)
	} else if !ok {
		t.Fatalf("episode-3 should still be cached")
	}
}

func TestPredictiveStartupCachePrewarmsEpisodesConcurrently(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fetcher := &predictiveTestFetcher{
		contentByURL: map[string][]byte{
			"https://example.com/ep2.mp4": []byte("0123456789abcdefghijklmnopqrstuvwxyz"),
			"https://example.com/ep3.mp4": []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"),
		},
		sleep: 40 * time.Millisecond,
	}
	cache := newPredictiveStartupCache(dir, fetcher, 10, 4, 0)

	err := cache.UpdateWindow(context.Background(), 7, []predictiveEpisode{
		{
			EpisodeID: "episode-2",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep2.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
		{
			EpisodeID: "episode-3",
			Candidate: domain.StreamCandidate{URL: "https://example.com/ep3.mp4", Container: "mp4", Quality: "720p", IsDirect: true, Playable: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWindow() error = %v", err)
	}

	if got := fetcher.maxInFlight.Load(); got < 2 {
		t.Fatalf("max concurrent fetches = %d, want at least 2", got)
	}
}

func predictiveSliceContent(source []byte, rangeHeader string) ([]byte, string, error) {
	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return nil, "", fmt.Errorf("unsupported range %q", rangeHeader)
	}
	spec := strings.TrimPrefix(rangeHeader, prefix)
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid range %q", rangeHeader)
	}

	var start, end int
	switch {
	case parts[0] == "":
		length := predictiveMustInt(parts[1])
		start = len(source) - length
		end = len(source) - 1
	case parts[1] == "":
		start = predictiveMustInt(parts[0])
		end = len(source) - 1
	default:
		start = predictiveMustInt(parts[0])
		end = predictiveMustInt(parts[1])
	}

	if start < 0 || end >= len(source) || start > end {
		return nil, "", fmt.Errorf("out of bounds %q", rangeHeader)
	}
	body := append([]byte(nil), source[start:end+1]...)
	return body, fmt.Sprintf("bytes %d-%d/%d", start, end, len(source)), nil
}

func predictiveMustInt(raw string) int {
	var value int
	for _, char := range raw {
		value = value*10 + int(char-'0')
	}
	return value
}
