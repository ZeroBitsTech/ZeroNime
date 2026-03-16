#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
ENV_FILE="$ROOT_DIR/.env"

SUDO=""
if [[ "${EUID:-$(id -u)}" -ne 0 ]] && command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
fi

install_docker_if_missing() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    return 0
  fi

  echo "docker / docker compose belum tersedia, mencoba install otomatis..."

  if ! command -v apt-get >/dev/null 2>&1; then
    echo "auto install hanya disiapkan untuk Ubuntu/Debian (apt-get)." >&2
    exit 1
  fi

  $SUDO apt-get update
  $SUDO apt-get install -y ca-certificates curl gnupg

  if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | $SUDO sh
  fi

  if ! docker compose version >/dev/null 2>&1; then
    $SUDO apt-get install -y docker-compose-plugin || true
  fi

  if ! command -v docker >/dev/null 2>&1; then
    echo "docker gagal terinstall" >&2
    exit 1
  fi

  if ! docker compose version >/dev/null 2>&1; then
    echo "docker compose masih tidak tersedia setelah install" >&2
    exit 1
  fi
}

install_docker_if_missing

install_runtime_tools_if_missing() {
  local missing=()

  command -v curl >/dev/null 2>&1 || missing+=("curl")
  command -v jq >/dev/null 2>&1 || missing+=("jq")

  if [[ ${#missing[@]} -eq 0 ]]; then
    return 0
  fi

  echo "tool runtime belum lengkap, install otomatis: ${missing[*]}"

  if ! command -v apt-get >/dev/null 2>&1; then
    echo "auto install tool runtime hanya disiapkan untuk Ubuntu/Debian (apt-get)." >&2
    exit 1
  fi

  $SUDO apt-get update
  $SUDO apt-get install -y "${missing[@]}"
}

install_runtime_tools_if_missing

if [[ ! -f "$ENV_FILE" ]]; then
  echo ".env tidak ditemukan di $ENV_FILE" >&2
  exit 1
fi

mkdir -p "$ROOT_DIR/data/postgres" "$ROOT_DIR/data/redis" "$ROOT_DIR/data/caddy" "$ROOT_DIR/config/caddy"

cd "$ROOT_DIR"

echo "[1/4] Build dan start container"
docker compose -f "$COMPOSE_FILE" up -d --build --remove-orphans

echo "[2/4] Status container"
docker compose -f "$COMPOSE_FILE" ps

echo "[3/4] Tunggu health backend"
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "[4/4] Health backend"
curl -fsS http://127.0.0.1:8080/health
echo
