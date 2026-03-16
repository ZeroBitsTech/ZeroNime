package stream

import (
	"testing"
	"time"

	"anime/develop/backend/internal/domain"
)

func TestPickPreferredStreamPrefers720pDirectMP4(t *testing.T) {
	t.Parallel()

	candidates := []domain.StreamCandidate{
		{
			URL:               "https://example.com/video-1080.mp4",
			Container:         "mp4",
			Quality:           "1080p",
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: 180,
		},
		{
			URL:               "https://example.com/video-720.mp4",
			Container:         "mp4",
			Quality:           "720p",
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: 170,
		},
	}

	selected, reason, ok := pickPreferredStream(candidates)
	if !ok {
		t.Fatalf("pickPreferredStream() ok = false, want true")
	}
	if selected.Quality != "720p" {
		t.Fatalf("pickPreferredStream() quality = %s, want 720p", selected.Quality)
	}
	if reason != "preferred_720p_direct_mp4" {
		t.Fatalf("pickPreferredStream() reason = %s, want preferred_720p_direct_mp4", reason)
	}
}

func TestPickPreferredStreamFallsBackWhen720pMissing(t *testing.T) {
	t.Parallel()

	candidates := []domain.StreamCandidate{
		{
			URL:               "https://example.com/video-1080.mp4",
			Container:         "mp4",
			Quality:           "1080p",
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: 180,
		},
		{
			URL:               "https://example.com/video-480.mp4",
			Container:         "mp4",
			Quality:           "480p",
			IsDirect:          true,
			Playable:          true,
			EstimatedPriority: 160,
		},
	}

	selected, reason, ok := pickPreferredStream(candidates)
	if !ok {
		t.Fatalf("pickPreferredStream() ok = false, want true")
	}
	if selected.Quality != "1080p" {
		t.Fatalf("pickPreferredStream() quality = %s, want 1080p", selected.Quality)
	}
	if reason != "preferred_direct_mp4" {
		t.Fatalf("pickPreferredStream() reason = %s, want preferred_direct_mp4", reason)
	}
}

func TestResultExpiresAtUsesSignedURLExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)
	result := domain.StreamResult{
		Candidates: []domain.StreamCandidate{
			{
				URL: "https://example.r2.cloudflarestorage.com/video.mp4?X-Amz-Date=20260316T070000Z&X-Amz-Expires=7200&X-Amz-Signature=abc",
			},
		},
	}

	expiresAt := resultExpiresAt(now, result)
	want := time.Date(2026, 3, 16, 8, 55, 0, 0, time.UTC)
	if !expiresAt.Equal(want) {
		t.Fatalf("resultExpiresAt() = %s, want %s", expiresAt, want)
	}
}

func TestResultIsExpiredForExpiredSignedURL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)
	result := domain.StreamResult{
		Candidates: []domain.StreamCandidate{
			{
				URL: "https://example.r2.cloudflarestorage.com/video.mp4?X-Amz-Date=20260315T070000Z&X-Amz-Expires=3600&X-Amz-Signature=abc",
			},
		},
	}

	if !resultIsExpired(now, result) {
		t.Fatalf("resultIsExpired() = false, want true")
	}
}

func TestResultExpiresAtFallsBackForUnsignedURL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)
	result := domain.StreamResult{
		Candidates: []domain.StreamCandidate{
			{URL: "https://anisphia.my.id/video.mp4"},
		},
	}

	expiresAt := resultExpiresAt(now, result)
	want := now.AddDate(10, 0, 0)
	if !expiresAt.Equal(want) {
		t.Fatalf("resultExpiresAt() = %s, want %s", expiresAt, want)
	}
}
