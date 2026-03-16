package httpserver

import (
	"context"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"

	"github.com/gofiber/fiber/v2"
)

func (s *server) primeStartupWindow(c *fiber.Ctx) error {
	var input struct {
		CurrentEpisodeID string   `json:"currentEpisodeId"`
		NextEpisodeIDs   []string `json:"nextEpisodeIds"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "Payload warmup episode tidak valid.", nil))
	}

	clientID := c.Locals("client_id").(uint)
	nextEpisodeIDs := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, episodeID := range input.NextEpisodeIDs {
		episodeID = strings.TrimSpace(episodeID)
		if episodeID == "" {
			continue
		}
		if _, ok := seen[episodeID]; ok {
			continue
		}
		seen[episodeID] = struct{}{}
		nextEpisodeIDs = append(nextEpisodeIDs, episodeID)
		if len(nextEpisodeIDs) == 2 {
			break
		}
	}

	go s.preparePredictiveWindow(clientID, nextEpisodeIDs)

	return c.Status(fiber.StatusAccepted).JSON(ok(map[string]any{
		"queued":         len(nextEpisodeIDs),
		"currentEpisode": input.CurrentEpisodeID,
	}, nil))
}

func (s *server) preparePredictiveWindow(clientID uint, nextEpisodeIDs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	episodes := make([]predictiveEpisode, 0, len(nextEpisodeIDs))
	for _, publicEpisodeID := range nextEpisodeIDs {
		internalEpisodeID, err := s.ids.ResolveEpisodeID(publicEpisodeID)
		if err != nil {
			continue
		}

		result, err := s.stream.Resolve(ctx, internalEpisodeID)
		if err != nil {
			continue
		}

		candidate := selectPredictiveCandidate(result)
		if candidate == nil {
			continue
		}

		episodes = append(episodes, predictiveEpisode{
			EpisodeID: internalEpisodeID,
			Candidate: *candidate,
		})
	}

	_ = s.predictive.UpdateWindow(ctx, clientID, episodes)
}

func selectPredictiveCandidate(result domain.StreamResult) *domain.StreamCandidate {
	if result.PreferredStream != nil && result.PreferredStream.IsDirect && result.PreferredStream.Playable {
		candidate := *result.PreferredStream
		return &candidate
	}

	var fallback *domain.StreamCandidate
	for _, candidate := range result.Candidates {
		if !candidate.IsDirect || !candidate.Playable {
			continue
		}
		if strings.Contains(strings.ToLower(candidate.Quality), "720") {
			selected := candidate
			return &selected
		}
		if fallback == nil {
			selected := candidate
			fallback = &selected
		}
	}

	return fallback
}
