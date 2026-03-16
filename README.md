# ZeroNime Backend

ZeroNime Backend is a Go service for anime catalog discovery, episode metadata, stream resolution, and playback-oriented media proxying.

The service is designed around one practical goal: make first-frame playback fast enough for browser video players even when the upstream provider is slow, inconsistent, or uses short-lived signed URLs.

This repository contains the backend only. Frontend apps, production secrets, local cache volumes, and machine-specific deployment files are intentionally excluded from the public setup.

## What This Backend Does

- Serves an `api/v2` JSON API for search, schedules, catalog pages, watchlist, history, and continue-watching.
- Resolves provider-specific episode pages into playable media candidates.
- Proxies images and media through controlled endpoints instead of exposing raw provider behavior directly to the client.
- Uses a startup media cache so browsers can begin playback without waiting for a full upstream fetch.
- Supports optional object storage backing for cache durability across restarts and deployments.

## Why The Media Cache Exists

Anime providers often serve direct MP4 files behind unstable hosts or signed URLs. In practice, the first playback request is where most latency comes from:

- the player needs MP4 metadata quickly,
- the upstream host may be slow to deliver the first bytes,
- the URL may expire between resolution and playback,
- and multiple browsers will often request overlapping byte ranges at startup.

This backend addresses that by caching only the parts of the file that matter most during startup.

## What "Cache Chunks" Means

For each episode, the backend can store two partial byte ranges:

- `head` — the first bytes of the MP4 file
- `tail` — the last bytes of the MP4 file

With the default configuration:

- `head = 10 MB`
- `tail = 2 MB`

Why both?

- The beginning of the file usually contains bytes the player needs immediately for startup.
- The end of the file often contains MP4 metadata, including the `moov` atom, which some files place near the tail instead of the head.

By caching only these two regions, the backend can satisfy the browser's early requests much faster than fetching the entire file up front.

## Cache Modes

### 1. Local-Only Cache

This is the simplest mode and works without DObject.

How it works:

- startup chunks are stored on local disk under `ANIME_MEDIA_CACHE_DIR`
- `/api/v2/media` reads from local cache first
- cache misses fall back to upstream
- if the backend restarts or the disk is cleared, the cache is rebuilt on demand

Use this mode when:

- you are developing locally
- you run a single backend instance
- you do not need cache persistence across redeploys

### 2. Local Cache + DObject Backing Store

This is the recommended mode if you want durable cache storage.

How it works:

- local disk remains the fastest read path
- DObject acts as a persistent backing store for cached startup chunks
- if local cache is empty, the backend can restore from DObject instead of re-fetching from the upstream provider
- this reduces startup penalties after container restarts or redeploys

Use this mode when:

- you want startup cache to survive restarts
- you are running in containers or ephemeral environments
- you want to reduce repeated upstream downloads over time

### 3. Predictive Next-Episode Cache

The backend also supports a short-lived local predictive cache for the next episodes in a watching session.

In practice:

- while the current episode is already playing
- the backend can prepare startup chunks for the next one or two episodes
- those predictive chunks are stored locally and rotated out as the viewing window moves

This cache is intentionally temporary and session-oriented. It is separate from the main startup cache strategy.

## Is DObject Required?

No. DObject is optional.

If all `DOBJECT_*` variables are left empty, the backend will run in local-only mode and still function correctly.

You should think of DObject as a durability layer, not a hard dependency:

- without DObject: faster setup, fewer moving parts, cache disappears with local storage
- with DObject: better cache persistence, better restart behavior, more infrastructure to manage

The public `.env.example` keeps DObject disabled by default.

## Architecture Overview

Key packages:

- `cmd/server` — HTTP server entrypoint
- `cmd/prewarm-startup` — CLI for prewarming startup cache
- `cmd/purge-startup-cache` — CLI for clearing startup cache
- `internal/config` — environment parsing and runtime configuration
- `internal/httpserver` — routes, middleware, public handlers
- `internal/provider` — provider contracts and implementations
- `internal/service/catalog` — catalog aggregation and cache-aware catalog logic
- `internal/service/stream` — stream candidate selection and stream cache behavior
- `internal/service/library` — watchlist, history, continue-watching
- `internal/store/postgres` — PostgreSQL persistence
- `internal/mediacache` — startup chunk cache implementation
- `internal/mediaproxy` — media proxy behavior for ranged playback

## API Overview

Public endpoints:

- `GET /health`
- `POST /api/v2/session/anonymous`
- `GET /api/v2/home`
- `GET /api/v2/search?q=...`
- `GET /api/v2/schedule`
- `GET /api/v2/index`
- `GET /api/v2/catalog/:catalogId`
- `GET /api/v2/catalog/:catalogId/episodes`
- `GET /api/v2/stream/:episodeId`
- `GET /api/v2/media?...`
- `GET /api/v2/image?url=...`

Endpoints requiring `X-Client-Token`:

- `GET /api/v2/watchlist`
- `POST /api/v2/watchlist`
- `DELETE /api/v2/watchlist/:catalogId`
- `GET /api/v2/history`
- `POST /api/v2/history`
- `DELETE /api/v2/history/:catalogId`
- `GET /api/v2/continue-watching`

## Requirements

- Go `1.26+`
- PostgreSQL `15+`
- Redis `7+` if you want Redis-backed cache behavior
- Chromium if your active provider requires browser rendering outside Docker

## Quick Start

1. Copy the example environment file:

```bash
cp .env.example .env
```

2. Start the local stack:

```bash
docker compose -f docker-compose.local.yml up -d --build
```

3. Smoke test:

```bash
curl -s http://127.0.0.1:8080/health
curl -s -X POST http://127.0.0.1:8080/api/v2/session/anonymous
curl -s "http://127.0.0.1:8080/api/v2/search?q=naruto"
```

## Run Without Docker

1. Start PostgreSQL and Redis yourself.
2. Copy `.env.example` to `.env` and adjust values as needed.
3. Load the environment and run the server:

```bash
set -a
source .env
set +a
go run ./cmd/server
```

## Environment Reference

The complete reference is in `.env.example`. The variables are grouped by responsibility so the setup remains easy to reason about.

### Core Runtime

- `ANIME_LISTEN_ADDR` — HTTP listen address
- `DATABASE_URL` — PostgreSQL connection string
- `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB` — optional Redis settings

### Provider Configuration

- `ANIME_ACTIVE_PROVIDER` — currently active provider implementation
- `OTAKUDESU_BASE_URL` — Otakudesu base URL
- `KURAMANIME_BASE_URL` — Kuramanime base URL
- `ANIME_BROWSER_PATH` — browser executable path for providers that need rendering

### Request, Cache, and Rate Limits

- `ANIME_REQUEST_TIMEOUT`
- `ANIME_CACHE_TTL`
- `ANIME_STREAM_CACHE_TTL`
- `ANIME_IMAGE_CACHE_TTL`
- `ANIME_BROWSER_RENDER_BUDGET`
- `ANIME_PUBLIC_RATE_LIMIT_RPM`
- `ANIME_WRITE_RATE_LIMIT_RPM`

### Media Startup Cache

- `ANIME_MEDIA_CACHE_DIR` — local storage directory for startup chunks
- `ANIME_MEDIA_CACHE_HEAD_BYTES` — number of bytes cached from the file head
- `ANIME_MEDIA_CACHE_TAIL_BYTES` — number of bytes cached from the file tail
- `ANIME_MEDIA_CACHE_FETCH_TIMEOUT` — timeout when building startup cache from upstream

### Optional DObject Backing Store

- `DOBJECT_URL`
- `DOBJECT_S3_ACCESS_KEY`
- `DOBJECT_S3_SECRET_KEY`
- `DOBJECT_BUCKET`
- `DOBJECT_REGION`
- `DOBJECT_FORCE_PATH`
- `DOBJECT_AUTO_CREATE`
- `DOBJECT_USE_WHEN_READY`

If `DOBJECT_URL`, access key, or secret key are empty, the backend stays in local-only mode.

## Reverse Proxy

An example Caddy config is available at `Caddyfile.example`.

For public deployment, copy it to `Caddyfile` and replace `api.example.com` with your own domain.

## Development Notes

- `.env`, deployment notes, local volumes, and cache directories are ignored by `.gitignore`.
- `docker-compose.local.yml` is intended for local development.
- `docker-compose.yml` is a fuller stack example with additional services such as reverse proxy.

## Testing

```bash
go test ./cmd/... ./internal/...
```

## License

No license is included by default. Add one before publishing if you want to permit reuse.
