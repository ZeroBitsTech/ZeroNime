package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/config"
	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/mediacache"
	"anime/develop/backend/internal/mediaproxy"
	"anime/develop/backend/internal/provider"
	"anime/develop/backend/internal/provider/kuramanime"
	"anime/develop/backend/internal/provider/otakudesu"
	"anime/develop/backend/internal/service/catalog"
	"anime/develop/backend/internal/service/stream"
	"anime/develop/backend/internal/store/postgres"
)

func main() {
	query := flag.String("query", "Jujutsu Kaisen 2nd Season", "anime title query")
	latest := flag.Int("latest", 0, "only prewarm the latest N episodes; 0 prewarms all episodes")
	concurrency := flag.Int("concurrency", 4, "number of episodes to prewarm in parallel")
	flag.Parse()

	cfg := config.Load()
	store, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}

	registry := provider.NewRegistry(
		otakudesu.New(cfg.OtakudesuBaseURL, cfg.UserAgent, cfg.RequestTimeout),
		kuramanime.New(cfg.KuramanimeBaseURL, cfg.UserAgent, cfg.RequestTimeout, cfg.BrowserRenderBudget, cfg.BrowserPath),
	)
	activeProvider, err := registry.Select(cfg.ActiveProvider)
	if err != nil {
		log.Fatalf("select provider: %v", err)
	}

	memCache := cache.New(cfg.CacheTTL)
	catalogService := catalog.New(activeProvider, memCache, store)
	streamService := stream.New(activeProvider, memCache, store, cfg.StreamCacheTTL)
	blobStore, err := mediacache.NewBlobStore(cfg)
	if err != nil {
		log.Fatalf("configure media cache store: %v", err)
	}
	startupService := mediacache.New(store, blobStore, mediaproxy.New(), cfg.MediaCacheHeadBytes, cfg.MediaCacheTailBytes, cfg.MediaCacheFetchTTL)

	ctx := context.Background()
	results, err := catalogService.Search(ctx, *query)
	if err != nil {
		log.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		log.Fatalf("no search results for %q", *query)
	}

	selected := pickBestMatch(results, *query)
	detail, err := catalogService.Catalog(ctx, selected.ID)
	if err != nil {
		log.Fatalf("catalog detail: %v", err)
	}

	episodes := selectEpisodes(detail.Episodes, *latest)
	success, skipped := prewarmEpisodes(ctx, episodes, *concurrency, func(ctx context.Context, episode domain.Episode) error {
		result, resolveErr := streamService.Resolve(ctx, episode.ID)
		if resolveErr != nil {
			return fmt.Errorf("resolve: %w", resolveErr)
		}

		candidate, ok := pickStartupCandidate(result.Candidates)
		if !ok {
			return fmt.Errorf("no 720p mp4 candidate")
		}

		if err := startupService.Prewarm(ctx, episode.ID, candidate); err != nil {
			return fmt.Errorf("prewarm: %w", err)
		}
		log.Printf("prewarmed episode=%s quality=%s", episode.ID, candidate.Quality)
		return nil
	})

	fmt.Printf("catalog=%s title=%s episodes=%d selected=%d success=%d skipped=%d\n", detail.ID, detail.Title, len(detail.Episodes), len(episodes), success, skipped)
}

func pickBestMatch(itemsRaw []domain.AnimeCard, query string) domain.AnimeCard {
	normalizedQuery := normalizeTitle(query)
	for _, item := range itemsRaw {
		if normalizeTitle(item.Title) == normalizedQuery {
			return item
		}
	}
	for _, item := range itemsRaw {
		if strings.Contains(normalizeTitle(item.Title), normalizedQuery) {
			return item
		}
	}
	return itemsRaw[0]
}

func normalizeTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("2nd season", "season 2", " season", "", "-", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func pickStartupCandidate(candidates []domain.StreamCandidate) (domain.StreamCandidate, bool) {
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.Container, "mp4") && strings.EqualFold(candidate.Quality, "720p") {
			return candidate, true
		}
	}
	return domain.StreamCandidate{}, false
}

func selectEpisodes(episodes []domain.Episode, latest int) []domain.Episode {
	sorted := append([]domain.Episode(nil), episodes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Number == sorted[j].Number {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Number > sorted[j].Number
	})

	limit := latest
	if limit <= 0 || limit > 25 {
		limit = 25
	}
	if limit >= len(sorted) {
		return sorted
	}
	selected := make([]domain.Episode, limit)
	copy(selected, sorted[:limit])
	return selected
}

func prewarmEpisodes(ctx context.Context, episodes []domain.Episode, concurrency int, fn func(context.Context, domain.Episode) error) (success int, skipped int) {
	if concurrency < 1 {
		concurrency = 1
	}
	if len(episodes) == 0 {
		return 0, 0
	}
	if concurrency > len(episodes) && len(episodes) > 0 {
		concurrency = len(episodes)
	}

	jobs := make(chan domain.Episode)
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var skippedCount atomic.Int32

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for episode := range jobs {
				if err := fn(ctx, episode); err != nil {
					log.Printf("skip episode %s: %v", episode.ID, err)
					skippedCount.Add(1)
					continue
				}
				successCount.Add(1)
			}
		}()
	}

	for _, episode := range episodes {
		jobs <- episode
	}
	close(jobs)
	wg.Wait()

	return int(successCount.Load()), int(skippedCount.Load())
}
