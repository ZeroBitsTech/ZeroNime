package httpserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"anime/develop/backend/internal/cache"
)

func TestCoalescingRangeKeyAcceptsSmallExplicitRange(t *testing.T) {
	t.Parallel()

	key, ok := coalescingRangeKey("https://example.com/video.mp4", "bytes=0-1048575", startupCoalesceMaxBytes)
	if !ok {
		t.Fatalf("coalescingRangeKey() ok = false, want true")
	}
	if key == "" {
		t.Fatalf("coalescingRangeKey() key = empty, want non-empty")
	}
}

func TestCoalescingRangeKeyRejectsOpenEndedRange(t *testing.T) {
	t.Parallel()

	if _, ok := coalescingRangeKey("https://example.com/video.mp4", "bytes=0-", startupCoalesceMaxBytes); ok {
		t.Fatalf("coalescingRangeKey() ok = true, want false")
	}
}

func TestMediaFetcherCoalescesConcurrentSmallRanges(t *testing.T) {
	t.Parallel()

	memCache := cache.New(time.Minute)
	fetcher := &mediaFetcher{
		cache:            memCache,
		bufferTTL:        startupCoalesceTTL,
		maxBufferedBytes: startupCoalesceMaxBytes,
		slowThreshold:    startupSlowFetchThreshold,
	}

	var upstreamCalls atomic.Int32
	fetcher.fetch = func(_ context.Context, _ string, _ string) (*http.Response, error) {
		upstreamCalls.Add(1)
		time.Sleep(80 * time.Millisecond)
		body := strings.Repeat("a", 1024)
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header: http.Header{
				"Content-Length": {"1024"},
				"Content-Range":  {"bytes 0-1023/2048"},
			},
			ContentLength: 1024,
			Body:          io.NopCloser(strings.NewReader(body)),
		}, nil
	}

	const workers = 2
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			buffered, live, _, err := fetcher.Fetch(context.Background(), "https://example.com/video.mp4", "bytes=0-1023")
			if err != nil {
				t.Errorf("Fetch() error = %v", err)
				return
			}
			if buffered == nil {
				t.Errorf("Fetch() buffered = nil, want non-nil")
			}
			if live != nil {
				t.Errorf("Fetch() live response = non-nil, want nil")
			}
		}()
	}
	wg.Wait()

	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream call count = %d, want 1", got)
	}
}

func TestMediaFetcherCachesSmallRangeBriefly(t *testing.T) {
	t.Parallel()

	memCache := cache.New(time.Minute)
	fetcher := &mediaFetcher{
		cache:            memCache,
		bufferTTL:        startupCoalesceTTL,
		maxBufferedBytes: startupCoalesceMaxBytes,
		slowThreshold:    startupSlowFetchThreshold,
	}

	var upstreamCalls atomic.Int32
	fetcher.fetch = func(_ context.Context, _ string, _ string) (*http.Response, error) {
		upstreamCalls.Add(1)
		body := strings.Repeat("b", 512)
		return &http.Response{
			StatusCode:    http.StatusPartialContent,
			Header:        http.Header{"Content-Length": {"512"}},
			ContentLength: 512,
			Body:          io.NopCloser(strings.NewReader(body)),
		}, nil
	}

	for range 2 {
		buffered, live, _, err := fetcher.Fetch(context.Background(), "https://example.com/video.mp4", "bytes=0-511")
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if buffered == nil || live != nil {
			t.Fatalf("Fetch() buffered/live mismatch: buffered=%v live=%v", buffered != nil, live != nil)
		}
	}

	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream call count = %d, want 1", got)
	}
}

func TestMediaFetcherShouldRefreshAfterSlowStartupFetch(t *testing.T) {
	t.Parallel()

	fetcher := &mediaFetcher{slowThreshold: startupSlowFetchThreshold}
	if !fetcher.ShouldRefreshAfterFetch("bytes=0-1023", startupSlowFetchThreshold) {
		t.Fatalf("ShouldRefreshAfterFetch() = false, want true")
	}
	if fetcher.ShouldRefreshAfterFetch("bytes=1048576-2097151", startupSlowFetchThreshold) {
		t.Fatalf("ShouldRefreshAfterFetch() for non-startup range = true, want false")
	}
}
