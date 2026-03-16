package httpserver

import (
	"fmt"

	"anime/develop/backend/internal/cache"
	"anime/develop/backend/internal/config"
	"anime/develop/backend/internal/imageproxy"
	"anime/develop/backend/internal/mediacache"
	"anime/develop/backend/internal/mediaproxy"
	"anime/develop/backend/internal/provider"
	"anime/develop/backend/internal/provider/kuramanime"
	"anime/develop/backend/internal/provider/otakudesu"
	"anime/develop/backend/internal/service/catalog"
	"anime/develop/backend/internal/service/identifiers"
	"anime/develop/backend/internal/service/identity"
	"anime/develop/backend/internal/service/library"
	"anime/develop/backend/internal/service/stream"
	"anime/develop/backend/internal/store/postgres"
)

type server struct {
	cfg       config.Config
	provider  string
	store     *postgres.Store
	catalog   *catalog.Service
	ids       *identifiers.Service
	identity  *identity.Service
	library   *library.Service
	stream    *stream.Service
	startup   *mediacache.Service
	predictive *predictiveStartupCache
	image     *imageproxy.Proxy
	media     *mediaproxy.Proxy
	mediaFetch *mediaFetcher
	cache     *cache.Cache
	publicRPM *rateCounter
	writeRPM  *rateCounter
}

func Run() error {
	cfg := config.Load()
	store, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	registry := provider.NewRegistry(
		otakudesu.New(cfg.OtakudesuBaseURL, cfg.UserAgent, cfg.RequestTimeout),
		kuramanime.New(cfg.KuramanimeBaseURL, cfg.UserAgent, cfg.RequestTimeout, cfg.BrowserRenderBudget, cfg.BrowserPath),
	)
	activeProvider, err := registry.Select(cfg.ActiveProvider)
	if err != nil {
		return fmt.Errorf("select provider: %w", err)
	}
	memCache := cache.New(cfg.CacheTTL)
	mediaProxy := mediaproxy.New()
	blobStore, err := mediacache.NewBlobStore(cfg)
	if err != nil {
		return fmt.Errorf("configure media cache store: %w", err)
	}
	var predictive *predictiveStartupCache
	if cfg.PredictiveCacheEnabled {
		predictive = newPredictiveStartupCache(
			cfg.PredictiveCacheDir,
			mediaProxy,
			cfg.PredictiveCacheHeadBytes,
			cfg.PredictiveCacheTailBytes,
			cfg.MediaCacheFetchTTL,
		)
	}
	s := &server{
		cfg:       cfg,
		provider:  activeProvider.Name(),
		store:     store,
		catalog:   catalog.New(activeProvider, memCache, store),
		ids:       identifiers.New(store),
		identity:  identity.New(store),
		library:   library.New(store, activeProvider),
		stream:    stream.New(activeProvider, memCache, store, cfg.StreamCacheTTL),
		startup:   mediacache.New(store, blobStore, mediaProxy, cfg.MediaCacheHeadBytes, cfg.MediaCacheTailBytes, cfg.MediaCacheFetchTTL),
		predictive: predictive,
		image:     imageproxy.New(cfg.RequestTimeout),
		media:     mediaProxy,
		mediaFetch: newMediaFetcher(memCache, mediaProxy),
		cache:     memCache,
		publicRPM: newRateCounter(),
		writeRPM:  newRateCounter(),
	}
	return s.routes().Listen(cfg.ListenAddr)
}
