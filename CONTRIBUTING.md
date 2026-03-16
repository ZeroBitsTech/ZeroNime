# Contributing

Thanks for taking the time to contribute.

## Development Setup

1. Copy the example environment file:

```bash
cp .env.example .env
```

2. Start the local dependencies:

```bash
docker compose -f docker-compose.local.yml up -d
```

3. Run the backend locally:

```bash
set -a
source .env
set +a
go run ./cmd/server
```

## Testing

Run the backend test suite before opening a pull request:

```bash
go test ./cmd/... ./internal/...
```

## Pull Requests

- Keep changes scoped and focused.
- Prefer small, reviewable commits.
- Update `README.md`, `.env.example`, or `CHANGELOG.md` when behavior or setup changes.
- Add or update tests for code changes that affect behavior.

## Commit Style

Conventional-style commit messages are preferred:

- `feat: ...`
- `fix: ...`
- `perf: ...`
- `docs: ...`
- `chore: ...`
- `ci: ...`

