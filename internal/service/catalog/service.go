package catalog

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/provider"
	"anime/develop/backend/internal/store/postgres"
)

type Service struct {
	provider provider.Provider
	cache    *cache.Cache
	store    *postgres.Store
}

var episodeNumberPattern = regexp.MustCompile(`\d+`)
var scoreLabelPattern = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
const discoveryCacheVersion = "v2r2"

func New(p provider.Provider, c *cache.Cache, store *postgres.Store) *Service {
	return &Service{provider: p, cache: c, store: store}
}

func (s *Service) Home(ctx context.Context) (domain.HomeFeed, error) {
	const key = "home"
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.HomeFeed); ok {
			return value, nil
		}
	}
	value, err := s.provider.Home(ctx)
	if err == nil {
		value = normalizeHome(value)
		s.cache.Set(key, value, 0)
	}
	return value, err
}

func (s *Service) Search(ctx context.Context, query string) ([]domain.AnimeCard, error) {
	key := fmt.Sprintf("search:%s", query)
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.([]domain.AnimeCard); ok {
			if value == nil {
				return make([]domain.AnimeCard, 0), nil
			}
			return value, nil
		}
	}
	value, err := s.provider.Search(ctx, query)
	if err == nil {
		value = normalizeAnimeCards(value)
		if value == nil {
			value = make([]domain.AnimeCard, 0)
		}
		s.cache.Set(key, value, 0)
	}
	return value, err
}

func (s *Service) Schedule(ctx context.Context) ([]domain.ScheduleDay, error) {
	const key = "schedule"
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.([]domain.ScheduleDay); ok {
			return value, nil
		}
	}
	value, err := s.provider.Schedule(ctx)
	if err == nil {
		value = normalizeSchedule(value)
		s.cache.Set(key, value, 0)
	}
	return value, err
}

func (s *Service) SchedulePage(ctx context.Context, day string, page int) (domain.CollectionPage, error) {
	key := fmt.Sprintf("discovery:%s:schedule:%s:%d", discoveryCacheVersion, strings.TrimSpace(day), page)
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.CollectionPage); ok {
			return normalizeCollectionPage(value), nil
		}
	}
	if s.store != nil {
		var cached domain.CollectionPage
		if ok, err := s.store.GetDiscoveryCache(key, &cached); err == nil && ok {
			normalized := normalizeCollectionPage(cached)
			s.cache.Set(key, normalized, 0)
			return normalized, nil
		}
	}
	value, err := s.provider.SchedulePage(ctx, day, page)
	if err == nil {
		value = normalizeCollectionPage(value)
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertDiscoveryCache(key, value)
		}
	}
	return value, err
}

func (s *Service) Index(ctx context.Context) (map[string][]domain.AnimeCard, error) {
	const key = "index"
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(map[string][]domain.AnimeCard); ok {
			return value, nil
		}
	}
	value, err := s.provider.Index(ctx)
	if err == nil {
		s.cache.Set(key, value, 0)
	}
	return value, err
}

func (s *Service) PropertyList(ctx context.Context, kind string) (domain.PropertyList, error) {
	key := fmt.Sprintf("discovery:%s:property-list:%s", discoveryCacheVersion, strings.TrimSpace(kind))
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.PropertyList); ok {
			return value, nil
		}
	}
	if s.store != nil {
		var cached domain.PropertyList
		if ok, err := s.store.GetDiscoveryCache(key, &cached); err == nil && ok {
			s.cache.Set(key, cached, 0)
			return cached, nil
		}
	}
	value, err := s.provider.PropertyList(ctx, kind)
	if err == nil {
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertDiscoveryCache(key, value)
		}
	}
	return value, err
}

func (s *Service) PropertyCatalog(ctx context.Context, kind, propertyID, order string, page int) (domain.CollectionPage, error) {
	key := fmt.Sprintf("discovery:%s:property:%s:%s:%s:%d", discoveryCacheVersion, strings.TrimSpace(kind), strings.TrimSpace(propertyID), strings.TrimSpace(order), page)
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.CollectionPage); ok {
			return normalizeCollectionPage(value), nil
		}
	}
	if s.store != nil {
		var cached domain.CollectionPage
		if ok, err := s.store.GetDiscoveryCache(key, &cached); err == nil && ok {
			normalized := normalizeCollectionPage(cached)
			s.cache.Set(key, normalized, 0)
			return normalized, nil
		}
	}
	value, err := s.provider.PropertyCatalog(ctx, kind, propertyID, order, page)
	if err == nil {
		value = normalizeCollectionPage(value)
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertDiscoveryCache(key, value)
		}
	}
	return value, err
}

func (s *Service) QuickCatalog(ctx context.Context, kind, order string, page int) (domain.CollectionPage, error) {
	key := fmt.Sprintf("discovery:%s:quick:%s:%s:%d", discoveryCacheVersion, strings.TrimSpace(kind), strings.TrimSpace(order), page)
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.CollectionPage); ok {
			return normalizeCollectionPage(value), nil
		}
	}
	if s.store != nil {
		var cached domain.CollectionPage
		if ok, err := s.store.GetDiscoveryCache(key, &cached); err == nil && ok {
			normalized := normalizeCollectionPage(cached)
			s.cache.Set(key, normalized, 0)
			return normalized, nil
		}
	}
	value, err := s.provider.QuickCatalog(ctx, kind, order, page)
	if err == nil {
		value = normalizeCollectionPage(value)
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertDiscoveryCache(key, value)
		}
	}
	return value, err
}

func (s *Service) SeasonalPopular(ctx context.Context) (domain.CollectionPage, error) {
	const key = "discovery:v2r2:seasonal-popular"
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.CollectionPage); ok {
			return normalizeCollectionPage(value), nil
		}
	}
	if s.store != nil {
		var cached domain.CollectionPage
		if ok, err := s.store.GetDiscoveryCache(key, &cached); err == nil && ok {
			normalized := normalizeCollectionPage(cached)
			s.cache.Set(key, normalized, 0)
			return normalized, nil
		}
	}
	value, err := s.provider.SeasonalPopular(ctx)
	if err == nil {
		value = normalizeCollectionPage(value)
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertDiscoveryCache(key, value)
		}
	}
	return value, err
}

func (s *Service) Catalog(ctx context.Context, id string) (domain.AnimeDetail, error) {
	key := fmt.Sprintf("catalog:%s", id)
	if raw, ok := s.cache.Get(key); ok {
		if value, ok := raw.(domain.AnimeDetail); ok {
			value = s.refreshPrimaryDetailCover(ctx, id, value)
			value = s.refreshEpisodeReleaseDates(ctx, id, value)
			value = s.hydrateRecommendationCovers(ctx, value)
			value = normalizeDetail(value)
			s.cache.Set(key, value, 0)
			return value, nil
		}
	}

	if s.store != nil {
		if value, ok, err := s.store.GetCatalogCache(id); err == nil && ok && value != nil {
			hydrated := s.refreshPrimaryDetailCover(ctx, id, *value)
			hydrated = s.refreshEpisodeReleaseDates(ctx, id, hydrated)
			hydrated = s.hydrateRecommendationCovers(ctx, hydrated)
			hydrated = normalizeDetail(hydrated)
			s.cache.Set(key, hydrated, 0)
			_ = s.store.UpsertCatalogCache(id, hydrated)
			return hydrated, nil
		}
	}

	value, err := s.provider.Catalog(ctx, id)
	if err == nil {
		value = s.refreshPrimaryDetailCover(ctx, id, value)
		value = s.refreshEpisodeReleaseDates(ctx, id, value)
		value = s.hydrateRecommendationCovers(ctx, value)
		value = normalizeDetail(value)
		s.cache.Set(key, value, 0)
		if s.store != nil {
			_ = s.store.UpsertCatalogCache(id, value)
		}
	}
	return value, err
}

func (s *Service) Episodes(ctx context.Context, id string) ([]domain.Episode, error) {
	detail, err := s.Catalog(ctx, id)
	if err != nil {
		return nil, err
	}
	return detail.Episodes, nil
}

func normalizeDetail(detail domain.AnimeDetail) domain.AnimeDetail {
	if detail.Metadata == nil {
		return detail
	}

	delete(detail.Metadata, "eksplisit")
	delete(detail.Metadata, "tema")

	if len(detail.Episodes) > 0 {
		value := strings.TrimSpace(detail.Metadata["episode"])
		if value == "" || value == "?" {
			detail.Metadata["episode"] = fmt.Sprintf("%d", inferredEpisodeCount(detail.Episodes))
		}
	}

	return detail
}

func (s *Service) hydrateRecommendationCovers(ctx context.Context, detail domain.AnimeDetail) domain.AnimeDetail {
	if len(detail.Recommendations) == 0 {
		return detail
	}

	for index := range detail.Recommendations {
		currentCover := strings.TrimSpace(detail.Recommendations[index].CoverImage)
		if currentCover != "" && !isMyAnimeListCover(currentCover) {
			continue
		}
		cover := strings.TrimSpace(s.findRecommendationCover(ctx, detail.Recommendations[index]))
		if cover == "" {
			continue
		}
		detail.Recommendations[index].CoverImage = cover
	}

	return detail
}

func (s *Service) refreshPrimaryDetailCover(ctx context.Context, catalogID string, detail domain.AnimeDetail) domain.AnimeDetail {
	currentCover := strings.TrimSpace(detail.CoverImage)
	if currentCover != "" && !isMyAnimeListCover(currentCover) {
		return detail
	}

	freshDetail, err := s.provider.Catalog(ctx, catalogID)
	if err != nil {
		return detail
	}
	freshCover := strings.TrimSpace(freshDetail.CoverImage)
	if freshCover == "" || isMyAnimeListCover(freshCover) {
		return detail
	}
	detail.CoverImage = freshCover
	return detail
}

func (s *Service) findRecommendationCover(ctx context.Context, item domain.AnimeCard) string {
	if cover := s.coverFromRelatedCatalog(ctx, item.ID); cover != "" {
		return cover
	}

	results, err := s.provider.Search(ctx, item.Title)
	if err != nil || len(results) == 0 {
		return ""
	}

	for _, candidate := range results {
		if candidate.ID == item.ID && strings.TrimSpace(candidate.CoverImage) != "" {
			return candidate.CoverImage
		}
	}

	normalizedTitle := normalizeComparisonTitle(item.Title)
	for _, candidate := range results {
		if normalizeComparisonTitle(candidate.Title) == normalizedTitle && strings.TrimSpace(candidate.CoverImage) != "" {
			return candidate.CoverImage
		}
	}

	for _, candidate := range results {
		if strings.TrimSpace(candidate.CoverImage) != "" {
			return candidate.CoverImage
		}
	}

	return ""
}

func normalizeComparisonTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "’", "'")
	value = strings.ReplaceAll(value, "`", "'")
	return value
}

func (s *Service) coverFromRelatedCatalog(ctx context.Context, catalogID string) string {
	cacheKey := fmt.Sprintf("catalog:%s", catalogID)
	if raw, ok := s.cache.Get(cacheKey); ok {
		if value, ok := raw.(domain.AnimeDetail); ok && strings.TrimSpace(value.CoverImage) != "" {
			cover := strings.TrimSpace(value.CoverImage)
			if !isMyAnimeListCover(cover) {
				return cover
			}
		}
	}

	if s.store != nil {
		if value, ok, err := s.store.GetCatalogCache(catalogID); err == nil && ok && value != nil && strings.TrimSpace(value.CoverImage) != "" {
			cover := strings.TrimSpace(value.CoverImage)
			if !isMyAnimeListCover(cover) {
				s.cache.Set(cacheKey, *value, 0)
				return cover
			}
		}
	}

	detail, err := s.provider.Catalog(ctx, catalogID)
	if err != nil || detail.ID != catalogID || strings.TrimSpace(detail.CoverImage) == "" {
		return ""
	}

	detail = normalizeDetail(detail)
	s.cache.Set(cacheKey, detail, 0)
	if s.store != nil {
		_ = s.store.UpsertCatalogCache(catalogID, detail)
	}
	return strings.TrimSpace(detail.CoverImage)
}

func isMyAnimeListCover(value string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), "cdn.myanimelist.net/")
}

func (s *Service) refreshEpisodeReleaseDates(ctx context.Context, catalogID string, detail domain.AnimeDetail) domain.AnimeDetail {
	if !needsEpisodeReleaseRefresh(detail.Episodes) {
		return detail
	}

	freshDetail, err := s.provider.Catalog(ctx, catalogID)
	if err != nil || !hasEpisodeReleaseDates(freshDetail.Episodes) {
		return detail
	}

	detail.Episodes = freshDetail.Episodes
	return detail
}

func needsEpisodeReleaseRefresh(items []domain.Episode) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.ReleasedAt) == "" {
			return true
		}
	}
	return false
}

func hasEpisodeReleaseDates(items []domain.Episode) bool {
	for _, item := range items {
		if strings.TrimSpace(item.ReleasedAt) != "" {
			return true
		}
	}
	return false
}

func normalizeHome(feed domain.HomeFeed) domain.HomeFeed {
	feed.Featured = normalizeAnimeCards(feed.Featured)
	feed.Ongoing = normalizeAnimeCards(feed.Ongoing)
	feed.Recent = normalizeAnimeCards(feed.Recent)
	return feed
}

func normalizeSchedule(items []domain.ScheduleDay) []domain.ScheduleDay {
	if len(items) == 0 {
		return items
	}
	cloned := make([]domain.ScheduleDay, 0, len(items))
	for _, item := range items {
		item.Items = normalizeAnimeCards(item.Items)
		cloned = append(cloned, item)
	}
	return cloned
}

func normalizeCollectionPage(page domain.CollectionPage) domain.CollectionPage {
	page.Items = normalizeAnimeCards(page.Items)
	return page
}

func normalizeAnimeCards(items []domain.AnimeCard) []domain.AnimeCard {
	if len(items) == 0 {
		return items
	}
	cloned := make([]domain.AnimeCard, 0, len(items))
	for _, item := range items {
		if scoreLabelPattern.MatchString(strings.TrimSpace(item.EpisodeLabel)) {
			item.EpisodeLabel = ""
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func inferredEpisodeCount(items []domain.Episode) int {
	maxNumber := 0
	for _, item := range items {
		if item.Number > maxNumber {
			maxNumber = item.Number
		}

		for _, raw := range episodeNumberPattern.FindAllString(item.Label, -1) {
			var value int
			_, _ = fmt.Sscanf(raw, "%d", &value)
			if value > maxNumber {
				maxNumber = value
			}
		}

		for _, raw := range episodeNumberPattern.FindAllString(item.Title, -1) {
			var value int
			_, _ = fmt.Sscanf(raw, "%d", &value)
			if value > maxNumber {
				maxNumber = value
			}
		}
	}

	if maxNumber == 0 {
		return len(items)
	}
	return maxNumber
}
