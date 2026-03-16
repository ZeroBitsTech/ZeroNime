package provider_test

import (
	"context"
	"testing"

	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/provider"
)

type stubProvider struct {
	name string
}

func (s stubProvider) Name() string { return s.name }

func (s stubProvider) Home(context.Context) (domain.HomeFeed, error) { return domain.HomeFeed{}, nil }

func (s stubProvider) Search(context.Context, string) ([]domain.AnimeCard, error) { return nil, nil }

func (s stubProvider) Schedule(context.Context) ([]domain.ScheduleDay, error) { return nil, nil }

func (s stubProvider) SchedulePage(context.Context, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (s stubProvider) Index(context.Context) (map[string][]domain.AnimeCard, error) { return nil, nil }

func (s stubProvider) PropertyList(context.Context, string) (domain.PropertyList, error) {
	return domain.PropertyList{}, nil
}

func (s stubProvider) PropertyCatalog(context.Context, string, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (s stubProvider) QuickCatalog(context.Context, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (s stubProvider) SeasonalPopular(context.Context) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, nil
}

func (s stubProvider) Catalog(context.Context, string) (domain.AnimeDetail, error) {
	return domain.AnimeDetail{}, nil
}

func (s stubProvider) Episodes(context.Context, string) ([]domain.Episode, error) { return nil, nil }

func (s stubProvider) StreamCandidates(context.Context, string) ([]domain.StreamCandidate, error) {
	return nil, nil
}

func TestSelectUsesExplicitlyConfiguredProvider(t *testing.T) {
	registry := provider.NewRegistry(
		stubProvider{name: "otakudesu"},
		stubProvider{name: "kuramanime"},
	)

	selected, err := registry.Select("kuramanime")
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if selected.Name() != "kuramanime" {
		t.Fatalf("expected selected provider kuramanime, got %s", selected.Name())
	}
}
