package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPredictiveCacheDefaults(t *testing.T) {
	t.Setenv("ANIME_PREDICTIVE_CACHE_ENABLED", "")
	t.Setenv("ANIME_PREDICTIVE_CACHE_DIR", "")
	t.Setenv("ANIME_PREDICTIVE_CACHE_HEAD_BYTES", "")
	t.Setenv("ANIME_PREDICTIVE_CACHE_TAIL_BYTES", "")

	cfg := Load()

	if !cfg.PredictiveCacheEnabled {
		t.Fatalf("PredictiveCacheEnabled = false, want true")
	}

	wantDir := filepath.Join(os.TempDir(), "zeronime-predictive-next")
	if cfg.PredictiveCacheDir != wantDir {
		t.Fatalf("PredictiveCacheDir = %q, want %q", cfg.PredictiveCacheDir, wantDir)
	}

	if cfg.PredictiveCacheHeadBytes != 6*1024*1024 {
		t.Fatalf("PredictiveCacheHeadBytes = %d, want %d", cfg.PredictiveCacheHeadBytes, 6*1024*1024)
	}

	if cfg.PredictiveCacheTailBytes != 1*1024*1024 {
		t.Fatalf("PredictiveCacheTailBytes = %d, want %d", cfg.PredictiveCacheTailBytes, 1*1024*1024)
	}
}

func TestLoadPredictiveCacheOverrides(t *testing.T) {
	t.Setenv("ANIME_PREDICTIVE_CACHE_ENABLED", "false")
	t.Setenv("ANIME_PREDICTIVE_CACHE_DIR", "/tmp/custom-predictive")
	t.Setenv("ANIME_PREDICTIVE_CACHE_HEAD_BYTES", "1234")
	t.Setenv("ANIME_PREDICTIVE_CACHE_TAIL_BYTES", "567")

	cfg := Load()

	if cfg.PredictiveCacheEnabled {
		t.Fatalf("PredictiveCacheEnabled = true, want false")
	}

	if cfg.PredictiveCacheDir != "/tmp/custom-predictive" {
		t.Fatalf("PredictiveCacheDir = %q, want %q", cfg.PredictiveCacheDir, "/tmp/custom-predictive")
	}

	if cfg.PredictiveCacheHeadBytes != 1234 {
		t.Fatalf("PredictiveCacheHeadBytes = %d, want %d", cfg.PredictiveCacheHeadBytes, 1234)
	}

	if cfg.PredictiveCacheTailBytes != 567 {
		t.Fatalf("PredictiveCacheTailBytes = %d, want %d", cfg.PredictiveCacheTailBytes, 567)
	}
}
