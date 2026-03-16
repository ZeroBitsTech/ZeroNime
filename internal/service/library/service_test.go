package library

import (
	"testing"
	"time"

	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/store/postgres"
)

func TestBuildContinueWatchingItemsFallsBackToCatalogMeta(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	history := []postgres.WatchHistory{
		{
			CatalogID:       "kuramanime:4205/digimon-beatbreak",
			EpisodeID:       "kuramanime:4205/digimon-beatbreak/episode/22",
			PositionSeconds: 245,
			UpdatedAt:       now,
		},
	}

	items := buildContinueWatchingItems(history, nil, map[string]domain.AnimeDetail{
		"kuramanime:4205/digimon-beatbreak": {
			Title:      "Digimon Beatbreak",
			CoverImage: "https://r2.nyomo.my.id/images/digimon.jpg",
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one continue-watching item, got %d", len(items))
	}
	if items[0].Title != "Digimon Beatbreak" {
		t.Fatalf("expected fallback title, got %q", items[0].Title)
	}
	if items[0].CoverImage != "https://r2.nyomo.my.id/images/digimon.jpg" {
		t.Fatalf("expected fallback cover, got %q", items[0].CoverImage)
	}
}

func TestBuildContinueWatchingItemsKeepsWatchlistMetaWhenAvailable(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	history := []postgres.WatchHistory{
		{
			CatalogID:       "otakudesu:dgbb-sub-indo",
			EpisodeID:       "otakudesu:dgbb-episode-22-sub-indo",
			PositionSeconds: 31,
			UpdatedAt:       now,
		},
	}
	watchlist := []postgres.Watchlist{
		{
			CatalogID:  "otakudesu:dgbb-sub-indo",
			Title:      "Digimon Beatbreak",
			CoverImage: "https://otakudesu.blog/wp-content/uploads/digimon.jpg",
		},
	}

	items := buildContinueWatchingItems(history, watchlist, map[string]domain.AnimeDetail{
		"otakudesu:dgbb-sub-indo": {
			Title:      "Wrong Title",
			CoverImage: "https://example.com/wrong.jpg",
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one continue-watching item, got %d", len(items))
	}
	if items[0].Title != "Digimon Beatbreak" {
		t.Fatalf("expected watchlist title to win, got %q", items[0].Title)
	}
	if items[0].CoverImage != "https://otakudesu.blog/wp-content/uploads/digimon.jpg" {
		t.Fatalf("expected watchlist cover to win, got %q", items[0].CoverImage)
	}
}

func TestBuildContinueWatchingItemsPrefersHistoryMetadataWhenPresent(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	history := []postgres.WatchHistory{
		{
			CatalogID:       "kuramanime:4205/digimon-beatbreak",
			EpisodeID:       "kuramanime:4205/digimon-beatbreak/episode/22",
			Title:           "Digimon Beatbreak",
			CoverImage:      "https://r2.nyomo.my.id/images/digimon-history.jpg",
			PositionSeconds: 321,
			UpdatedAt:       now,
		},
	}

	items := buildContinueWatchingItems(history, []postgres.Watchlist{
		{
			CatalogID:  "kuramanime:4205/digimon-beatbreak",
			Title:      "Wrong Watchlist Title",
			CoverImage: "https://example.com/wrong-watchlist.jpg",
		},
	}, map[string]domain.AnimeDetail{
		"kuramanime:4205/digimon-beatbreak": {
			Title:      "Wrong Fallback Title",
			CoverImage: "https://example.com/wrong-fallback.jpg",
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one continue-watching item, got %d", len(items))
	}
	if items[0].Title != "Digimon Beatbreak" {
		t.Fatalf("expected history title to win, got %q", items[0].Title)
	}
	if items[0].CoverImage != "https://r2.nyomo.my.id/images/digimon-history.jpg" {
		t.Fatalf("expected history cover to win, got %q", items[0].CoverImage)
	}
}
