package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr          string
	DatabaseURL         string
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
	ActiveProvider      string
	OtakudesuBaseURL    string
	KuramanimeBaseURL   string
	BrowserPath         string
	UserAgent           string
	RequestTimeout      time.Duration
	CacheTTL            time.Duration
	StreamCacheTTL      time.Duration
	ImageCacheTTL       time.Duration
	BrowserRenderBudget time.Duration
	MediaCacheFetchTTL  time.Duration
	PublicRateLimitRPM  int
	WriteRateLimitRPM   int
	MediaCacheDir       string
	MediaCacheHeadBytes int64
	MediaCacheTailBytes int64
	PredictiveCacheEnabled   bool
	PredictiveCacheDir       string
	PredictiveCacheHeadBytes int64
	PredictiveCacheTailBytes int64
	DObjectURL          string
	DObjectAccessKey    string
	DObjectSecretKey    string
	DObjectBucket       string
	DObjectRegion       string
	DObjectForcePath    bool
	DObjectAutoCreate   bool
	DObjectUseWhenReady bool
}

func Load() Config {
	return Config{
		ListenAddr:          getenv("ANIME_LISTEN_ADDR", ":8080"),
		DatabaseURL:         getenv("DATABASE_URL", "postgres://zeronime:zeronime%40123@localhost:5432/animedb?sslmode=disable"),
		RedisAddr:           getenv("REDIS_ADDR", ""),
		RedisPassword:       getenv("REDIS_PASSWORD", ""),
		RedisDB:             mustInt(getenv("REDIS_DB", "0"), 0),
		ActiveProvider:      strings.ToLower(getenv("ANIME_ACTIVE_PROVIDER", "kuramanime")),
		OtakudesuBaseURL:    strings.TrimRight(getenv("OTAKUDESU_BASE_URL", "https://otakudesu.best"), "/"),
		KuramanimeBaseURL:   strings.TrimRight(getenv("KURAMANIME_BASE_URL", "https://v17.kuramanime.ink"), "/"),
		BrowserPath:         strings.TrimSpace(getenv("ANIME_BROWSER_PATH", "")),
		UserAgent:           getenv("ANIME_UPSTREAM_USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		RequestTimeout:      mustDuration(getenv("ANIME_REQUEST_TIMEOUT", "12s"), 12*time.Second),
		CacheTTL:            mustDuration(getenv("ANIME_CACHE_TTL", "5m"), 5*time.Minute),
		StreamCacheTTL:      mustDuration(getenv("ANIME_STREAM_CACHE_TTL", "6h"), 6*time.Hour),
		ImageCacheTTL:       mustDuration(getenv("ANIME_IMAGE_CACHE_TTL", "24h"), 24*time.Hour),
		BrowserRenderBudget: mustDuration(getenv("ANIME_BROWSER_RENDER_BUDGET", "2500ms"), 2500*time.Millisecond),
		MediaCacheFetchTTL:  mustDuration(getenv("ANIME_MEDIA_CACHE_FETCH_TIMEOUT", "20s"), 20*time.Second),
		PublicRateLimitRPM:  mustInt(getenv("ANIME_PUBLIC_RATE_LIMIT_RPM", "180"), 180),
		WriteRateLimitRPM:   mustInt(getenv("ANIME_WRITE_RATE_LIMIT_RPM", "60"), 60),
		MediaCacheDir:       getenv("ANIME_MEDIA_CACHE_DIR", "./data/media-cache"),
		MediaCacheHeadBytes: mustInt64(getenv("ANIME_MEDIA_CACHE_HEAD_BYTES", "10485760"), 10*1024*1024),
		MediaCacheTailBytes: mustInt64(getenv("ANIME_MEDIA_CACHE_TAIL_BYTES", "2097152"), 2*1024*1024),
		PredictiveCacheEnabled:   mustBool(getenv("ANIME_PREDICTIVE_CACHE_ENABLED", "true"), true),
		PredictiveCacheDir:       getenv("ANIME_PREDICTIVE_CACHE_DIR", filepath.Join(os.TempDir(), "zeronime-predictive-next")),
		PredictiveCacheHeadBytes: mustInt64(getenv("ANIME_PREDICTIVE_CACHE_HEAD_BYTES", "6291456"), 6*1024*1024),
		PredictiveCacheTailBytes: mustInt64(getenv("ANIME_PREDICTIVE_CACHE_TAIL_BYTES", "1048576"), 1*1024*1024),
		DObjectURL:          strings.TrimSpace(getenv("DOBJECT_URL", "")),
		DObjectAccessKey:    strings.TrimSpace(getenv("DOBJECT_S3_ACCESS_KEY", "")),
		DObjectSecretKey:    strings.TrimSpace(getenv("DOBJECT_S3_SECRET_KEY", "")),
		DObjectBucket:       strings.TrimSpace(getenv("DOBJECT_BUCKET", "zeronime-cache")),
		DObjectRegion:       strings.TrimSpace(getenv("DOBJECT_REGION", "us-east-1")),
		DObjectForcePath:    mustBool(getenv("DOBJECT_FORCE_PATH", "true"), true),
		DObjectAutoCreate:   mustBool(getenv("DOBJECT_AUTO_CREATE", "true"), true),
		DObjectUseWhenReady: mustBool(getenv("DOBJECT_USE_WHEN_READY", "true"), true),
	}
}

func (c Config) DObjectConfigured() bool {
	return c.DObjectURL != "" && c.DObjectAccessKey != "" && c.DObjectSecretKey != "" && c.DObjectBucket != ""
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func mustInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func mustInt64(raw string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func mustDuration(raw string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func mustBool(raw string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
