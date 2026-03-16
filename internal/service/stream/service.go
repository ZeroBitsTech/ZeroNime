package stream

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/domain"
	"anime/develop/backend/internal/provider"
	"anime/develop/backend/internal/store/postgres"
)

type Service struct {
	provider provider.Provider
	cache    *cache.Cache
	store    *postgres.Store
	ttl      time.Duration
}

func New(p provider.Provider, c *cache.Cache, store *postgres.Store, ttl time.Duration) *Service {
	return &Service{provider: p, cache: c, store: store, ttl: ttl}
}

func (s *Service) Resolve(ctx context.Context, episodeID string) (domain.StreamResult, error) {
	return s.resolve(ctx, episodeID, false)
}

func (s *Service) Refresh(ctx context.Context, episodeID string) (domain.StreamResult, error) {
	return s.resolve(ctx, episodeID, true)
}

func (s *Service) Invalidate(episodeID string) {
	key := fmt.Sprintf("stream:%s", episodeID)
	s.cache.Delete(key)
	if s.store != nil {
		_ = s.store.DeleteStreamCache(episodeID)
	}
}

func (s *Service) resolve(ctx context.Context, episodeID string, forceRefresh bool) (domain.StreamResult, error) {
	key := fmt.Sprintf("stream:%s", episodeID)
	now := time.Now()
	if !forceRefresh {
		if raw, ok := s.cache.Get(key); ok {
			if value, ok := raw.(domain.StreamResult); ok {
				if !resultIsExpired(now, value) {
					return value, nil
				}
				s.cache.Delete(key)
			}
		}
	}

	if !forceRefresh && s.store != nil {
		if value, ok, err := s.store.GetStreamCache(episodeID, time.Now()); err == nil && ok && value != nil {
			if resultIsExpired(now, *value) {
				_ = s.store.DeleteStreamCache(episodeID)
			} else {
				s.cache.Set(key, *value, streamMemoryTTL(now, *value, s.ttl))
				return *value, nil
			}
		}
	}

	candidates, err := s.provider.StreamCandidates(ctx, episodeID)
	if err != nil {
		return domain.StreamResult{}, err
	}
	candidates = normalizeCandidates(candidates)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].EstimatedPriority > candidates[j].EstimatedPriority
	})
	result := domain.StreamResult{
		EpisodeID:       episodeID,
		Candidates:      candidates,
		SelectionReason: "preferred_direct_mp4",
		Provider:        s.provider.Name(),
	}
	if selected, reason, ok := pickPreferredStream(candidates); ok {
		result.PreferredStream = &selected
		result.SelectionReason = reason
		s.persist(key, episodeID, result)
		return result, nil
	}
	for _, candidate := range candidates {
		if candidate.IsDirect {
			selected := candidate
			result.PreferredStream = &selected
			result.SelectionReason = "preferred_direct_fallback"
			s.persist(key, episodeID, result)
			return result, nil
		}
	}
	if len(candidates) > 0 {
		selected := candidates[0]
		result.PreferredStream = &selected
		result.SelectionReason = "highest_ranked_candidate"
	}
	s.persist(key, episodeID, result)
	return result, nil
}

func pickPreferredStream(candidates []domain.StreamCandidate) (domain.StreamCandidate, string, bool) {
	for _, candidate := range candidates {
		if candidate.IsDirect && candidate.Playable && strings.EqualFold(candidate.Container, "mp4") && strings.EqualFold(candidate.Quality, "720p") {
			return candidate, "preferred_720p_direct_mp4", true
		}
	}
	for _, candidate := range candidates {
		if candidate.IsDirect && candidate.Playable && strings.EqualFold(candidate.Container, "mp4") {
			return candidate, "preferred_direct_mp4", true
		}
	}
	return domain.StreamCandidate{}, "", false
}

func (s *Service) persist(cacheKey, episodeID string, result domain.StreamResult) {
	now := time.Now()
	s.cache.Set(cacheKey, result, streamMemoryTTL(now, result, s.ttl))
	if s.store != nil {
		_ = s.store.UpsertStreamCache(episodeID, result, resultExpiresAt(now, result))
	}
}

func streamMemoryTTL(now time.Time, result domain.StreamResult, fallback time.Duration) time.Duration {
	expiresAt := resultExpiresAt(now, result)
	untilExpiry := time.Until(expiresAt)
	if expiresAt.After(now) {
		untilExpiry = expiresAt.Sub(now)
	}
	if untilExpiry <= 0 {
		return time.Second
	}
	if fallback <= 0 || untilExpiry < fallback {
		return untilExpiry
	}
	return fallback
}

func resultIsExpired(now time.Time, result domain.StreamResult) bool {
	expiresAt := resultExpiresAt(now, result)
	return !expiresAt.After(now)
}

func resultExpiresAt(now time.Time, result domain.StreamResult) time.Time {
	fallback := now.AddDate(10, 0, 0)
	expiresAt := fallback
	found := false

	consider := func(rawURL string) {
		if rawURL == "" {
			return
		}
		candidateExpiry, ok := signedURLExpiresAt(rawURL)
		if !ok {
			return
		}
		candidateExpiry = candidateExpiry.Add(-5 * time.Minute)
		if !found || candidateExpiry.Before(expiresAt) {
			expiresAt = candidateExpiry
			found = true
		}
	}

	if result.PreferredStream != nil {
		consider(result.PreferredStream.URL)
	}
	for _, candidate := range result.Candidates {
		consider(candidate.URL)
	}
	return expiresAt
}

func signedURLExpiresAt(rawURL string) (time.Time, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return time.Time{}, false
	}
	query := parsed.Query()
	dateRaw := strings.TrimSpace(query.Get("X-Amz-Date"))
	expiresRaw := strings.TrimSpace(query.Get("X-Amz-Expires"))
	if dateRaw == "" || expiresRaw == "" {
		return time.Time{}, false
	}
	startAt, err := time.Parse("20060102T150405Z", dateRaw)
	if err != nil {
		return time.Time{}, false
	}
	seconds, err := strconv.Atoi(expiresRaw)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	return startAt.Add(time.Duration(seconds) * time.Second), true
}

func normalizeCandidates(candidates []domain.StreamCandidate) []domain.StreamCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].EstimatedPriority > candidates[j].EstimatedPriority
	})

	seen := make(map[string]struct{}, len(candidates))
	deduped := make([]domain.StreamCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(strings.TrimSpace(candidate.Container) + "|" + strings.TrimSpace(candidate.Quality))
		if key == "|" {
			key = strings.ToLower(candidate.URL)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, candidate)
	}

	direct := make([]domain.StreamCandidate, 0, len(deduped))
	for _, candidate := range deduped {
		if candidate.IsDirect && candidate.Playable {
			direct = append(direct, candidate)
		}
	}
	if len(direct) == 0 {
		return deduped
	}

	mp4 := make([]domain.StreamCandidate, 0, len(direct))
	for _, candidate := range direct {
		if strings.EqualFold(candidate.Container, "mp4") {
			mp4 = append(mp4, candidate)
		}
	}
	if len(mp4) > 0 {
		return mp4
	}

	return direct
}
