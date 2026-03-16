package kuramanime

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

const Name = "kuramanime"

var (
	animePathPattern    = regexp.MustCompile(`/anime/(\d+)/([^/?#]+)`)
	episodePathPattern  = regexp.MustCompile(`/anime/(\d+)/([^/?#]+)/episode/([^/?#]+)`)
	episodeNumPattern   = regexp.MustCompile(`(\d+)`)
	episodeRangePattern = regexp.MustCompile(`\d+(?:\s*-\s*\d+)?`)
	spacePattern        = regexp.MustCompile(`\s+`)
	qualityPattern      = regexp.MustCompile(`(\d{3,4})`)
)

type Provider struct {
	baseURL             string
	userAgent           string
	client              *http.Client
	browserPath         string
	browserRenderBudget time.Duration
}

func New(baseURL, userAgent string, timeout, browserRenderBudget time.Duration, browserPath string) *Provider {
	resolvedBrowserPath := strings.TrimSpace(browserPath)
	if resolvedBrowserPath == "" {
		for _, candidate := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
			if found, err := exec.LookPath(candidate); err == nil {
				resolvedBrowserPath = found
				break
			}
		}
	}

	return &Provider{
		baseURL:             strings.TrimRight(baseURL, "/"),
		userAgent:           userAgent,
		client:              &http.Client{Timeout: timeout},
		browserPath:         resolvedBrowserPath,
		browserRenderBudget: browserRenderBudget,
	}
}

func (p *Provider) Name() string { return Name }

func (p *Provider) getDoc(ctx context.Context, path, ref string) (*goquery.Document, error) {
	return p.getDocWithUserAgent(ctx, path, ref, p.userAgent)
}

func (p *Provider) getDocWithUserAgent(ctx context.Context, path, ref, userAgent string) (*goquery.Document, error) {
	target := strings.TrimSpace(path)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = p.baseURL + "/" + strings.TrimLeft(target, "/")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userAgent) != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if strings.TrimSpace(ref) != "" {
		req.Header.Set("Referer", ref)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func (p *Provider) getText(ctx context.Context, path, ref string) (string, error) {
	target := strings.TrimSpace(path)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = p.baseURL + "/" + strings.TrimLeft(target, "/")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if strings.TrimSpace(ref) != "" {
		req.Header.Set("Referer", ref)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("provider status %d", resp.StatusCode)
	}

	body, err := goquery.NewDocumentFromReader(resp.Body)
	if err == nil {
		return body.Text(), nil
	}
	return "", err
}

func normalizeSpace(value string) string {
	return strings.TrimSpace(spacePattern.ReplaceAllString(value, " "))
}

func absoluteURL(base, href string) string {
	raw := strings.TrimSpace(href)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	resolved, err := url.Parse(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(raw, "/"))
	if err != nil {
		return raw
	}
	return resolved.String()
}

func animeSlugFromHref(href string) string {
	match := animePathPattern.FindStringSubmatch(href)
	if len(match) != 3 {
		return ""
	}
	return match[1] + "/" + match[2]
}

func episodeSlugFromHref(href string) string {
	match := episodePathPattern.FindStringSubmatch(href)
	if len(match) != 4 {
		return ""
	}
	return match[1] + "/" + match[2] + "/episode/" + match[3]
}

func animeIDFromSlug(slug string) string {
	parts := strings.Split(strings.Trim(slug, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func animeTitleSlug(slug string) string {
	parts := strings.Split(strings.Trim(slug, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func catalogIDFromSlug(slug string) string {
	return domain.NewCatalogID(Name, slug)
}

func parseEpisodeNumber(label string) int {
	match := episodeNumPattern.FindString(label)
	if match == "" {
		return 0
	}
	value, _ := strconv.Atoi(match)
	return value
}

func cardAnchor(selection *goquery.Selection) *goquery.Selection {
	if goquery.NodeName(selection) == "a" {
		return selection
	}
	if anchor := selection.Find("a").First(); anchor.Length() > 0 {
		return anchor
	}
	return selection
}

func cardRoot(selection *goquery.Selection) *goquery.Selection {
	if selection.HasClass("product__item") || selection.HasClass("product__sidebar__view__item") {
		return selection
	}
	if root := selection.Find(".product__item").First(); root.Length() > 0 {
		return root
	}
	if root := selection.Find(".product__sidebar__view__item").First(); root.Length() > 0 {
		return root
	}
	return selection
}

func parseAnimeCardAnchor(selection *goquery.Selection) domain.AnimeCard {
	anchor := cardAnchor(selection)
	root := cardRoot(selection)

	href, _ := anchor.Attr("href")
	animeSlug := animeSlugFromHref(href)
	title := normalizeSpace(firstNonEmpty(
		root.Find(".product__item__text h5").Text(),
		root.Find(".sidebar-title-h5").Text(),
		root.Find("h5").First().Text(),
	))
	cover, _ := root.Find(".product__item__pic").Attr("data-setbg")
	if cover == "" {
		cover, _ = root.Attr("data-setbg")
	}
	epLabel := normalizeSpace(root.Find(".ep").Text())
	if root.Find(".ep i.fa-star").Length() > 0 {
		epLabel = ""
	}
	epLabel = sanitizeEpisodeBadge(epLabel)
	status := normalizeSpace(root.Find(".d-none span").Text())
	quality := normalizeSpace(firstNonEmpty(
		root.Find(".view").First().Text(),
		root.Find(".product__item__text ul a:nth-child(2)").Text(),
	))

	return domain.AnimeCard{
		ID:           catalogIDFromSlug(animeSlug),
		Title:        title,
		CoverImage:   cover,
		StatusLabel:  firstNonEmpty(status, quality),
		EpisodeLabel: epLabel,
		RatingLabel:  quality,
		Provider:     Name,
	}
}

func parseEpisodeCardAnchor(selection *goquery.Selection) domain.AnimeCard {
	anchor := cardAnchor(selection)
	root := cardRoot(selection)

	href, _ := anchor.Attr("href")
	episodeSlug := episodeSlugFromHref(href)
	animeSlug := strings.Split(episodeSlug, "/episode/")[0]

	title := normalizeSpace(firstNonEmpty(
		root.Find(".product__item__text h5").Text(),
		root.Find(".sidebar-title-h5").Text(),
		root.Find("h5").First().Text(),
	))
	cover, _ := root.Find(".product__item__pic").Attr("data-setbg")
	if cover == "" {
		cover, _ = root.Attr("data-setbg")
	}
	epLabel := normalizeSpace(root.Find(".ep").Text())
	epLabel = sanitizeEpisodeBadge(epLabel)
	quality := normalizeSpace(firstNonEmpty(
		root.Find(".view").First().Text(),
		root.Find(".product__item__text ul a:nth-child(2)").Text(),
	))

	return domain.AnimeCard{
		ID:           catalogIDFromSlug(animeSlug),
		Title:        title,
		CoverImage:   cover,
		StatusLabel:  quality,
		EpisodeLabel: epLabel,
		RatingLabel:  quality,
		Provider:     Name,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortStreamCandidates(candidates []domain.StreamCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].EstimatedPriority > candidates[j].EstimatedPriority
	})
}

func qualityLabel(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(value), "p") {
		return value
	}
	return value + "p"
}

func streamPriority(quality string) int {
	score := 100
	switch quality {
	case "720p":
		score += 70
	case "480p":
		score += 60
	case "360p":
		score += 50
	default:
		score += 10
	}
	return score
}
