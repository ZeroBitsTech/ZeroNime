package kuramanime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"anime/develop/backend/internal/domain"
)

var scheduleDayOptions = []domain.SelectOption{
	{Value: "all", Label: "Semua"},
	{Value: "monday", Label: "Senin"},
	{Value: "tuesday", Label: "Selasa"},
	{Value: "wednesday", Label: "Rabu"},
	{Value: "thursday", Label: "Kamis"},
	{Value: "friday", Label: "Jumat"},
	{Value: "saturday", Label: "Sabtu"},
	{Value: "sunday", Label: "Minggu"},
	{Value: "random", Label: "Tidak Tentu"},
	{Value: "text", Label: "Mode Teks"},
}

var propertyTitles = map[string]string{
	"genre":   "Daftar Genre",
	"season":  "Daftar Musim",
	"studio":  "Daftar Studio",
	"country": "Daftar Negara",
}

var quickTitles = map[string]string{
	"finished": "Selesai Tayang",
	"ongoing":  "Sedang Tayang",
	"movie":    "Film Layar Lebar",
	"upcoming": "Segera Tayang",
}

type dataCountRow struct {
	AnimeID         string `json:"anime_id"`
	PostsViewsCount int64  `json:"posts_views_count"`
}

func (p *Provider) PropertyList(ctx context.Context, kind string) (domain.PropertyList, error) {
	path := "/properties/" + strings.Trim(strings.TrimSpace(kind), "/")
	doc, err := p.getDoc(ctx, path, p.baseURL)
	if err != nil {
		return domain.PropertyList{}, err
	}

	htmlBody, err := doc.Html()
	if err != nil {
		return domain.PropertyList{}, err
	}

	items := parsePropertyLinks(htmlBody, kind)
	return domain.PropertyList{
		Kind:  kind,
		Title: firstNonEmpty(propertyTitles[kind], normalizeSpace(doc.Find(".section-title h4").First().Text())),
		Items: items,
	}, nil
}

func (p *Provider) PropertyCatalog(ctx context.Context, kind, propertyID, order string, page int) (domain.CollectionPage, error) {
	path := fmt.Sprintf("/properties/%s/%s?order_by=%s&page=%d", strings.Trim(kind, "/"), strings.Trim(propertyID, "/"), sanitizeOrder(order), safePage(page))
	return p.collectionPage(ctx, path, propertyID, kind)
}

func (p *Provider) QuickCatalog(ctx context.Context, kind, order string, page int) (domain.CollectionPage, error) {
	path := fmt.Sprintf("/quick/%s?order_by=%s&page=%d", strings.Trim(kind, "/"), sanitizeOrder(order), safePage(page))
	return p.collectionPage(ctx, path, kind, "quick")
}

func (p *Provider) SchedulePage(ctx context.Context, day string, page int) (domain.CollectionPage, error) {
	selectedDay := strings.TrimSpace(day)
	if selectedDay == "" {
		selectedDay = "all"
	}
	path := fmt.Sprintf("/schedule?scheduled_day=%s&page=%d", selectedDay, safePage(page))
	doc, err := p.getDoc(ctx, path, p.baseURL)
	if err != nil {
		return domain.CollectionPage{}, err
	}

	htmlBody, err := doc.Html()
	if err != nil {
		return domain.CollectionPage{}, err
	}

	items := p.populateCardCounts(ctx, parseScheduleCardsHTML(htmlBody))
	_, _, pagination := parseDiscoveryControls(htmlBody)
	title := normalizeSpace(doc.Find(".section-title h4").First().Text())
	title = strings.ReplaceAll(title, "Jadwal Rilis:", "Jadwal Rilis")
	title = normalizeSpace(title)

	return domain.CollectionPage{
		Kind:          "schedule",
		Key:           selectedDay,
		Title:         firstNonEmpty(title, "Jadwal Rilis"),
		Items:         items,
		CurrentFilter: selectedDay,
		FilterOptions: append([]domain.SelectOption(nil), scheduleDayOptions...),
		Pagination:    pagination,
	}, nil
}

func (p *Provider) SeasonalPopular(ctx context.Context) (domain.CollectionPage, error) {
	seasons, err := p.PropertyList(ctx, "season")
	if err != nil {
		return domain.CollectionPage{}, err
	}
	current, ok := latestSeason(seasons.Items)
	if !ok {
		return domain.CollectionPage{}, fmt.Errorf("season list is empty")
	}
	page, err := p.PropertyCatalog(ctx, "season", current.ID, "most_viewed", 1)
	if err != nil {
		return domain.CollectionPage{}, err
	}
	page.Kind = "seasonal-popular"
	page.Key = current.ID
	page.Subtitle = current.Title
	if page.Title == "" {
		page.Title = "Terpopuler Musim Ini"
	}
	return page, nil
}

func (p *Provider) collectionPage(ctx context.Context, path, key, kind string) (domain.CollectionPage, error) {
	doc, err := p.getDoc(ctx, path, p.baseURL)
	if err != nil {
		return domain.CollectionPage{}, err
	}

	htmlBody, err := doc.Html()
	if err != nil {
		return domain.CollectionPage{}, err
	}

	options, currentOrder, pagination := parseDiscoveryControls(htmlBody)
	items := parseCardList(doc)
	if len(items) == 0 && strings.EqualFold(currentOrder, "text") {
		items = parseTextModeCards(htmlBody)
	}
	items = p.populateCardCounts(ctx, items)
	title := normalizeSpace(doc.Find(".section-title h4").First().Text())

	return domain.CollectionPage{
		Kind:         kind,
		Key:          strings.TrimSpace(key),
		Title:        title,
		Items:        items,
		CurrentOrder: currentOrder,
		OrderOptions: options,
		Pagination:   pagination,
	}, nil
}

func (p *Provider) populateCardCounts(ctx context.Context, items []domain.AnimeCard) []domain.AnimeCard {
	if len(items) == 0 {
		return items
	}

	ids := make([]string, 0, len(items))
	indexByAnimeID := map[string][]int{}
	for index, item := range items {
		_, slug, err := domain.ParseQualifiedID(item.ID)
		if err != nil {
			continue
		}
		animeID := animeIDFromSlug(slug)
		if animeID == "" {
			continue
		}
		if _, ok := indexByAnimeID[animeID]; !ok {
			ids = append(ids, animeID)
		}
		indexByAnimeID[animeID] = append(indexByAnimeID[animeID], index)
	}

	if len(ids) == 0 {
		return items
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/misc/anime/data-count?anime_id="+strings.Join(ids, ","), nil)
	if err != nil {
		return items
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Referer", p.baseURL)

	resp, err := p.client.Do(req)
	if err != nil {
		return items
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return items
	}

	var rows []dataCountRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return items
	}

	cloned := append([]domain.AnimeCard(nil), items...)
	for _, row := range rows {
		for _, index := range indexByAnimeID[row.AnimeID] {
			cloned[index].ViewsLabel = formatCompactViews(row.PostsViewsCount)
		}
	}
	return cloned
}

func sanitizeOrder(value string) string {
	order := strings.TrimSpace(value)
	if order == "" {
		return "ascending"
	}
	if _, ok := discoveryOrderLabels[order]; ok {
		return order
	}
	return "ascending"
}

func safePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func formatCompactViews(value int64) string {
	switch {
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func latestSeason(items []domain.PropertyItem) (domain.PropertyItem, bool) {
	if len(items) == 0 {
		return domain.PropertyItem{}, false
	}
	return items[len(items)-1], true
}
