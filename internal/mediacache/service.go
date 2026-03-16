package mediacache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"
)

type MetadataStore interface {
	GetStartupMediaCache(episodeID string) (*domain.StartupMediaCache, bool, error)
	UpsertStartupMediaCache(entry domain.StartupMediaCache) error
}

type BlobStore interface {
	Put(ctx context.Context, key string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

type Fetcher interface {
	Fetch(ctx context.Context, rawURL, rangeHeader string) (*http.Response, error)
}

type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type Service struct {
	store     MetadataStore
	blobs     BlobStore
	fetcher   Fetcher
	headBytes int64
	tailBytes int64
	timeout   time.Duration
}

func New(store MetadataStore, blobs BlobStore, fetcher Fetcher, headBytes, tailBytes int64, timeout time.Duration) *Service {
	return &Service{
		store:     store,
		blobs:     blobs,
		fetcher:   fetcher,
		headBytes: headBytes,
		tailBytes: tailBytes,
		timeout:   timeout,
	}
}

func (s *Service) Prewarm(ctx context.Context, episodeID string, candidate domain.StreamCandidate) error {
	if s == nil || s.store == nil || s.blobs == nil || s.fetcher == nil {
		return nil
	}
	if strings.TrimSpace(episodeID) == "" || strings.TrimSpace(candidate.URL) == "" {
		return fmt.Errorf("episode id and candidate url are required")
	}
	if s.isSufficientlyCached(ctx, episodeID, candidate.URL) {
		return nil
	}

	headEnd := max(0, s.headBytes-1)
	headCtx, headCancel := withOptionalTimeout(ctx, s.timeout)
	defer headCancel()
	headResp, err := s.fetcher.Fetch(headCtx, candidate.URL, fmt.Sprintf("bytes=0-%d", headEnd))
	if err != nil {
		return err
	}
	headBody, err := readAllAndClose(headResp)
	if err != nil {
		return err
	}
	contentLength := totalLengthFromResponse(headResp)
	if contentLength <= 0 {
		contentLength = int64(len(headBody))
	}
	contentType := strings.TrimSpace(headResp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "video/mp4"
	}

	headKey := blobKey(episodeID, candidate.URL, "head")
	if err := s.blobs.Put(ctx, headKey, headBody); err != nil {
		return err
	}

	entry := domain.StartupMediaCache{
		EpisodeID:     episodeID,
		SourceURL:     candidate.URL,
		SourceKey:     sourceKey(candidate.URL),
		Container:     candidate.Container,
		Quality:       candidate.Quality,
		ContentType:   contentType,
		ContentLength: contentLength,
		HeadKey:       headKey,
		HeadBytes:     int64(len(headBody)),
	}

	tailStart := max(0, contentLength-s.tailBytes)
	if s.tailBytes > 0 && contentLength > 0 && tailStart > 0 && tailStart >= entry.HeadBytes {
		tailCtx, tailCancel := withOptionalTimeout(ctx, s.timeout)
		defer tailCancel()
		tailResp, tailErr := s.fetcher.Fetch(tailCtx, candidate.URL, fmt.Sprintf("bytes=%d-%d", tailStart, contentLength-1))
		if tailErr == nil {
			tailBody, readErr := readAllAndClose(tailResp)
			if readErr == nil && len(tailBody) > 0 {
				tailKey := blobKey(episodeID, candidate.URL, "tail")
				if putErr := s.blobs.Put(ctx, tailKey, tailBody); putErr == nil {
					entry.TailKey = tailKey
					entry.TailBytes = int64(len(tailBody))
				}
			}
		}
	}

	return s.store.UpsertStartupMediaCache(entry)
}

func (s *Service) isSufficientlyCached(ctx context.Context, episodeID, candidateURL string) bool {
	entry, ok, err := s.store.GetStartupMediaCache(episodeID)
	if err != nil || !ok || entry == nil {
		return false
	}

	currentSourceKey := sourceKey(candidateURL)
	if entry.SourceKey != "" {
		if entry.SourceKey != currentSourceKey {
			return false
		}
	} else if sourceKey(entry.SourceURL) != currentSourceKey {
		return false
	}

	if entry.HeadKey == "" || entry.HeadBytes < s.headBytes {
		return false
	}
	if _, err := s.blobs.Get(ctx, entry.HeadKey); err != nil {
		return false
	}
	if s.tailBytes > 0 {
		if entry.TailKey == "" || entry.TailBytes < s.tailBytes {
			return false
		}
		if _, err := s.blobs.Get(ctx, entry.TailKey); err != nil {
			return false
		}
	}
	return true
}

func (s *Service) TryServeRange(ctx context.Context, episodeID, sourceURL, rangeHeader string) (*CachedResponse, bool, error) {
	if s == nil || s.store == nil || s.blobs == nil {
		return nil, false, nil
	}
	if strings.TrimSpace(rangeHeader) == "" || strings.TrimSpace(episodeID) == "" {
		return nil, false, nil
	}

	entry, ok, err := s.store.GetStartupMediaCache(episodeID)
	if err != nil || !ok || entry == nil {
		return nil, false, err
	}
	if entry.ContentLength <= 0 {
		return nil, false, nil
	}
	currentSourceKey := sourceKey(sourceURL)
	if entry.SourceKey != "" {
		if entry.SourceKey != currentSourceKey {
			return nil, false, nil
		}
	} else if sourceKey(entry.SourceURL) != currentSourceKey && entry.SourceURL != sourceURL {
		return nil, false, nil
	}

	requestedStart, requestedEnd, openEnded, err := parseRange(rangeHeader, entry.ContentLength)
	if err != nil {
		return nil, false, nil
	}
	if openEnded && requestedStart >= 0 && requestedStart < entry.HeadBytes {
		requestedEnd = entry.HeadBytes - 1
	}

	if requestedStart >= 0 && requestedEnd < entry.HeadBytes {
		body, readErr := s.blobs.Get(ctx, entry.HeadKey)
		if readErr != nil {
			return nil, false, readErr
		}
		return buildCachedPartialResponse(entry, body, requestedStart, requestedEnd, 0), true, nil
	}

	tailStart := entry.ContentLength - entry.TailBytes
	if entry.TailKey != "" && entry.TailBytes > 0 && requestedStart >= tailStart && requestedEnd < entry.ContentLength {
		body, readErr := s.blobs.Get(ctx, entry.TailKey)
		if readErr != nil {
			return nil, false, readErr
		}
		return buildCachedPartialResponse(entry, body, requestedStart, requestedEnd, tailStart), true, nil
	}

	return nil, false, nil
}

func buildCachedPartialResponse(entry *domain.StartupMediaCache, blob []byte, start, end, blobBase int64) *CachedResponse {
	localStart := start - blobBase
	localEnd := end - blobBase + 1
	body := append([]byte(nil), blob[localStart:localEnd]...)
	headers := make(http.Header)
	headers.Set("Accept-Ranges", "bytes")
	headers.Set("Content-Type", entry.ContentType)
	headers.Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
	headers.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, entry.ContentLength))
	headers.Set("Cache-Control", "public, max-age=31536000")
	return &CachedResponse{
		StatusCode: http.StatusPartialContent,
		Headers:    headers,
		Body:       body,
	}
}

func parseRange(header string, total int64) (int64, int64, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(header), "bytes=") {
		return 0, 0, false, fmt.Errorf("unsupported range")
	}
	spec := strings.TrimPrefix(strings.TrimSpace(header), "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}

	switch {
	case parts[0] == "":
		length, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || length <= 0 || length > total {
			return 0, 0, false, fmt.Errorf("invalid suffix range")
		}
		return total - length, total - 1, false, nil
	case parts[1] == "":
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 || start >= total {
			return 0, 0, false, fmt.Errorf("invalid open range")
		}
		return start, total - 1, true, nil
	default:
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, false, err
		}
		end, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, false, err
		}
		if start < 0 || end < start || end >= total {
			return 0, 0, false, fmt.Errorf("invalid bounded range")
		}
		return start, end, false, nil
	}
}

func totalLengthFromResponse(resp *http.Response) int64 {
	if resp == nil {
		return 0
	}
	contentRange := strings.TrimSpace(resp.Header.Get("Content-Range"))
	if contentRange != "" {
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 {
			if total, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
				return total
			}
		}
	}
	if contentLength := resp.ContentLength; contentLength > 0 {
		return contentLength
	}
	return 0
}

func readAllAndClose(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("empty response body")
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func withOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func blobKey(episodeID, sourceURL, suffix string) string {
	sum := sha1.Sum([]byte(episodeID + "|" + sourceURL + "|" + suffix))
	return fmt.Sprintf("startup-media/%s/%s.bin", episodeID, hex.EncodeToString(sum[:]))
}

func sourceKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	filename := strings.TrimSpace(path.Base(parsed.Path))
	if filename != "" && filename != "." && filename != "/" {
		return strings.ToLower(filename)
	}
	return strings.ToLower(strings.TrimSpace(parsed.Path))
}

func max(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
