package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"anime/develop/backend/internal/domain"
)

func TestSelectEpisodesLatest(t *testing.T) {
	t.Parallel()

	episodes := []domain.Episode{
		{ID: "ep-7", Number: 7},
		{ID: "ep-10", Number: 10},
		{ID: "ep-8", Number: 8},
		{ID: "ep-9", Number: 9},
	}

	selected := selectEpisodes(episodes, 2)
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if selected[0].ID != "ep-10" || selected[1].ID != "ep-9" {
		t.Fatalf("selected ids = %q, %q", selected[0].ID, selected[1].ID)
	}
}

func TestSelectEpisodesReturnsAllWhenLatestIsZero(t *testing.T) {
	t.Parallel()

	episodes := []domain.Episode{
		{ID: "ep-3", Number: 3},
		{ID: "ep-2", Number: 2},
	}

	selected := selectEpisodes(episodes, 0)
	if len(selected) != len(episodes) {
		t.Fatalf("len(selected) = %d, want %d", len(selected), len(episodes))
	}
}

func TestSelectEpisodesCapsAtTwentyFive(t *testing.T) {
	t.Parallel()

	episodes := make([]domain.Episode, 0, 30)
	for i := 1; i <= 30; i++ {
		episodes = append(episodes, domain.Episode{
			ID:     fmt.Sprintf("ep-%d", i),
			Number: i,
		})
	}

	selected := selectEpisodes(episodes, 25)
	if len(selected) != 25 {
		t.Fatalf("len(selected) = %d, want 25", len(selected))
	}
	if selected[0].Number != 30 || selected[24].Number != 6 {
		t.Fatalf("selected number range = %d..%d, want 30..6", selected[0].Number, selected[24].Number)
	}
}

func TestPrewarmEpisodesUsesBoundedConcurrency(t *testing.T) {
	t.Parallel()

	episodes := []domain.Episode{
		{ID: "ep-1", Number: 1},
		{ID: "ep-2", Number: 2},
		{ID: "ep-3", Number: 3},
		{ID: "ep-4", Number: 4},
	}

	var active atomic.Int32
	var maxActive atomic.Int32

	success, skipped := prewarmEpisodes(context.Background(), episodes, 2, func(_ context.Context, _ domain.Episode) error {
		current := active.Add(1)
		for {
			observed := maxActive.Load()
			if current <= observed || maxActive.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return nil
	})

	if success != len(episodes) || skipped != 0 {
		t.Fatalf("success=%d skipped=%d, want success=%d skipped=0", success, skipped, len(episodes))
	}
	if got := maxActive.Load(); got < 2 {
		t.Fatalf("max concurrency = %d, want at least 2", got)
	}
	if got := maxActive.Load(); got > 2 {
		t.Fatalf("max concurrency = %d, want at most 2", got)
	}
}
