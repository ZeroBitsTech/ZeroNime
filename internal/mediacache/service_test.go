package mediacache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"anime/develop/backend/internal/domain"
)

type testMetadataStore struct {
	entry *domain.StartupMediaCache
}

func (s *testMetadataStore) GetStartupMediaCache(episodeID string) (*domain.StartupMediaCache, bool, error) {
	if s.entry == nil || s.entry.EpisodeID != episodeID {
		return nil, false, nil
	}
	copyEntry := *s.entry
	return &copyEntry, true, nil
}

func (s *testMetadataStore) UpsertStartupMediaCache(entry domain.StartupMediaCache) error {
	copyEntry := entry
	s.entry = &copyEntry
	return nil
}

type testBlobStore struct {
	items map[string][]byte
}

func newTestBlobStore() *testBlobStore {
	return &testBlobStore{items: map[string][]byte{}}
}

func (s *testBlobStore) Put(_ context.Context, key string, body []byte) error {
	s.items[key] = append([]byte(nil), body...)
	return nil
}

func (s *testBlobStore) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := s.items[key]
	if !ok {
		return nil, fmt.Errorf("missing blob %s", key)
	}
	return append([]byte(nil), value...), nil
}

type testFetcher struct {
	content []byte
	calls   []string
}

func (f *testFetcher) Fetch(_ context.Context, rawURL, rangeHeader string) (*http.Response, error) {
	f.calls = append(f.calls, rangeHeader)
	body, contentRange, err := sliceContent(f.content, rangeHeader)
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
	resp.Request = &http.Request{Method: http.MethodGet}
	_ = rawURL
	return resp, nil
}

func TestServicePrewarmAndServeHeadRange(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	service := New(
		&testMetadataStore{},
		newTestBlobStore(),
		&testFetcher{content: source},
		10,
		4,
		0,
	)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-1", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}

	cached, ok, err := service.TryServeRange(context.Background(), "episode-1", candidate.URL, "bytes=0-4")
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if !ok {
		t.Fatalf("TryServeRange() cache miss")
	}
	if cached.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", cached.StatusCode, http.StatusPartialContent)
	}
	if string(cached.Body) != "01234" {
		t.Fatalf("body = %q, want %q", string(cached.Body), "01234")
	}
	if got := cached.Headers.Get("Content-Range"); got != "bytes 0-4/36" {
		t.Fatalf("content-range = %q", got)
	}
}

func TestServicePrewarmAndServeTailRange(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	service := New(
		&testMetadataStore{},
		newTestBlobStore(),
		&testFetcher{content: source},
		10,
		4,
		0,
	)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-1", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}

	cached, ok, err := service.TryServeRange(context.Background(), "episode-1", candidate.URL, "bytes=32-35")
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if !ok {
		t.Fatalf("TryServeRange() cache miss")
	}
	if string(cached.Body) != "wxyz" {
		t.Fatalf("body = %q, want %q", string(cached.Body), "wxyz")
	}
	if got := cached.Headers.Get("Content-Range"); got != "bytes 32-35/36" {
		t.Fatalf("content-range = %q", got)
	}
}

func TestServiceIgnoresMismatchedSourceURL(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	service := New(
		&testMetadataStore{},
		newTestBlobStore(),
		&testFetcher{content: source},
		10,
		4,
		0,
	)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-1", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}

	_, ok, err := service.TryServeRange(context.Background(), "episode-1", "https://example.com/other.mp4", "bytes=0-4")
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if ok {
		t.Fatalf("TryServeRange() unexpectedly hit cache")
	}
}

func TestServiceMatchesSameFileWithDifferentSignedURL(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	service := New(
		&testMetadataStore{},
		newTestBlobStore(),
		&testFetcher{content: source},
		10,
		4,
		0,
	)
	candidate := domain.StreamCandidate{
		URL:       "https://anisphia.my.id/kdrive/p5ZsBKdUu5G/Kuramanime-JJKS_S2_BD-03-720p-Kuso.mp4?lud=111&pid=1&sid=2",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-3", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}

	cached, ok, err := service.TryServeRange(
		context.Background(),
		"episode-3",
		"https://asuna.my.id/kdrive/other/Kuramanime-JJKS_S2_BD-03-720p-Kuso.mp4?lud=999&pid=9&sid=9",
		"bytes=0-4",
	)
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if !ok {
		t.Fatalf("TryServeRange() cache miss for same filename")
	}
	if string(cached.Body) != "01234" {
		t.Fatalf("body = %q, want %q", string(cached.Body), "01234")
	}
}

func TestServiceServesOpenEndedRangeFromHeadCache(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	service := New(
		&testMetadataStore{},
		newTestBlobStore(),
		&testFetcher{content: source},
		10,
		4,
		0,
	)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4?token=one",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-open", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}

	cached, ok, err := service.TryServeRange(context.Background(), "episode-open", "https://example.com/video.mp4?token=two", "bytes=0-")
	if err != nil {
		t.Fatalf("TryServeRange() error = %v", err)
	}
	if !ok {
		t.Fatalf("TryServeRange() cache miss")
	}
	if string(cached.Body) != "0123456789" {
		t.Fatalf("body = %q, want %q", string(cached.Body), "0123456789")
	}
	if got := cached.Headers.Get("Content-Range"); got != "bytes 0-9/36" {
		t.Fatalf("content-range = %q", got)
	}
}

func TestServicePrewarmSkipsWhenExistingCacheIsSufficient(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	store := &testMetadataStore{
		entry: &domain.StartupMediaCache{
			EpisodeID:     "episode-skip",
			SourceURL:     "https://example.com/video.mp4?token=old",
			SourceKey:     sourceKey("https://example.com/video.mp4?token=new"),
			Container:     "mp4",
			Quality:       "720p",
			ContentType:   "video/mp4",
			ContentLength: int64(len(source)),
			HeadKey:       "head",
			HeadBytes:     10,
			TailKey:       "tail",
			TailBytes:     4,
		},
	}
	blobs := newTestBlobStore()
	blobs.items["head"] = append([]byte(nil), source[:10]...)
	blobs.items["tail"] = append([]byte(nil), source[len(source)-4:]...)
	fetcher := &testFetcher{content: source}
	service := New(store, blobs, fetcher, 10, 4, 0)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4?token=new",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-skip", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}
	if len(fetcher.calls) != 0 {
		t.Fatalf("fetcher called %d times, want 0", len(fetcher.calls))
	}
}

func TestServicePrewarmRefetchesWhenMetadataExistsButBlobMissing(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	store := &testMetadataStore{
		entry: &domain.StartupMediaCache{
			EpisodeID:     "episode-refetch",
			SourceURL:     "https://example.com/video.mp4?token=old",
			SourceKey:     sourceKey("https://example.com/video.mp4?token=new"),
			Container:     "mp4",
			Quality:       "720p",
			ContentType:   "video/mp4",
			ContentLength: int64(len(source)),
			HeadKey:       "missing-head",
			HeadBytes:     10,
			TailKey:       "missing-tail",
			TailBytes:     4,
		},
	}
	fetcher := &testFetcher{content: source}
	service := New(store, newTestBlobStore(), fetcher, 10, 4, 0)
	candidate := domain.StreamCandidate{
		URL:       "https://example.com/video.mp4?token=new",
		Container: "mp4",
		Quality:   "720p",
		IsDirect:  true,
		Playable:  true,
	}

	if err := service.Prewarm(context.Background(), "episode-refetch", candidate); err != nil {
		t.Fatalf("Prewarm() error = %v", err)
	}
	if len(fetcher.calls) == 0 {
		t.Fatalf("fetcher was not called")
	}
}

func sliceContent(source []byte, rangeHeader string) ([]byte, string, error) {
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
		length := mustInt(parts[1])
		start = len(source) - length
		end = len(source) - 1
	case parts[1] == "":
		start = mustInt(parts[0])
		end = len(source) - 1
	default:
		start = mustInt(parts[0])
		end = mustInt(parts[1])
	}

	if start < 0 || end >= len(source) || start > end {
		return nil, "", fmt.Errorf("out of bounds %q", rangeHeader)
	}
	body := append([]byte(nil), source[start:end+1]...)
	return body, fmt.Sprintf("bytes %d-%d/%d", start, end, len(source)), nil
}

func mustInt(raw string) int {
	var value int
	for _, char := range raw {
		value = value*10 + int(char-'0')
	}
	return value
}
