package httpserver

import "github.com/gofiber/fiber/v2"

func (s *server) watchlist(c *fiber.Ctx) error {
	items, err := s.library.ListWatchlist(c.Locals("client_id").(uint))
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicWatchlist(items)}, nil))
}

func (s *server) saveWatchlist(c *fiber.Ctx) error {
	var input struct {
		CatalogID string `json:"catalogId"`
	}
	if err := c.BodyParser(&input); err != nil || input.CatalogID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId is required.", nil))
	}
	internalCatalogID, err := s.ids.ResolveCatalogID(input.CatalogID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId is invalid.", nil))
	}
	if err := s.library.SaveWatchlist(c.UserContext(), c.Locals("client_id").(uint), internalCatalogID); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(errEnvelope("watchlist_failed", err.Error(), nil))
	}
	return c.Status(fiber.StatusCreated).JSON(ok(map[string]any{"catalogId": input.CatalogID}, nil))
}

func (s *server) deleteWatchlist(c *fiber.Ctx) error {
	internalCatalogID, err := s.ids.ResolveCatalogID(decodePathParam(c.Params("catalogId")))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId is invalid.", nil))
	}
	if err := s.library.DeleteWatchlist(c.Locals("client_id").(uint), internalCatalogID); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"deleted": true}, nil))
}

func (s *server) history(c *fiber.Ctx) error {
	items, err := s.library.ListHistory(c.Locals("client_id").(uint))
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicHistory(items)}, nil))
}

func (s *server) saveHistory(c *fiber.Ctx) error {
	var input struct {
		CatalogID       string `json:"catalogId"`
		EpisodeID       string `json:"episodeId"`
		PositionSeconds int    `json:"positionSeconds"`
		Title           string `json:"title"`
		CoverImage      string `json:"coverImage"`
		Provider        string `json:"provider"`
	}
	if err := c.BodyParser(&input); err != nil || input.CatalogID == "" || input.EpisodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId and episodeId are required.", nil))
	}
	internalCatalogID, err := s.ids.ResolveCatalogID(input.CatalogID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId is invalid.", nil))
	}
	internalEpisodeID, err := s.ids.ResolveEpisodeID(input.EpisodeID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "episodeId is invalid.", nil))
	}
	if err := s.library.SaveHistory(
		c.Locals("client_id").(uint),
		internalCatalogID,
		internalEpisodeID,
		input.PositionSeconds,
		input.Title,
		input.CoverImage,
		input.Provider,
	); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"catalogId": input.CatalogID, "episodeId": input.EpisodeID}, nil))
}

func (s *server) deleteHistory(c *fiber.Ctx) error {
	internalCatalogID, err := s.ids.ResolveCatalogID(decodePathParam(c.Params("catalogId")))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "catalogId is invalid.", nil))
	}
	if err := s.library.DeleteHistory(c.Locals("client_id").(uint), internalCatalogID); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"deleted": true}, nil))
}

func (s *server) continueWatching(c *fiber.Ctx) error {
	items, err := s.library.ContinueWatching(c.UserContext(), c.Locals("client_id").(uint))
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(errEnvelope("storage_unavailable", "Database unavailable.", nil))
	}
	return c.JSON(ok(map[string]any{"items": s.ids.PublicContinueWatching(items)}, nil))
}
