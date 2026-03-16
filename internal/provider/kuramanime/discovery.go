package kuramanime

import (
	"net/url"
	"strconv"
	"strings"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

var discoveryOrderLabels = map[string]string{
	"ascending":   "A-Z",
	"descending":  "Z-A",
	"oldest":      "Terlama",
	"latest":      "Terbaru",
	"popular":     "Teratas",
	"most_viewed": "Terpopuler",
	"updated":     "Terupdate",
	"text":        "Mode Teks",
}

func parsePropertyLinks(rawHTML, kind string) []domain.PropertyItem {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	items := make([]domain.PropertyItem, 0)
	seen := map[string]struct{}{}
	doc.Find("#animeList a[href*='/properties/"+kind+"/']").Each(func(_ int, anchor *goquery.Selection) {
		href, _ := anchor.Attr("href")
		id := propertyIDFromHref(href)
		title := normalizeSpace(anchor.Text())
		if id == "" || title == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		items = append(items, domain.PropertyItem{ID: id, Title: title})
	})
	return items
}

func parseDiscoveryControls(rawHTML string) ([]domain.SelectOption, string, domain.Pagination) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, "", domain.Pagination{}
	}

	options := make([]domain.SelectOption, 0)
	current := ""
	doc.Find("#filterAnime option").Each(func(_ int, option *goquery.Selection) {
		value, _ := option.Attr("value")
		order := queryValueFromURL(value, "order_by")
		label := normalizeSpace(option.Text())
		if order == "" || label == "" {
			return
		}
		options = append(options, domain.SelectOption{Value: order, Label: firstNonEmpty(label, discoveryOrderLabels[order])})
		if _, selected := option.Attr("selected"); selected || current == "" && strings.EqualFold(order, "ascending") {
			if _, selected := option.Attr("selected"); selected {
				current = order
			}
		}
	})

	pagination := domain.Pagination{}
	if node := doc.Find(".product__pagination .current-page").First(); node.Length() > 0 {
		pagination.CurrentPage = parseInteger(node.Text())
	}
	lastPage := 0
	doc.Find(".product__pagination a.page__link").Each(func(_ int, anchor *goquery.Selection) {
		href, _ := anchor.Attr("href")
		page := parseInteger(queryValueFromURL(href, "page"))
		if page > lastPage {
			lastPage = page
		}
		if page > pagination.CurrentPage && (pagination.NextPage == 0 || page < pagination.NextPage) {
			pagination.NextPage = page
		}
		if page < pagination.CurrentPage && page > pagination.PrevPage {
			pagination.PrevPage = page
		}
	})
	pagination.TotalPages = lastPage
	return options, current, pagination
}

func parseScheduleCardsHTML(rawHTML string) []domain.AnimeCard {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	scheduleEpisode := map[string]string{}
	doc.Find("input.actual-schedule-ep-1480-real, input[class*='actual-schedule-ep-'][class$='-real']").Each(func(_ int, input *goquery.Selection) {
		class, _ := input.Attr("class")
		animeID := scheduleAnimeID(class)
		value, _ := input.Attr("value")
		if animeID != "" && strings.TrimSpace(value) != "" {
			scheduleEpisode[animeID] = strings.TrimSpace(value)
		}
	})

	items := make([]domain.AnimeCard, 0)
	doc.Find("#animeList .product__item").Each(func(_ int, item *goquery.Selection) {
		card := parseAnimeCardAnchor(item)
		if card.ID == "" || card.Title == "" {
			return
		}
		animeID := animeIDFromSlug(strings.TrimPrefix(card.ID, Name+":"))
		if next := strings.TrimSpace(scheduleEpisode[animeID]); next != "" {
			card.EpisodeLabel = "Ep " + next
		}
		card.StatusLabel = normalizeSpace(item.Find(".view-end li").First().Text())
		if card.StatusLabel == "" {
			card.StatusLabel = normalizeSpace(item.Find(".actual-schedule-info-" + animeID).First().Text())
		}
		card.RatingLabel = normalizeSpace(item.Find(".product__item__text ul a:nth-child(2)").Text())
		items = append(items, card)
	})
	return items
}

func parseTextModeCards(rawHTML string) []domain.AnimeCard {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	items := make([]domain.AnimeCard, 0)
	doc.Find("#animeList a.anime__list__link[href*='/anime/']").Each(func(_ int, anchor *goquery.Selection) {
		href, _ := anchor.Attr("href")
		slug := animeSlugFromHref(href)
		title := normalizeSpace(anchor.Text())
		if slug == "" || title == "" {
			return
		}
		items = append(items, domain.AnimeCard{
			ID:       catalogIDFromSlug(slug),
			Title:    title,
			Provider: Name,
		})
	})
	return items
}

func sanitizeEpisodeBadge(value string) string {
	label := normalizeSpace(strings.ReplaceAll(value, "/ ?", ""))
	label = strings.ReplaceAll(label, "/?", "")
	label = strings.ReplaceAll(label, " / ? ", "")
	label = strings.ReplaceAll(label, " / ?", "")
	return strings.TrimSpace(label)
}

func propertyIDFromHref(href string) string {
	value := strings.TrimSpace(href)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func queryValueFromURL(rawURL, key string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get(key))
}

func parseInteger(value string) int {
	number, _ := strconv.Atoi(strings.TrimSpace(value))
	return number
}

func scheduleAnimeID(className string) string {
	for _, part := range strings.Fields(className) {
		if strings.HasPrefix(part, "actual-schedule-ep-") && strings.HasSuffix(part, "-real") {
			return strings.TrimSuffix(strings.TrimPrefix(part, "actual-schedule-ep-"), "-real")
		}
	}
	return ""
}
