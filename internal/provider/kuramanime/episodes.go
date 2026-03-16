package kuramanime

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"anime/develop/backend/internal/domain"
	"github.com/PuerkitoBio/goquery"
)

var (
	rawEpisodeHrefPattern = regexp.MustCompile(`href=['"]([^'"]+/anime/[^'"]+/episode/[^'"?]+(?:\?[^'"]*)?)['"]`)
	rawEpisodePagePattern = regexp.MustCompile(`href=['"]([^'"]+/anime/[^'"]+(?:\?page=\d+))['"]`)
)

type episodeRef struct {
	Slug    string
	Path    string
	Number  int
	Display string
}

func extractEpisodeRefsFromHTML(rawHTML, animeSlug string) []episodeRef {
	seen := make(map[string]episodeRef)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err == nil {
		doc.Find("a[href*='/anime/'][href*='/episode/']").Each(func(_ int, anchor *goquery.Selection) {
			href, ok := anchor.Attr("href")
			if !ok || strings.Contains(href, "?page=") || anchor.HasClass("page__link__episode") {
				return
			}

			appendEpisodeRef(seen, href, normalizeSpace(anchor.Text()), animeSlug)
		})
	}

	for _, match := range rawEpisodeHrefPattern.FindAllStringSubmatch(rawHTML, -1) {
		if len(match) != 2 || strings.Contains(match[1], "?page=") {
			continue
		}
		appendEpisodeRef(seen, match[1], "", animeSlug)
	}

	return sortedEpisodeRefs(seen)
}

func extractEpisodePageLinks(rawHTML string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	links := make([]string, 0)
	doc.Find("a.page__link__episode[href*='?page=']").Each(func(_ int, anchor *goquery.Selection) {
		href, ok := anchor.Attr("href")
		if !ok {
			return
		}

		path := normalizeEpisodePagePath(href)
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		links = append(links, path)
	})

	for _, match := range rawEpisodePagePattern.FindAllStringSubmatch(rawHTML, -1) {
		if len(match) != 2 {
			continue
		}
		path := normalizeEpisodePagePath(match[1])
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		links = append(links, path)
	}

	sort.Slice(links, func(i, j int) bool {
		return pageValue(links[i]) < pageValue(links[j])
	})
	return links
}

func appendEpisodeRef(seen map[string]episodeRef, href, text, animeSlug string) {
	slug := episodeSlugFromHref(href)
	if slug == "" || !strings.HasPrefix(slug, animeSlug+"/episode/") {
		return
	}

	if _, exists := seen[slug]; exists {
		return
	}

	display := episodeDisplayValue(normalizeSpace(text), slug)
	seen[slug] = episodeRef{
		Slug:    slug,
		Path:    "/anime/" + slug,
		Number:  parseEpisodeNumber(display),
		Display: display,
	}
}

func mergeEpisodeRefs(base []episodeRef, extra []episodeRef) []episodeRef {
	seen := make(map[string]episodeRef, len(base)+len(extra))
	for _, item := range base {
		seen[item.Slug] = item
	}
	for _, item := range extra {
		if _, exists := seen[item.Slug]; !exists {
			seen[item.Slug] = item
		}
	}
	return sortedEpisodeRefs(seen)
}

func sortedEpisodeRefs(items map[string]episodeRef) []episodeRef {
	refs := make([]episodeRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, item)
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Number == refs[j].Number {
			return refs[i].Slug < refs[j].Slug
		}
		return refs[i].Number < refs[j].Number
	})
	return refs
}

func episodeDisplayValue(text, slug string) string {
	value := strings.TrimSpace(firstNonEmpty(episodeRangeValue(text), episodeRangeValue(slug)))
	if value == "" {
		return strconv.Itoa(parseEpisodeNumber(slug))
	}
	return value
}

func episodeRangeValue(value string) string {
	if strings.Contains(value, "/episode/") {
		parts := strings.SplitN(value, "/episode/", 2)
		if len(parts) == 2 {
			value = parts[1]
		}
	}
	match := episodeRangePattern.FindStringSubmatch(value)
	if len(match) == 0 {
		return ""
	}
	return strings.ReplaceAll(match[0], " ", "")
}

func normalizeEpisodePagePath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}

	if parsed.IsAbs() {
		return parsed.RequestURI()
	}

	if parsed.Path == "" {
		return ""
	}

	if parsed.RawQuery == "" {
		return parsed.Path
	}
	return parsed.Path + "?" + parsed.RawQuery
}

func pageValue(raw string) int {
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	value, _ := strconv.Atoi(parsed.Query().Get("page"))
	return value
}

func (p *Provider) enrichEpisodeReleaseDates(ctx context.Context, episodes []domain.Episode) []domain.Episode {
	if len(episodes) == 0 {
		return episodes
	}

	const workerCount = 8
	type job struct {
		index int
		id    string
	}

	jobs := make(chan job)
	cloned := append([]domain.Episode(nil), episodes...)
	var wg sync.WaitGroup

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				_, slug, err := domain.ParseQualifiedID(item.id)
				if err != nil {
					continue
				}
				doc, fetchErr := p.getDoc(ctx, "/anime/"+slug, p.baseURL)
				if fetchErr != nil {
					continue
				}
				publishedAt, _ := doc.Find("meta[property='article:published_time']").Attr("content")
				releasedAt := episodeReleaseLabelFromPublishedAt(publishedAt)
				if strings.TrimSpace(releasedAt) == "" {
					continue
				}
				cloned[item.index].ReleasedAt = releasedAt
			}
		}()
	}

	for index, episode := range episodes {
		jobs <- job{index: index, id: episode.ID}
	}
	close(jobs)
	wg.Wait()

	return cloned
}

func episodeReleaseLabelFromPublishedAt(value string) string {
	timestamp := strings.TrimSpace(value)
	if timestamp == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return ""
	}

	months := [...]string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}

	monthIndex := int(parsed.Month()) - 1
	if monthIndex < 0 || monthIndex >= len(months) {
		return ""
	}

	return strconv.Itoa(parsed.Day()) + " " + months[monthIndex] + " " + strconv.Itoa(parsed.Year())
}
