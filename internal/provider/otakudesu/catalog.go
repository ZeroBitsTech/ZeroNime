package otakudesu

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"anime/develop/backend/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

func (p *Provider) Home(ctx context.Context) (domain.HomeFeed, error) {
	doc, err := p.getDoc(ctx, "/")
	if err != nil {
		return domain.HomeFeed{}, err
	}

	var ongoing []domain.AnimeCard
	var recent []domain.AnimeCard
	sections := doc.Find(".rseries")
	sections.Eq(0).Find(".venz ul li").Each(func(_ int, s *goquery.Selection) {
		ongoing = append(ongoing, cardFromSelection(s))
	})
	sections.Eq(1).Find(".venz ul li").Each(func(_ int, s *goquery.Selection) {
		recent = append(recent, cardFromSelection(s))
	})

	featured := ongoing
	if len(featured) > 5 {
		featured = featured[:5]
	}

	return domain.HomeFeed{
		Featured: featured,
		Ongoing:  ongoing,
		Recent:   recent,
	}, nil
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.AnimeCard, error) {
	doc, err := p.getDoc(ctx, "/?s="+url.QueryEscape(query)+"&post_type=anime")
	if err != nil {
		return nil, err
	}
	var items []domain.AnimeCard
	doc.Find(".chivsrc li").Each(func(_ int, s *goquery.Selection) {
		items = append(items, cardFromSelection(s))
	})
	if len(items) == 0 {
		doc.Find(".venz ul li").Each(func(_ int, s *goquery.Selection) {
			items = append(items, cardFromSelection(s))
		})
	}
	return items, nil
}

func (p *Provider) Schedule(ctx context.Context) ([]domain.ScheduleDay, error) {
	doc, err := p.getDoc(ctx, "/jadwal-rilis/")
	if err != nil {
		return nil, err
	}
	var days []domain.ScheduleDay
	doc.Find(".kglist321").Each(func(_ int, s *goquery.Selection) {
		day := strings.TrimSpace(s.Find("h2").Text())
		var items []domain.AnimeCard
		s.Find("ul li a").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			slug := slugFromHref(href, "/anime/")
			items = append(items, domain.AnimeCard{
				ID:       toCatalogID(slug),
				Title:    strings.TrimSpace(a.Text()),
				Provider: Name,
			})
		})
		if day != "" {
			days = append(days, domain.ScheduleDay{Day: day, Items: items})
		}
	})
	return days, nil
}

func (p *Provider) SchedulePage(context.Context, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, fmt.Errorf("schedule page is not supported by %s", Name)
}

func (p *Provider) Index(ctx context.Context) (map[string][]domain.AnimeCard, error) {
	doc, err := p.getDoc(ctx, "/anime-list/")
	if err != nil {
		return nil, err
	}
	list := make(map[string][]domain.AnimeCard)
	doc.Find(".bariskelom").Each(func(_ int, s *goquery.Selection) {
		char := strings.TrimSpace(s.Find(".barispenz a").Text())
		if char == "" {
			return
		}
		s.Find(".jdlbar ul li a.hodebgst").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			slug := slugFromHref(href, "/anime/")
			title := strings.TrimSpace(a.Text())
			if slug == "" || title == "" {
				return
			}
			list[char] = append(list[char], domain.AnimeCard{
				ID:       toCatalogID(slug),
				Title:    title,
				Provider: Name,
			})
		})
	})
	return list, nil
}

func (p *Provider) PropertyList(context.Context, string) (domain.PropertyList, error) {
	return domain.PropertyList{}, fmt.Errorf("property list is not supported by %s", Name)
}

func (p *Provider) PropertyCatalog(context.Context, string, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, fmt.Errorf("property catalog is not supported by %s", Name)
}

func (p *Provider) QuickCatalog(context.Context, string, string, int) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, fmt.Errorf("quick catalog is not supported by %s", Name)
}

func (p *Provider) SeasonalPopular(context.Context) (domain.CollectionPage, error) {
	return domain.CollectionPage{}, fmt.Errorf("seasonal popular is not supported by %s", Name)
}

func (p *Provider) Catalog(ctx context.Context, catalogID string) (domain.AnimeDetail, error) {
	_, slug, err := domain.ParseQualifiedID(catalogID)
	if err != nil {
		return domain.AnimeDetail{}, err
	}
	doc, err := p.getDoc(ctx, "/anime/"+slug+"/")
	if err != nil {
		return domain.AnimeDetail{}, err
	}

	info := doc.Find(".infozin .infozingle")
	genres := make([]string, 0)
	info.Find("p").Each(func(_ int, s *goquery.Selection) {
		if strings.Contains(strings.ToLower(s.Text()), "genre") {
			s.Find("a").Each(func(_ int, a *goquery.Selection) {
				name := strings.TrimSpace(a.Text())
				if name != "" {
					genres = append(genres, name)
				}
			})
		}
	})

	episodes := make([]domain.Episode, 0)
	doc.Find(".episodelist").Each(func(_ int, el *goquery.Selection) {
		if strings.Contains(strings.ToLower(el.Prev().Text()), "batch") {
			return
		}
		el.Find("ul li").Each(func(_ int, s *goquery.Selection) {
			a := s.Find("a")
			href, _ := a.Attr("href")
			if !strings.Contains(href, "/episode/") {
				return
			}
			label := strings.TrimSpace(a.Text())
			episodeSlug := slugFromHref(href, "/episode/")
			episodes = append(episodes, domain.Episode{
				ID:         toCatalogID(episodeSlug),
				CatalogID:  catalogID,
				Number:     parseEpisodeNumber(label),
				Title:      label,
				Label:      label,
				ReleasedAt: strings.TrimSpace(s.Find(".zeebr").Text()),
			})
		})
	})

	sort.Slice(episodes, func(i, j int) bool { return episodes[i].Number > episodes[j].Number })

	title := detailValue(info, "judul")
	if title == "" {
		title = strings.TrimSpace(doc.Find(".infozin .infozingle p:nth-child(1) span:nth-child(2)").Text())
	}
	cover, _ := doc.Find(".fotoanime img").Attr("src")
	synopsis := strings.TrimSpace(doc.Find(".sinopc").Text())
	if synopsis == "" {
		synopsis, _ = doc.Find(`meta[name="description"]`).Attr("content")
	}
	if synopsis == "" {
		synopsis, _ = doc.Find(`meta[property="og:description"]`).Attr("content")
	}

	return domain.AnimeDetail{
		ID:         catalogID,
		Title:      title,
		Synopsis:   strings.TrimSpace(synopsis),
		CoverImage: cover,
		Genres:     genres,
		Metadata: map[string]string{
			"status":   detailValue(info, "status"),
			"rating":   detailValue(info, "skor"),
			"type":     detailValue(info, "tipe"),
			"studio":   detailValue(info, "studio"),
			"duration": detailValue(info, "durasi"),
			"release":  detailValue(info, "tanggal rilis"),
		},
		Episodes:        episodes,
		Recommendations: make([]domain.AnimeCard, 0),
		Provider:        Name,
	}, nil
}

func (p *Provider) Episodes(ctx context.Context, catalogID string) ([]domain.Episode, error) {
	detail, err := p.Catalog(ctx, catalogID)
	if err != nil {
		return nil, err
	}
	return detail.Episodes, nil
}
