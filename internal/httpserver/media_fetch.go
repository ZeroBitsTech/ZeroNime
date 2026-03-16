package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/mediaproxy"

	"golang.org/x/sync/singleflight"
)

const (
	startupCoalesceMaxBytes   int64         = 2 * 1024 * 1024
	startupCoalesceTTL        time.Duration = 4 * time.Second
	startupSlowFetchThreshold time.Duration = 900 * time.Millisecond
	startupRefreshThrottleTTL time.Duration = 5 * time.Second
)

var errResponseNotBufferable = errors.New("response not bufferable")

type bufferedMediaResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type mediaFetcher struct {
	cache            *cache.Cache
	group            singleflight.Group
	fetch            func(context.Context, string, string) (*http.Response, error)
	bufferTTL        time.Duration
	maxBufferedBytes int64
	slowThreshold    time.Duration
}

func newMediaFetcher(memCache *cache.Cache, proxy *mediaproxy.Proxy) *mediaFetcher {
	return &mediaFetcher{
		cache:            memCache,
		fetch:            proxy.Fetch,
		bufferTTL:        startupCoalesceTTL,
		maxBufferedBytes: startupCoalesceMaxBytes,
		slowThreshold:    startupSlowFetchThreshold,
	}
}

func (f *mediaFetcher) Fetch(ctx context.Context, rawURL, rangeHeader string) (*bufferedMediaResponse, *http.Response, time.Duration, error) {
	startedAt := time.Now()

	key, ok := coalescingRangeKey(rawURL, rangeHeader, f.maxBufferedBytes)
	if !ok {
		resp, err := f.fetch(ctx, rawURL, rangeHeader)
		return nil, resp, time.Since(startedAt), err
	}

	if cached, ok := f.cachedBuffered(key); ok {
		return cached, nil, time.Since(startedAt), nil
	}

	value, err, _ := f.group.Do(key, func() (any, error) {
		if cached, ok := f.cachedBuffered(key); ok {
			return cached, nil
		}

		resp, err := f.fetch(ctx, rawURL, rangeHeader)
		if err != nil {
			return nil, err
		}

		buffered, readErr := readBufferedResponse(resp, f.maxBufferedBytes)
		if readErr != nil {
			return nil, readErr
		}

		f.cache.Set(key, buffered, f.bufferTTL)
		return buffered, nil
	})
	if err == nil {
		buffered, _ := value.(*bufferedMediaResponse)
		return cloneBufferedResponse(buffered), nil, time.Since(startedAt), nil
	}
	if !errors.Is(err, errResponseNotBufferable) {
		return nil, nil, time.Since(startedAt), err
	}

	resp, directErr := f.fetch(ctx, rawURL, rangeHeader)
	return nil, resp, time.Since(startedAt), directErr
}

func (f *mediaFetcher) ShouldRefreshAfterFetch(rangeHeader string, fetchDuration time.Duration) bool {
	return isStartupRange(rangeHeader) && fetchDuration >= f.slowThreshold
}

func (f *mediaFetcher) cachedBuffered(key string) (*bufferedMediaResponse, bool) {
	raw, ok := f.cache.Get(key)
	if !ok {
		return nil, false
	}
	buffered, ok := raw.(*bufferedMediaResponse)
	if !ok || buffered == nil {
		return nil, false
	}
	return cloneBufferedResponse(buffered), true
}

func coalescingRangeKey(rawURL, rangeHeader string, maxBytes int64) (string, bool) {
	start, end, ok := explicitRange(rangeHeader)
	if !ok {
		return "", false
	}
	if start < 0 || end < start {
		return "", false
	}
	if end-start+1 > maxBytes {
		return "", false
	}
	return fmt.Sprintf("media-buffer:%s|%s", rawURL, strings.TrimSpace(rangeHeader)), true
}

func explicitRange(rangeHeader string) (int64, int64, bool) {
	rangeHeader = strings.TrimSpace(rangeHeader)
	if !strings.HasPrefix(strings.ToLower(rangeHeader), "bytes=") {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimSpace(rangeHeader[6:]), ",", 2)
	if len(parts) == 0 {
		return 0, 0, false
	}
	bounds := strings.SplitN(strings.TrimSpace(parts[0]), "-", 2)
	if len(bounds) != 2 || strings.TrimSpace(bounds[0]) == "" || strings.TrimSpace(bounds[1]) == "" {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(bounds[0]), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.ParseInt(strings.TrimSpace(bounds[1]), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return start, end, true
}

func isStartupRange(rangeHeader string) bool {
	rangeHeader = strings.TrimSpace(strings.ToLower(rangeHeader))
	if rangeHeader == "" {
		return true
	}
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return false
	}
	rangeHeader = strings.TrimSpace(rangeHeader[6:])
	parts := strings.SplitN(rangeHeader, ",", 2)
	if len(parts) == 0 {
		return false
	}
	bounds := strings.SplitN(strings.TrimSpace(parts[0]), "-", 2)
	if len(bounds) != 2 {
		return false
	}
	return strings.TrimSpace(bounds[0]) == "0"
}

func readBufferedResponse(resp *http.Response, maxBytes int64) (*bufferedMediaResponse, error) {
	if resp == nil {
		return nil, errResponseNotBufferable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, errResponseNotBufferable
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 || contentLength > maxBytes {
		return nil, errResponseNotBufferable
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errResponseNotBufferable
	}

	return &bufferedMediaResponse{
		StatusCode: resp.StatusCode,
		Headers:    cloneHeader(resp.Header),
		Body:       append([]byte(nil), body...),
	}, nil
}

func cloneBufferedResponse(value *bufferedMediaResponse) *bufferedMediaResponse {
	if value == nil {
		return nil
	}
	return &bufferedMediaResponse{
		StatusCode: value.StatusCode,
		Headers:    cloneHeader(value.Headers),
		Body:       append([]byte(nil), value.Body...),
	}
}

func cloneHeader(header http.Header) http.Header {
	cloned := make(http.Header, len(header))
	for key, values := range header {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}
