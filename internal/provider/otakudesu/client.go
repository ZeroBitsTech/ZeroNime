package otakudesu

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

const Name = "otakudesu"

type Provider struct {
	baseURL   string
	userAgent string
	client    *http.Client
}

func New(baseURL, userAgent string, timeout time.Duration) *Provider {
	return &Provider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: userAgent,
		client:    &http.Client{Timeout: timeout},
	}
}

func (p *Provider) Name() string { return Name }

func (p *Provider) getDoc(ctx context.Context, path string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

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

func toCatalogID(slug string) string {
	return domain.NewCatalogID(Name, slug)
}

func slugFromHref(href, prefix string) string {
	slug := strings.TrimSpace(href)
	if parsed, err := url.Parse(slug); err == nil && parsed.Path != "" {
		slug = parsed.Path
	}
	slug = strings.TrimPrefix(slug, prefix)
	slug = strings.TrimPrefix(slug, "/")
	slug = strings.Trim(slug, "/")
	return slug
}

func parseEpisodeNumber(label string) int {
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindAllStringSubmatch(label, -1)
	if len(matches) == 0 {
		return 0
	}
	value, _ := strconv.Atoi(matches[len(matches)-1][1])
	return value
}

func cardFromSelection(sel *goquery.Selection) domain.AnimeCard {
	title := strings.TrimSpace(sel.Find(".jdlflm").Text())
	if title == "" {
		title = strings.TrimSpace(sel.Find("h2 a").Text())
	}
	href, _ := sel.Find("a").Attr("href")
	if href == "" {
		href, _ = sel.Find("h2 a").Attr("href")
	}
	slug := slugFromHref(href, "/anime/")
	cover, _ := sel.Find("img").Attr("src")
	if cover == "" {
		cover, _ = sel.Find("img").Attr("data-src")
	}
	status := strings.TrimSpace(sel.Find(".epzti").Text())
	if status == "" {
		status = strings.TrimSpace(sel.Find(".col-anime-eps").Text())
	}
	rating := strings.TrimSpace(sel.Find(".col-anime-rating").Text())

	return domain.AnimeCard{
		ID:           toCatalogID(slug),
		Title:        title,
		CoverImage:   cover,
		StatusLabel:  status,
		EpisodeLabel: status,
		RatingLabel:  rating,
		Provider:     Name,
	}
}

func detailValue(info *goquery.Selection, label string) string {
	want := strings.ToLower(label)
	value := ""
	info.Find("p").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.ToLower(strings.TrimSpace(s.Text()))
		if !strings.Contains(text, want) {
			return true
		}
		value = strings.TrimSpace(s.Find("span:nth-child(2)").Text())
		if value == "" {
			parts := strings.SplitN(strings.TrimSpace(s.Text()), ":", 2)
			if len(parts) == 2 {
				value = strings.TrimSpace(parts[1])
			}
		}
		return false
	})
	return value
}
