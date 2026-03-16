package kuramanime

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"unicode"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

func (p *Provider) Home(ctx context.Context) (domain.HomeFeed, error) {
	doc, err := p.getDoc(ctx, "/", "https://google.com")
	if err != nil {
		return domain.HomeFeed{}, err
	}

	sections := doc.Find(".trending__product")
	var ongoing []domain.AnimeCard
	var completed []domain.AnimeCard
	var movie []domain.AnimeCard

	sections.Each(func(index int, section *goquery.Selection) {
		target := &ongoing
		if index == 1 {
			target = &completed
		}
		if index >= 2 {
			target = &movie
		}

		section.Find(".product__item").Each(func(_ int, item *goquery.Selection) {
			card := parseAnimeCardAnchor(item)
			if index == 0 {
				card = parseEpisodeCardAnchor(item)
			}
			if card.ID == "" || card.Title == "" {
				return
			}
			*target = append(*target, card)
		})
	})

	featured := ongoing
	if len(featured) > 6 {
		featured = featured[:6]
	}

	recent := completed
	if len(recent) == 0 {
		recent = movie
	}

	featured = p.populateCardCounts(ctx, featured)
	ongoing = p.populateCardCounts(ctx, ongoing)
	recent = p.populateCardCounts(ctx, recent)

	return domain.HomeFeed{
		Featured: featured,
		Ongoing:  ongoing,
		Recent:   recent,
	}, nil
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.AnimeCard, error) {
	doc, err := p.getDoc(ctx, "/anime?order_by=latest&page=1&search="+urlQueryEscape(query), p.baseURL)
	if err != nil {
		return nil, err
	}
	return p.populateCardCounts(ctx, parseCardList(doc)), nil
}

func (p *Provider) Schedule(ctx context.Context) ([]domain.ScheduleDay, error) {
	days := []struct {
		label string
		key   string
	}{
		{label: "Senin", key: "monday"},
		{label: "Selasa", key: "tuesday"},
		{label: "Rabu", key: "wednesday"},
		{label: "Kamis", key: "thursday"},
		{label: "Jumat", key: "friday"},
		{label: "Sabtu", key: "saturday"},
		{label: "Minggu", key: "sunday"},
	}

	result := make([]domain.ScheduleDay, 0, len(days))
	for _, day := range days {
		doc, err := p.getDoc(ctx, "/schedule?scheduled_day="+day.key+"&page=1", p.baseURL)
		if err != nil {
			return nil, err
		}
		items := parseCardList(doc)
		items = p.populateCardCounts(ctx, items)
		result = append(result, domain.ScheduleDay{
			Day:   day.label,
			Items: items,
		})
	}

	return result, nil
}

func (p *Provider) Index(ctx context.Context) (map[string][]domain.AnimeCard, error) {
	index := make(map[string][]domain.AnimeCard)

	for page := 1; page <= 60; page++ {
		doc, err := p.getDoc(ctx, fmt.Sprintf("/anime?order_by=ascending&page=%d", page), p.baseURL)
		if err != nil {
			return nil, err
		}

		items := parseCardList(doc)
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			key := indexBucket(item.Title)
			index[key] = append(index[key], item)
		}
	}

	return index, nil
}

func indexBucket(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "#"
	}

	first := []rune(trimmed)[0]
	if unicode.IsLetter(first) {
		return strings.ToUpper(string(first))
	}
	return "#"
}

func (p *Provider) Catalog(ctx context.Context, catalogID string) (domain.AnimeDetail, error) {
	_, slug, err := domain.ParseQualifiedID(catalogID)
	if err != nil {
		return domain.AnimeDetail{}, err
	}

	doc, err := p.getDoc(ctx, "/anime/"+slug, p.baseURL)
	if err != nil {
		return domain.AnimeDetail{}, err
	}

	title := normalizeSpace(doc.Find(".anime__details__title h3").First().Text())
	cover := detailCoverFromDoc(doc)
	if isMyAnimeListCover(cover) {
		if altDoc, altErr := p.getDocWithUserAgent(ctx, "/anime/"+slug, p.baseURL, "curl/8.5.0"); altErr == nil {
			cover = preferredDetailCover(cover, detailCoverFromDoc(altDoc))
		}
	}
	synopsis := normalizeSpace(doc.Find("#synopsisField").Text())

	metadata := map[string]string{}
	doc.Find(".anime__details__widget ul li").Each(func(_ int, item *goquery.Selection) {
		label := normalizeSpace(item.Find(".col-3").Text())
		value := normalizeSpace(item.Find(".col-9").Text())
		if label == "" || value == "" {
			return
		}
		label = strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(label), ":"), " :")
		metadata[label] = value
	})

	genres := make([]string, 0)
	doc.Find(".anime__details__widget ul li .col-9 a[href*='/properties/genre/']").Each(func(_ int, a *goquery.Selection) {
		value := normalizeSpace(a.Text())
		if value != "" {
			genres = append(genres, value)
		}
	})

	episodes, err := p.collectEpisodes(ctx, doc, catalogID, slug)
	if err != nil {
		return domain.AnimeDetail{}, err
	}

	delete(metadata, "eksplisit")
	if count := len(episodes); count > 0 {
		current := strings.TrimSpace(metadata["episode"])
		if current == "" || current == "?" {
			metadata["episode"] = fmt.Sprintf("%d", count)
		}
	}

	recommendations := make([]domain.AnimeCard, 0)
	doc.Find(".breadcrumb__links__v2 a[href*='/anime/']").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		animeSlug := animeSlugFromHref(href)
		if animeSlug == "" {
			return
		}
		title := strings.TrimPrefix(normalizeSpace(a.Text()), "- ")
		if title == "" {
			return
		}
		recommendations = append(recommendations, domain.AnimeCard{
			ID:       catalogIDFromSlug(animeSlug),
			Title:    title,
			Provider: Name,
		})
	})

	return domain.AnimeDetail{
		ID:              catalogID,
		Title:           title,
		Synopsis:        synopsis,
		CoverImage:      cover,
		Genres:          genres,
		Metadata:        metadata,
		Episodes:        episodes,
		Recommendations: recommendations,
		Provider:        Name,
	}, nil
}

func detailCoverFromDoc(doc *goquery.Document) string {
	cover, _ := doc.Find(".anime__details__pic").First().Attr("data-setbg")
	if strings.TrimSpace(cover) != "" {
		return strings.TrimSpace(cover)
	}
	cover, _ = doc.Find(".anime__details__pic__mobile").First().Attr("data-setbg")
	return strings.TrimSpace(cover)
}

func preferredDetailCover(primary, alternative string) string {
	primary = strings.TrimSpace(primary)
	alternative = strings.TrimSpace(alternative)
	if alternative != "" && isMyAnimeListCover(primary) && !isMyAnimeListCover(alternative) {
		return alternative
	}
	if primary != "" {
		return primary
	}
	return alternative
}

func isMyAnimeListCover(value string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), "cdn.myanimelist.net/")
}

func (p *Provider) Episodes(ctx context.Context, catalogID string) ([]domain.Episode, error) {
	detail, err := p.Catalog(ctx, catalogID)
	if err != nil {
		return nil, err
	}
	return detail.Episodes, nil
}

func parseCardList(doc *goquery.Document) []domain.AnimeCard {
	items := make([]domain.AnimeCard, 0)
	doc.Find("#animeList .product__item").Each(func(_ int, item *goquery.Selection) {
		card := parseAnimeCardAnchor(item)
		if card.ID == "" || card.Title == "" {
			return
		}
		items = append(items, card)
	})
	return items
}

func (p *Provider) collectEpisodes(ctx context.Context, doc *goquery.Document, catalogID, slug string) ([]domain.Episode, error) {
	htmlBody, err := doc.Html()
	if err != nil {
		return nil, err
	}

	content := html.UnescapeString(htmlBody)
	refs := extractEpisodeRefsFromHTML(content, slug)
	if len(refs) == 0 {
		return nil, fmt.Errorf("episode list content missing")
	}

	queue := make([]string, 0, 8)
	visitedPages := map[string]struct{}{}

	for _, page := range extractEpisodePageLinks(content) {
		if _, exists := visitedPages[page]; exists {
			continue
		}
		visitedPages[page] = struct{}{}
		queue = append(queue, page)
	}

	if seedPath := refs[0].Path; seedPath != "" {
		if _, exists := visitedPages[seedPath]; !exists {
			visitedPages[seedPath] = struct{}{}
			queue = append(queue, seedPath)
		}
	}

	const maxEpisodePages = 24
	for i := 0; i < len(queue) && i < maxEpisodePages; i++ {
		pagePath := queue[i]
		pageDoc, fetchErr := p.getDoc(ctx, pagePath, p.baseURL+"/anime/"+slug)
		if fetchErr != nil {
			continue
		}

		pageHTML, htmlErr := pageDoc.Html()
		if htmlErr != nil {
			continue
		}

		pageContent := html.UnescapeString(pageHTML)
		refs = mergeEpisodeRefs(refs, extractEpisodeRefsFromHTML(pageContent, slug))
		for _, page := range extractEpisodePageLinks(pageContent) {
			if _, exists := visitedPages[page]; exists {
				continue
			}
			visitedPages[page] = struct{}{}
			queue = append(queue, page)
		}
	}

	episodes := make([]domain.Episode, 0, len(refs))
	for _, ref := range refs {
		display := ref.Display
		if display == "" {
			display = fmt.Sprintf("%d", ref.Number)
		}

		episodes = append(episodes, domain.Episode{
			ID:        domain.NewCatalogID(Name, ref.Slug),
			CatalogID: catalogID,
			Number:    ref.Number,
			Title:     "Episode " + display,
			Label:     display,
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].Number == episodes[j].Number {
			return episodes[i].ID < episodes[j].ID
		}
		return episodes[i].Number > episodes[j].Number
	})

	return p.enrichEpisodeReleaseDates(ctx, episodes), nil
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(" ", "+")
	return replacer.Replace(strings.TrimSpace(value))
}
