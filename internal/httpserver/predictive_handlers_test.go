package httpserver

import (
	"testing"
	"time"

	"anime/develop/backend/internal/cache"
)

func TestNormalizePredictiveEpisodeIDs(t *testing.T) {
	t.Parallel()

	got := normalizePredictiveEpisodeIDs([]string{
		"  ep-2  ",
		"",
		"ep-2",
		"ep-3",
		"ep-4",
	}, 2)

	want := []string{"ep-2", "ep-3"}
	if len(got) != len(want) {
		t.Fatalf("len(normalized) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalized[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShouldQueuePredictiveWindowDedupesRecentRequests(t *testing.T) {
	t.Parallel()

	mem := cache.New(1 * time.Minute)
	clientID := uint(42)
	episodeIDs := []string{"ep-2", "ep-3"}

	if !shouldQueuePredictiveWindow(mem, clientID, episodeIDs, 30*time.Second) {
		t.Fatalf("first request should be queued")
	}

	if shouldQueuePredictiveWindow(mem, clientID, episodeIDs, 30*time.Second) {
		t.Fatalf("duplicate request should be skipped")
	}
}

func TestShouldQueuePredictiveWindowAllowsDifferentWindows(t *testing.T) {
	t.Parallel()

	mem := cache.New(1 * time.Minute)
	clientID := uint(42)

	if !shouldQueuePredictiveWindow(mem, clientID, []string{"ep-2", "ep-3"}, 30*time.Second) {
		t.Fatalf("first request should be queued")
	}

	if !shouldQueuePredictiveWindow(mem, clientID, []string{"ep-3", "ep-4"}, 30*time.Second) {
		t.Fatalf("different episode window should still be queued")
	}
}
