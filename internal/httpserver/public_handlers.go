package httpserver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"

	"github.com/gofiber/fiber/v2"
)

func (s *server) health(c *fiber.Ctx) error {
	dbOK := s.store.Ping() == nil
	return c.JSON(ok(map[string]any{
		"service": "zeronime-api-v2",
		"db":      dbOK,
	}, nil))
}

func (s *server) ensureSession(c *fiber.Ctx) error {
	client, _, err := s.identity.Ensure(c.Get("X-Client-Token"))
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	c.Set("X-Client-Token", client.Token)
	return c.JSON(ok(map[string]any{
		"clientToken": client.Token,
		"clientId":    fmt.Sprintf("anon_%d", client.ID),
	}, nil))
}

func (s *server) home(c *fiber.Ctx) error {
	data, err := s.catalog.Home(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicHome(data), map[string]any{"provider": s.provider}))
}

func (s *server) search(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_query", "Search query must be at least 2 characters.", nil))
	}
	data, err := s.catalog.Search(c.UserContext(), q)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicAnimeCards(data)}, map[string]any{"query": q}))
}

func (s *server) schedule(c *fiber.Ctx) error {
	data, err := s.catalog.Schedule(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicSchedule(data)}, nil))
}

func (s *server) index(c *fiber.Ctx) error {
	data, err := s.catalog.Index(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicIndex(data)}, nil))
}

func (s *server) propertyList(c *fiber.Ctx) error {
	kind := strings.TrimSpace(c.Params("kind"))
	data, err := s.catalog.PropertyList(c.UserContext(), kind)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicPropertyList(data), nil))
}

func (s *server) propertyCatalog(c *fiber.Ctx) error {
	kind := strings.TrimSpace(c.Params("kind"))
	propertyID := decodePathParam(c.Params("propertyId"))
	order := strings.TrimSpace(c.Query("order"))
	page := c.QueryInt("page", 1)
	data, err := s.catalog.PropertyCatalog(c.UserContext(), kind, propertyID, order, page)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicCollectionPage(data), nil))
}

func (s *server) quickCatalog(c *fiber.Ctx) error {
	kind := strings.TrimSpace(c.Params("kind"))
	order := strings.TrimSpace(c.Query("order"))
	page := c.QueryInt("page", 1)
	data, err := s.catalog.QuickCatalog(c.UserContext(), kind, order, page)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicCollectionPage(data), nil))
}

func (s *server) schedulePage(c *fiber.Ctx) error {
	day := strings.TrimSpace(c.Query("day"))
	page := c.QueryInt("page", 1)
	data, err := s.catalog.SchedulePage(c.UserContext(), day, page)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicCollectionPage(data), nil))
}

func (s *server) seasonalPopular(c *fiber.Ctx) error {
	data, err := s.catalog.SeasonalPopular(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicCollectionPage(data), nil))
}

func (s *server) catalogDetail(c *fiber.Ctx) error {
	id := decodePathParam(c.Params("catalogId"))
	internalID, err := s.ids.ResolveCatalogID(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_identifier", "Catalog identifier is invalid.", nil))
	}
	data, err := s.catalog.Catalog(c.UserContext(), internalID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(s.ids.PublicDetail(data), nil))
}

func (s *server) catalogEpisodes(c *fiber.Ctx) error {
	id := decodePathParam(c.Params("catalogId"))
	internalID, err := s.ids.ResolveCatalogID(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_identifier", "Catalog identifier is invalid.", nil))
	}
	data, err := s.catalog.Episodes(c.UserContext(), internalID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	return c.JSON(ok(map[string]any{"catalogId": s.ids.PublicCatalogID(internalID), "items": s.ids.PublicEpisodes(data)}, nil))
}

func (s *server) streamResolve(c *fiber.Ctx) error {
	id := decodePathParam(c.Params("episodeId"))
	internalID, err := s.ids.ResolveEpisodeID(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_identifier", "Episode identifier is invalid.", nil))
	}
	data, err := s.stream.Resolve(c.UserContext(), internalID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("provider_failed", err.Error(), nil))
	}
	response := cloneStreamResult(s.ids.PublicStream(data))
	baseURL := c.BaseURL()
	for index := range response.Candidates {
		if !response.Candidates[index].IsDirect {
			continue
		}
		response.Candidates[index].URL = proxyMediaURL(
			baseURL,
			response.EpisodeID,
			response.Candidates[index].URL,
			response.Candidates[index].Container,
		)
	}
	if response.PreferredStream != nil && response.PreferredStream.IsDirect {
		selected := *response.PreferredStream
		selected.URL = proxyMediaURL(baseURL, response.EpisodeID, selected.URL, selected.Container)
		response.PreferredStream = &selected
	}
	return c.JSON(ok(response, map[string]any{"provider": s.provider}))
}

func (s *server) imageRoute(c *fiber.Ctx) error {
	rawURL := c.Query("url")
	if strings.TrimSpace(rawURL) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_image_url", "Image URL is required.", nil))
	}
	if !s.image.Allowed(rawURL) {
		return c.Status(fiber.StatusForbidden).JSON(errEnvelope("image_host_not_allowed", "Image host is not allowed.", nil))
	}
	body, contentType, err := s.image.Fetch(rawURL)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("image_fetch_failed", err.Error(), nil))
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderCacheControl, fmt.Sprintf("public, max-age=%d", int(s.cfg.ImageCacheTTL.Seconds())))
	return c.Send(body)
}

func (s *server) mediaRoute(c *fiber.Ctx) error {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_media_url", "Media URL is required.", nil))
	}
	if !s.media.Allowed(rawURL) {
		return c.Status(fiber.StatusForbidden).JSON(errEnvelope("media_host_not_allowed", "Media host is not allowed.", nil))
	}
	internalEpisodeID := ""
	if episodeID := strings.TrimSpace(c.Query("episodeId")); episodeID != "" {
		internalEpisodeID = episodeID
		if resolvedEpisodeID, resolveErr := s.ids.ResolveEpisodeID(episodeID); resolveErr == nil {
			internalEpisodeID = resolvedEpisodeID
		}
		cached, ok, err := s.startup.TryServeRange(c.UserContext(), internalEpisodeID, rawURL, c.Get("Range"))
		if err == nil && ok && cached != nil {
			for header, values := range cached.Headers {
				if len(values) > 0 {
					c.Set(header, values[0])
				}
			}
			c.Status(cached.StatusCode)
			return c.Send(cached.Body)
		}
		cached, ok, err = s.predictive.TryServeRange(c.UserContext(), internalEpisodeID, rawURL, c.Get("Range"))
		if err == nil && ok && cached != nil {
			for header, values := range cached.Headers {
				if len(values) > 0 {
					c.Set(header, values[0])
				}
			}
			c.Status(cached.StatusCode)
			return c.Send(cached.Body)
		}
	}

	buffered, resp, fetchDuration, err := s.mediaFetch.Fetch(c.UserContext(), rawURL, c.Get("Range"))
	if err != nil {
		refreshedResp, refreshErr := s.retryMediaFetch(c, rawURL, c.Get("container"))
		if refreshErr != nil {
			return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("media_fetch_failed", err.Error(), nil))
		}
		resp = refreshedResp
		buffered = nil
		fetchDuration = 0
	}
	if internalEpisodeID != "" && s.mediaFetch.ShouldRefreshAfterFetch(c.Get("Range"), fetchDuration) {
		s.refreshStreamAsync(internalEpisodeID)
	}

	if buffered != nil {
		for header, values := range buffered.Headers {
			if len(values) > 0 {
				c.Set(header, values[0])
			}
		}

		contentType := buffered.Headers.Get(fiber.HeaderContentType)
		if contentType == "" || contentType == "application/octet-stream" {
			switch strings.ToLower(strings.TrimSpace(c.Query("container"))) {
			case "mkv":
				contentType = "video/x-matroska"
			default:
				contentType = "video/mp4"
			}
		}
		c.Set(fiber.HeaderContentType, contentType)
		c.Status(buffered.StatusCode)
		return c.Send(buffered.Body)
	}

	for _, header := range []string{
		fiber.HeaderAcceptRanges,
		fiber.HeaderCacheControl,
		fiber.HeaderContentLength,
		fiber.HeaderContentRange,
		fiber.HeaderETag,
		fiber.HeaderLastModified,
	} {
		if value := resp.Header.Get(header); value != "" {
			c.Set(header, value)
		}
	}

	contentType := resp.Header.Get(fiber.HeaderContentType)
	if contentType == "" || contentType == "application/octet-stream" {
		switch strings.ToLower(strings.TrimSpace(c.Query("container"))) {
		case "mkv":
			contentType = "video/x-matroska"
		default:
			contentType = "video/mp4"
		}
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Status(resp.StatusCode)
	c.Context().SetBodyStreamWriter(func(writer *bufio.Writer) {
		defer resp.Body.Close()
		_, _ = io.Copy(writer, resp.Body)
	})
	return nil
}

func (s *server) refreshStreamAsync(episodeID string) {
	if strings.TrimSpace(episodeID) == "" {
		return
	}

	cacheKey := "stream-refresh:" + episodeID
	if _, ok := s.cache.Get(cacheKey); ok {
		return
	}
	s.cache.Set(cacheKey, true, startupRefreshThrottleTTL)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		s.stream.Invalidate(episodeID)
		_, _ = s.stream.Refresh(ctx, episodeID)
	}()
}

func cloneStreamResult(result domain.StreamResult) domain.StreamResult {
	cloned := result
	if len(result.Candidates) > 0 {
		cloned.Candidates = append(make([]domain.StreamCandidate, 0, len(result.Candidates)), result.Candidates...)
	}
	if result.PreferredStream != nil {
		selected := *result.PreferredStream
		cloned.PreferredStream = &selected
	}
	return cloned
}

func (s *server) retryMediaFetch(c *fiber.Ctx, rawURL, container string) (*http.Response, error) {
	episodeID := strings.TrimSpace(c.Query("episodeId"))
	if episodeID == "" {
		return nil, fmt.Errorf("episode id missing")
	}

	internalEpisodeID, err := s.ids.ResolveEpisodeID(episodeID)
	if err != nil {
		return nil, err
	}

	s.stream.Invalidate(internalEpisodeID)
	refreshed, err := s.stream.Refresh(c.UserContext(), internalEpisodeID)
	if err != nil {
		return nil, err
	}

	candidate := refreshed.PreferredStream
	if candidate == nil {
		candidate = selectCandidateByContainer(refreshed.Candidates, container)
	}
	if candidate == nil {
		return nil, fmt.Errorf("no refreshed candidate available")
	}

	return s.media.Fetch(c.UserContext(), candidate.URL, c.Get("Range"))
}

func proxyMediaURL(baseURL, episodeID, rawURL, container string) string {
	if isLocalMediaURL(rawURL) {
		return rawURL
	}
	return fmt.Sprintf(
		"%s/api/v2/media?url=%s&container=%s&episodeId=%s",
		baseURL,
		url.QueryEscape(rawURL),
		url.QueryEscape(container),
		url.QueryEscape(episodeID),
	)
}

func selectCandidateByContainer(candidates []domain.StreamCandidate, container string) *domain.StreamCandidate {
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.Container, container) {
			selected := candidate
			return &selected
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	selected := candidates[0]
	return &selected
}

func isLocalMediaURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	if parsed.Path != "/api/v2/media" {
		return false
	}
	host := parsed.Hostname()
	return host == "" || host == "localhost" || host == "127.0.0.1" || net.ParseIP(host) != nil
}
