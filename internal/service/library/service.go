package library

import (
	"context"
	"strings"

	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/provider"
	"anime/develop/backend/internal/store/postgres"
)

type Service struct {
	store    *postgres.Store
	provider provider.Provider
}

func New(store *postgres.Store, p provider.Provider) *Service {
	return &Service{store: store, provider: p}
}

func (s *Service) ListWatchlist(clientID uint) ([]domain.WatchlistItem, error) {
	rows, err := s.store.ListWatchlist(clientID)
	if err != nil {
		return nil, err
	}
	items := make([]domain.WatchlistItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.WatchlistItem{
			CatalogID:  row.CatalogID,
			Title:      row.Title,
			CoverImage: row.CoverImage,
			Provider:   row.Provider,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *Service) SaveWatchlist(ctx context.Context, clientID uint, catalogID string) error {
	detail, err := s.provider.Catalog(ctx, catalogID)
	if err != nil {
		return err
	}
	return s.store.UpsertWatchlist(clientID, catalogID, detail.Title, detail.CoverImage, detail.Provider)
}

func (s *Service) DeleteWatchlist(clientID uint, catalogID string) error {
	return s.store.DeleteWatchlist(clientID, catalogID)
}

func (s *Service) ListHistory(clientID uint) ([]domain.HistoryItem, error) {
	rows, err := s.store.ListHistory(clientID)
	if err != nil {
		return nil, err
	}
	items := make([]domain.HistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.HistoryItem{
			CatalogID:       row.CatalogID,
			EpisodeID:       row.EpisodeID,
			PositionSeconds: row.PositionSeconds,
			UpdatedAt:       row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *Service) SaveHistory(clientID uint, catalogID, episodeID string, positionSeconds int, title, coverImage, provider string) error {
	return s.store.UpsertHistory(clientID, catalogID, episodeID, positionSeconds, title, coverImage, provider)
}

func (s *Service) DeleteHistory(clientID uint, catalogID string) error {
	return s.store.DeleteHistory(clientID, catalogID)
}

func (s *Service) ContinueWatching(ctx context.Context, clientID uint) ([]domain.ContinueWatchingItem, error) {
	history, err := s.store.ListHistory(clientID)
	if err != nil {
		return nil, err
	}
	watchlist, err := s.store.ListWatchlist(clientID)
	if err != nil {
		return nil, err
	}

	fallbackMeta := make(map[string]domain.AnimeDetail)
	for _, catalogID := range collectMissingContinueWatchingMeta(history, watchlist) {
		detail, detailErr := s.provider.Catalog(ctx, catalogID)
		if detailErr != nil {
			continue
		}
		fallbackMeta[catalogID] = detail
	}

	return buildContinueWatchingItems(history, watchlist, fallbackMeta), nil
}

func collectMissingContinueWatchingMeta(history []postgres.WatchHistory, watchlist []postgres.Watchlist) []string {
	lookup := make(map[string]postgres.Watchlist, len(watchlist))
	for _, item := range watchlist {
		lookup[item.CatalogID] = item
	}

	seen := make(map[string]struct{}, len(history))
	missing := make([]string, 0, len(history))
	for _, row := range history {
		if _, exists := seen[row.CatalogID]; exists {
			continue
		}
		seen[row.CatalogID] = struct{}{}

		if strings.TrimSpace(row.Title) != "" && strings.TrimSpace(row.CoverImage) != "" {
			continue
		}

		meta, ok := lookup[row.CatalogID]
		if ok && strings.TrimSpace(meta.Title) != "" && strings.TrimSpace(meta.CoverImage) != "" {
			continue
		}

		missing = append(missing, row.CatalogID)
	}
	return missing
}

func buildContinueWatchingItems(
	history []postgres.WatchHistory,
	watchlist []postgres.Watchlist,
	fallbackMeta map[string]domain.AnimeDetail,
) []domain.ContinueWatchingItem {
	lookup := make(map[string]postgres.Watchlist, len(watchlist))
	for _, item := range watchlist {
		lookup[item.CatalogID] = item
	}

	items := make([]domain.ContinueWatchingItem, 0, len(history))
	for _, row := range history {
		meta := lookup[row.CatalogID]
		title := strings.TrimSpace(row.Title)
		coverImage := strings.TrimSpace(row.CoverImage)

		if title == "" {
			title = strings.TrimSpace(meta.Title)
		}
		if coverImage == "" {
			coverImage = strings.TrimSpace(meta.CoverImage)
		}

		if fallback, ok := fallbackMeta[row.CatalogID]; ok {
			if title == "" {
				title = strings.TrimSpace(fallback.Title)
			}
			if coverImage == "" {
				coverImage = strings.TrimSpace(fallback.CoverImage)
			}
		}

		items = append(items, domain.ContinueWatchingItem{
			CatalogID:       row.CatalogID,
			EpisodeID:       row.EpisodeID,
			Title:           title,
			CoverImage:      coverImage,
			PositionSeconds: row.PositionSeconds,
			UpdatedAt:       row.UpdatedAt,
		})
	}
	return items
}
