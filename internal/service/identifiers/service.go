package identifiers

import (
	"strings"

	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/store/postgres"
)

const (
	kindCatalog = "catalog"
	kindEpisode = "episode"
)

type Service struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Service {
	return &Service{store: store}
}

func (s *Service) ResolveCatalogID(value string) (string, error) {
	return s.resolve(kindCatalog, value)
}

func (s *Service) ResolveEpisodeID(value string) (string, error) {
	return s.resolve(kindEpisode, value)
}

func (s *Service) PublicCatalogID(internalID string) string {
	return s.public(kindCatalog, internalID)
}

func (s *Service) PublicEpisodeID(internalID string) string {
	return s.public(kindEpisode, internalID)
}

func (s *Service) PublicAnimeCard(item domain.AnimeCard) domain.AnimeCard {
	if item.ID != "" {
		item.ID = s.PublicCatalogID(item.ID)
	}
	return item
}

func (s *Service) PublicAnimeCards(items []domain.AnimeCard) []domain.AnimeCard {
	if len(items) == 0 {
		return items
	}
	cloned := make([]domain.AnimeCard, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, s.PublicAnimeCard(item))
	}
	return cloned
}

func (s *Service) PublicHome(feed domain.HomeFeed) domain.HomeFeed {
	feed.Featured = s.PublicAnimeCards(feed.Featured)
	feed.Ongoing = s.PublicAnimeCards(feed.Ongoing)
	feed.Recent = s.PublicAnimeCards(feed.Recent)
	return feed
}

func (s *Service) PublicSchedule(items []domain.ScheduleDay) []domain.ScheduleDay {
	cloned := make([]domain.ScheduleDay, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, domain.ScheduleDay{
			Day:   item.Day,
			Items: s.PublicAnimeCards(item.Items),
		})
	}
	return cloned
}

func (s *Service) PublicIndex(items map[string][]domain.AnimeCard) map[string][]domain.AnimeCard {
	cloned := make(map[string][]domain.AnimeCard, len(items))
	for key, value := range items {
		cloned[key] = s.PublicAnimeCards(value)
	}
	return cloned
}

func (s *Service) PublicPropertyList(item domain.PropertyList) domain.PropertyList {
	cloned := item
	if len(item.Items) > 0 {
		cloned.Items = append(make([]domain.PropertyItem, 0, len(item.Items)), item.Items...)
	}
	return cloned
}

func (s *Service) PublicCollectionPage(item domain.CollectionPage) domain.CollectionPage {
	cloned := item
	cloned.Items = s.PublicAnimeCards(item.Items)
	if len(item.FilterOptions) > 0 {
		cloned.FilterOptions = append(make([]domain.SelectOption, 0, len(item.FilterOptions)), item.FilterOptions...)
	}
	if len(item.OrderOptions) > 0 {
		cloned.OrderOptions = append(make([]domain.SelectOption, 0, len(item.OrderOptions)), item.OrderOptions...)
	}
	return cloned
}

func (s *Service) PublicEpisodes(items []domain.Episode) []domain.Episode {
	cloned := make([]domain.Episode, 0, len(items))
	for _, item := range items {
		if item.ID != "" {
			item.ID = s.PublicEpisodeID(item.ID)
		}
		if item.CatalogID != "" {
			item.CatalogID = s.PublicCatalogID(item.CatalogID)
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func (s *Service) PublicDetail(item domain.AnimeDetail) domain.AnimeDetail {
	if item.ID != "" {
		item.ID = s.PublicCatalogID(item.ID)
	}
	item.Episodes = s.PublicEpisodes(item.Episodes)
	item.Recommendations = s.PublicAnimeCards(item.Recommendations)
	return item
}

func (s *Service) PublicStream(item domain.StreamResult) domain.StreamResult {
	if item.EpisodeID != "" {
		item.EpisodeID = s.PublicEpisodeID(item.EpisodeID)
	}
	return item
}

func (s *Service) PublicWatchlist(items []domain.WatchlistItem) []domain.WatchlistItem {
	cloned := make([]domain.WatchlistItem, 0, len(items))
	for _, item := range items {
		if item.CatalogID != "" {
			item.CatalogID = s.PublicCatalogID(item.CatalogID)
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func (s *Service) PublicHistory(items []domain.HistoryItem) []domain.HistoryItem {
	cloned := make([]domain.HistoryItem, 0, len(items))
	for _, item := range items {
		if item.CatalogID != "" {
			item.CatalogID = s.PublicCatalogID(item.CatalogID)
		}
		if item.EpisodeID != "" {
			item.EpisodeID = s.PublicEpisodeID(item.EpisodeID)
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func (s *Service) PublicContinueWatching(items []domain.ContinueWatchingItem) []domain.ContinueWatchingItem {
	cloned := make([]domain.ContinueWatchingItem, 0, len(items))
	for _, item := range items {
		if item.CatalogID != "" {
			item.CatalogID = s.PublicCatalogID(item.CatalogID)
		}
		if item.EpisodeID != "" {
			item.EpisodeID = s.PublicEpisodeID(item.EpisodeID)
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func (s *Service) resolve(kind, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", domain.ErrInvalidIdentifier
	}
	if _, _, err := domain.ParseQualifiedID(trimmed); err == nil {
		return trimmed, nil
	}
	internalID, ok, err := s.store.ResolveAlias(kind, trimmed)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", domain.ErrInvalidIdentifier
	}
	return internalID, nil
}

func (s *Service) public(kind, internalID string) string {
	trimmed := strings.TrimSpace(internalID)
	if trimmed == "" {
		return ""
	}
	publicID, err := s.store.EnsureAlias(kind, trimmed)
	if err != nil {
		return trimmed
	}
	return publicID
}
