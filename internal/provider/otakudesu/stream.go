package otakudesu

import (
	"context"
	"fmt"
	"strings"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

func (p *Provider) StreamCandidates(ctx context.Context, episodeID string) ([]domain.StreamCandidate, error) {
	_, slug, err := domain.ParseQualifiedID(episodeID)
	if err != nil {
		return nil, err
	}
	doc, err := p.getDoc(ctx, "/episode/"+slug+"/")
	if err != nil {
		return nil, err
	}

	candidates := make([]domain.StreamCandidate, 0)
	doc.Find(".download ul li").Each(func(_ int, s *goquery.Selection) {
		quality := strings.TrimSpace(s.Find("strong").Text())
		s.Find("a").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			label := strings.TrimSpace(a.Text())
			if href == "" {
				return
			}
			container := containerFrom(quality, href)
			isDirect, directURL := normalizeDownloadURL(href)
			url := directURL
			if url == "" {
				url = href
			}
			candidates = append(candidates, domain.StreamCandidate{
				URL:               url,
				Container:         container,
				Quality:           quality,
				IsDirect:          isDirect,
				Playable:          isDirect,
				EstimatedPriority: streamPriority(container, quality, isDirect),
				Label:             label,
			})
		})
	})
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no downloadable candidates found")
	}
	return candidates, nil
}

func containerFrom(quality, href string) string {
	source := strings.ToLower(quality + " " + href)
	switch {
	case strings.Contains(source, "mp4"):
		return "mp4"
	case strings.Contains(source, "mkv"):
		return "mkv"
	default:
		return "unknown"
	}
}

func normalizeDownloadURL(href string) (bool, string) {
	switch {
	case strings.Contains(href, "pixeldrain.com/u/"):
		parts := strings.Split(strings.TrimRight(href, "/"), "/")
		return true, "https://pixeldrain.com/api/file/" + parts[len(parts)-1] + "?download"
	case strings.Contains(href, "pixeldrain.com/api/file/"):
		return true, href
	case strings.HasSuffix(strings.ToLower(href), ".mp4"):
		return true, href
	default:
		return false, href
	}
}

func streamPriority(container, quality string, isDirect bool) int {
	score := 0
	if isDirect {
		score += 100
	}
	if container == "mp4" {
		score += 50
	}
	switch {
	case strings.Contains(strings.ToLower(quality), "720"):
		score += 40
	case strings.Contains(strings.ToLower(quality), "480"):
		score += 35
	case strings.Contains(strings.ToLower(quality), "1080"):
		score += 30
	case strings.Contains(strings.ToLower(quality), "360"):
		score += 20
	default:
		score += 10
	}
	return score
}
