package provider

import (
	"context"

	"anime/develop/backend/internal/domain"
)

type Provider interface {
	Name() string
	Home(context.Context) (domain.HomeFeed, error)
	Search(context.Context, string) ([]domain.AnimeCard, error)
	Schedule(context.Context) ([]domain.ScheduleDay, error)
	SchedulePage(context.Context, string, int) (domain.CollectionPage, error)
	Index(context.Context) (map[string][]domain.AnimeCard, error)
	PropertyList(context.Context, string) (domain.PropertyList, error)
	PropertyCatalog(context.Context, string, string, string, int) (domain.CollectionPage, error)
	QuickCatalog(context.Context, string, string, int) (domain.CollectionPage, error)
	SeasonalPopular(context.Context) (domain.CollectionPage, error)
	Catalog(context.Context, string) (domain.AnimeDetail, error)
	Episodes(context.Context, string) ([]domain.Episode, error)
	StreamCandidates(context.Context, string) ([]domain.StreamCandidate, error)
}
