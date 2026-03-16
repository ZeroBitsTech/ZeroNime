package catalog

import (
	"context"
	"testing"
	"time"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/domain"
)

func TestInferredEpisodeCountUsesHighestRangeBound(t *testing.T) {
	items := []domain.Episode{
		{Number: 131, Label: "131", Title: "Episode 131"},
		{Number: 3, Label: "3", Title: "Episode 3"},
		{Number: 1, Label: "1-2", Title: "Episode 1-2"},
	}

	if got := inferredEpisodeCount(items); got != 131 {
		t.Fatalf("expected inferred episode count 131, got %d", got)
	}
}

type fakeProvider struct {
	catalogResult domain.AnimeDetail
	catalogByID   map[string]domain.AnimeDetail
	searchResult  []domain.AnimeCard
}

func (f fakeProvider) Name() string { return "fake" }

func (f fakeProvider) Home(context.Context) (domain.HomeFeed, error) {
	return domain.HomeFeed{}, nil
}

func (f fakeProvider) Search(context.Context, string) ([]domain.AnimeCard, error) {
	return append([]domain.AnimeCard(nil), f.searchResult...), nil
}

func (f fakeProvider) Schedule(context.Context) ([]domain.ScheduleDay, error) {
	return nil, nil
}

func (f fakeProvider) SchedulePage(context.Context, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (f fakeProvider) Index(context.Context) (map[string][]domain.AnimeCard, error) {
	return nil, nil
}

func (f fakeProvider) PropertyList(context.Context, string) (domain.PropertyList, error) {
	return domain.PropertyList{}, nil
}

func (f fakeProvider) PropertyCatalog(context.Context, string, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (f fakeProvider) QuickCatalog(context.Context, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (f fakeProvider) SeasonalPopular(context.Context) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (f fakeProvider) Catalog(_ context.Context, id string) (domain.AnimeDetail, error) {
	if len(f.catalogByID) > 0 {
		if value, ok := f.catalogByID[id]; ok {
			return value, nil
		}
	}
	return f.catalogResult, nil
}

func (f fakeProvider) Episodes(context.Context, string) ([]domain.Episode, error) {
	return nil, nil
}

func (f fakeProvider) StreamCandidates(context.Context, string) ([]domain.StreamCandidate, error) {
	return nil, nil
}

func TestCatalogHydratesRecommendationCoverFromSearch(t *testing.T) {
	svc := New(
		fakeProvider{
			catalogResult: domain.AnimeDetail{
				ID:         "kuramanime:1929/pokemon-2023",
				Title:      "Pokemon (2023)",
				CoverImage: "https://example.com/pokemon.jpg",
				Recommendations: []domain.AnimeCard{
					{
						ID:       "kuramanime:2132/houkago-no-breath",
						Title:    "Houkago no Breath",
						Provider: "kuramanime",
					},
				},
				Provider: "kuramanime",
			},
			searchResult: []domain.AnimeCard{
				{
					ID:         "kuramanime:2132/houkago-no-breath",
					Title:      "Houkago no Breath",
					CoverImage: "https://example.com/houkago.jpg",
					Provider:   "kuramanime",
				},
			},
		},
		cache.New(time.Minute),
		nil,
	)

	detail, err := svc.Catalog(context.Background(), "kuramanime:1929/pokemon-2023")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detail.Recommendations[0].CoverImage; got != "https://example.com/houkago.jpg" {
		t.Fatalf("expected hydrated recommendation cover, got %q", got)
	}
}

func TestCatalogHydratesRecommendationCoverFromCachedDetail(t *testing.T) {
	cacheStore := cache.New(time.Minute)
	cacheStore.Set("catalog:kuramanime:1929/pokemon-2023", domain.AnimeDetail{
		ID:         "kuramanime:1929/pokemon-2023",
		Title:      "Pokemon (2023)",
		CoverImage: "https://example.com/pokemon.jpg",
		Recommendations: []domain.AnimeCard{
			{
				ID:       "kuramanime:2132/houkago-no-breath",
				Title:    "Houkago no Breath",
				Provider: "kuramanime",
			},
		},
		Provider: "kuramanime",
	}, time.Minute)

	svc := New(
		fakeProvider{
			searchResult: []domain.AnimeCard{
				{
					ID:         "kuramanime:2132/houkago-no-breath",
					Title:      "Houkago no Breath",
					CoverImage: "https://example.com/houkago.jpg",
					Provider:   "kuramanime",
				},
			},
		},
		cacheStore,
		nil,
	)

	detail, err := svc.Catalog(context.Background(), "kuramanime:1929/pokemon-2023")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detail.Recommendations[0].CoverImage; got != "https://example.com/houkago.jpg" {
		t.Fatalf("expected cached detail recommendation cover to be hydrated, got %q", got)
	}
}

func TestCatalogHydratesRecommendationCoverFromRelatedCatalog(t *testing.T) {
	svc := New(
		fakeProvider{
			catalogResult: domain.AnimeDetail{
				ID:         "kuramanime:1929/pokemon-2023",
				Title:      "Pokemon (2023)",
				CoverImage: "https://example.com/pokemon.jpg",
				Recommendations: []domain.AnimeCard{
					{
						ID:       "kuramanime:2132/houkago-no-breath",
						Title:    "Houkago no Breath",
						Provider: "kuramanime",
					},
				},
				Provider: "kuramanime",
			},
			catalogByID: map[string]domain.AnimeDetail{
				"kuramanime:2132/houkago-no-breath": {
					ID:         "kuramanime:2132/houkago-no-breath",
					Title:      "Houkago no Breath",
					CoverImage: "https://example.com/houkago-detail.jpg",
					Provider:   "kuramanime",
				},
			},
		},
		cache.New(time.Minute),
		nil,
	)

	detail, err := svc.Catalog(context.Background(), "kuramanime:1929/pokemon-2023")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detail.Recommendations[0].CoverImage; got != "https://example.com/houkago-detail.jpg" {
		t.Fatalf("expected recommendation cover from related catalog, got %q", got)
	}
}

func TestCatalogReplacesExistingMyAnimeListRecommendationCover(t *testing.T) {
	cacheStore := cache.New(time.Minute)
	cacheStore.Set("catalog:kuramanime:1929/pokemon-2023", domain.AnimeDetail{
		ID:         "kuramanime:1929/pokemon-2023",
		Title:      "Pokemon (2023)",
		CoverImage: "https://example.com/pokemon.jpg",
		Recommendations: []domain.AnimeCard{
			{
				ID:         "kuramanime:2132/houkago-no-breath",
				Title:      "Houkago no Breath",
				CoverImage: "https://cdn.myanimelist.net/images/anime/1222/139173l.jpg",
				Provider:   "kuramanime",
			},
		},
		Provider: "kuramanime",
	}, time.Minute)

	svc := New(
		fakeProvider{
			catalogByID: map[string]domain.AnimeDetail{
				"kuramanime:2132/houkago-no-breath": {
					ID:         "kuramanime:2132/houkago-no-breath",
					Title:      "Houkago no Breath",
					CoverImage: "https://r2.nyomo.my.id/images/houkago.jpg",
					Provider:   "kuramanime",
				},
			},
		},
		cacheStore,
		nil,
	)

	detail, err := svc.Catalog(context.Background(), "kuramanime:1929/pokemon-2023")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detail.Recommendations[0].CoverImage; got != "https://r2.nyomo.my.id/images/houkago.jpg" {
		t.Fatalf("expected MAL recommendation cover to be replaced, got %q", got)
	}
}

func TestCatalogRefreshesMissingEpisodeReleaseDatesFromProvider(t *testing.T) {
	cacheStore := cache.New(time.Minute)
	cacheStore.Set("catalog:kuramanime:1929/pokemon-2023", domain.AnimeDetail{
		ID:         "kuramanime:1929/pokemon-2023",
		Title:      "Pokemon (2023)",
		CoverImage: "https://example.com/pokemon.jpg",
		Episodes: []domain.Episode{
			{
				ID:        "kuramanime:1929/pokemon-2023/episode/131",
				CatalogID: "kuramanime:1929/pokemon-2023",
				Number:    131,
				Title:     "Episode 131",
				Label:     "131",
			},
		},
		Provider: "kuramanime",
	}, time.Minute)

	svc := New(
		fakeProvider{
			catalogByID: map[string]domain.AnimeDetail{
				"kuramanime:1929/pokemon-2023": {
					ID:         "kuramanime:1929/pokemon-2023",
					Title:      "Pokemon (2023)",
					CoverImage: "https://example.com/pokemon.jpg",
					Episodes: []domain.Episode{
						{
							ID:         "kuramanime:1929/pokemon-2023/episode/131",
							CatalogID:  "kuramanime:1929/pokemon-2023",
							Number:     131,
							Title:      "Episode 131",
							Label:      "131",
							ReleasedAt: "15 Maret 2026",
						},
					},
					Provider: "kuramanime",
				},
			},
		},
		cacheStore,
		nil,
	)

	detail, err := svc.Catalog(context.Background(), "kuramanime:1929/pokemon-2023")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detail.Episodes[0].ReleasedAt; got != "15 Maret 2026" {
		t.Fatalf("expected release date to be refreshed from provider, got %q", got)
	}
}
