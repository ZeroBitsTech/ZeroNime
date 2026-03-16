package kuramanime

import "testing"

func TestEpisodeReleaseLabelFromPublishedAt(t *testing.T) {
	got := episodeReleaseLabelFromPublishedAt("2026-03-15T11:02:08+07:00")
	if got != "15 Maret 2026" {
		t.Fatalf("expected formatted release label, got %q", got)
	}
}

func TestEpisodeReleaseLabelFromPublishedAtInvalid(t *testing.T) {
	if got := episodeReleaseLabelFromPublishedAt("invalid"); got != "" {
		t.Fatalf("expected empty label for invalid timestamp, got %q", got)
	}
}
