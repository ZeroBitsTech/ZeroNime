package kuramanime

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

var sourceQualityPattern = regexp.MustCompile(`(\d{3,4})`)

func (p *Provider) StreamCandidates(ctx context.Context, episodeID string) ([]domain.StreamCandidate, error) {
	_, slug, err := domain.ParseQualifiedID(episodeID)
	if err != nil {
		return nil, err
	}

	mainPath := "/anime/" + slug
	ref := p.baseURL + mainPath
	secret, err := p.fetchSecret(ctx, strings.TrimPrefix(mainPath, "/"), ref)
	if err != nil {
		return nil, err
	}

	playerPath := fmt.Sprintf("%s?Ub3BzhijicHXZdv=%s&C2XAPerzX1BM7V9=kuramadrive&page=1", mainPath, secret)
	doc, err := p.getDoc(ctx, playerPath, ref)
	if err != nil {
		return nil, err
	}

	html, err := doc.Html()
	if err != nil {
		return nil, err
	}

	candidates, err := ExtractStreamCandidates(html)
	if err == nil {
		return candidates, nil
	}

	renderedHTML, renderErr := p.renderPlayerHTML(ctx, p.baseURL+"/"+strings.TrimLeft(playerPath, "/"))
	if renderErr != nil {
		return nil, err
	}
	return ExtractStreamCandidates(renderedHTML)
}

func ExtractStreamCandidates(html string) ([]domain.StreamCandidate, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	candidates := make([]domain.StreamCandidate, 0)
	doc.Find("#player").Each(func(_ int, video *goquery.Selection) {
		rawURL, _ := video.Attr("src")
		if strings.TrimSpace(rawURL) == "" {
			return
		}

		quality := qualityLabel(qualityFromURL(rawURL))
		candidates = append(candidates, domain.StreamCandidate{
			URL:               strings.TrimSpace(rawURL),
			Container:         "mp4",
			Quality:           quality,
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: streamPriority(quality),
			Label:             "kuramadrive",
		})
	})

	doc.Find("#player source").Each(func(_ int, source *goquery.Selection) {
		rawURL, _ := source.Attr("src")
		if strings.TrimSpace(rawURL) == "" {
			return
		}

		rawQuality, _ := source.Attr("size")
		if rawQuality == "" {
			rawQuality = qualityFromURL(rawURL)
		}
		quality := qualityLabel(rawQuality)

		candidates = append(candidates, domain.StreamCandidate{
			URL:               strings.TrimSpace(rawURL),
			Container:         "mp4",
			Quality:           quality,
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: streamPriority(quality),
			Label:             "kuramadrive",
		})
	})

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no stream candidates found")
	}

	sortStreamCandidates(candidates)
	return dedupeCandidates(candidates), nil
}

func (p *Provider) fetchSecret(ctx context.Context, routePath, ref string) (string, error) {
	text, err := p.getText(ctx, "/assets/Ks6sqSgloPTlHMl.txt", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func qualityFromURL(rawURL string) string {
	match := sourceQualityPattern.FindStringSubmatch(rawURL)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func dedupeCandidates(items []domain.StreamCandidate) []domain.StreamCandidate {
	seen := make(map[string]struct{}, len(items))
	result := make([]domain.StreamCandidate, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(item.Quality + "|" + item.URL)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		left := extractQualityNumber(result[i].Quality)
		right := extractQualityNumber(result[j].Quality)
		return left > right
	})
	return result
}

func extractQualityNumber(label string) int {
	match := sourceQualityPattern.FindStringSubmatch(label)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(match[1])
	return value
}
