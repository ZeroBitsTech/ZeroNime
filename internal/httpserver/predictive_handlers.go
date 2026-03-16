package httpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/domain"

	"github.com/gofiber/fiber/v2"
)

const predictiveWindowThrottleTTL = 30 * time.Second

func (s *server) primeStartupWindow(c *fiber.Ctx) error {
	var input struct {
		CurrentEpisodeID string   `json:"currentEpisodeId"`
		NextEpisodeIDs   []string `json:"nextEpisodeIds"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errEnvelope("invalid_payload", "Payload warmup episode tidak valid.", nil))
	}

	clientID := c.Locals("client_id").(uint)
	nextEpisodeIDs := normalizePredictiveEpisodeIDs(input.NextEpisodeIDs, 2)
	queued := len(nextEpisodeIDs)
	deduped := false

	if s.predictive == nil {
		queued = 0
	} else if queued > 0 {
		if shouldQueuePredictiveWindow(s.cache, clientID, nextEpisodeIDs, predictiveWindowThrottleTTL) {
			go s.preparePredictiveWindow(clientID, nextEpisodeIDs)
		} else {
			queued = 0
			deduped = true
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(ok(map[string]any{
		"queued":         queued,
		"currentEpisode": input.CurrentEpisodeID,
		"deduped":        deduped,
		"enabled":        s.predictive != nil,
	}, nil))
}

func (s *server) preparePredictiveWindow(clientID uint, nextEpisodeIDs []string) {
	if s.predictive == nil || len(nextEpisodeIDs) == 0 {
		return
	}

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

func normalizePredictiveEpisodeIDs(rawEpisodeIDs []string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	normalized := make([]string, 0, limit)
	seen := make(map[string]struct{}, len(rawEpisodeIDs))
	for _, episodeID := range rawEpisodeIDs {
		episodeID = strings.TrimSpace(episodeID)
		if episodeID == "" {
			continue
		}
		if _, ok := seen[episodeID]; ok {
			continue
		}
		seen[episodeID] = struct{}{}
		normalized = append(normalized, episodeID)
		if len(normalized) == limit {
			break
		}
	}

	return normalized
}

func shouldQueuePredictiveWindow(mem *cache.Cache, clientID uint, episodeIDs []string, ttl time.Duration) bool {
	if mem == nil || len(episodeIDs) == 0 {
		return false
	}

	key := fmt.Sprintf("predictive-window:%d:%s", clientID, strings.Join(episodeIDs, ","))
	if _, ok := mem.Get(key); ok {
		return false
	}
	mem.Set(key, true, ttl)
	return true
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
