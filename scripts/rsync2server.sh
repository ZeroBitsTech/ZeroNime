#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_FILE="$ROOT_DIR/.deploy.remote.env"
REMOTE_DIR_DEFAULT="/opt/anime-backend"
MODE="${1:-sync}"

prompt() {
  local label="$1"
  local default_value="${2:-}"
  local value
  if [[ -n "$default_value" ]]; then
    read -r -p "$label [$default_value]: " value
    printf '%s' "${value:-$default_value}"
  else
    read -r -p "$label: " value
    printf '%s' "$value"
  fi
}

load_state() {
  if [[ -f "$STATE_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$STATE_FILE"
  fi
}

save_state() {
  cat >"$STATE_FILE" <<EOF
SERVER_IP='$SERVER_IP'
SSH_USER='$SSH_USER'
REMOTE_DIR='$REMOTE_DIR'
EOF
}

require_rsync() {
  if command -v rsync >/dev/null 2>&1; then
    return 0
  fi
  echo "rsync tidak ditemukan di lokal." >&2
  exit 1
}

sync_once() {
  echo "[sync] rsync ke $SSH_USER@$SERVER_IP:$REMOTE_DIR"
  rsync -az --delete \
    --exclude '.git/' \
    --exclude 'data/' \
    --exclude 'config/' \
    --exclude '.deploy.remote.env' \
    "$ROOT_DIR/" "$SSH_USER@$SERVER_IP:$REMOTE_DIR/"

  echo "[remote] deploy.sh"
  ssh -o StrictHostKeyChecking=accept-new "$SSH_USER@$SERVER_IP" \
    "chmod +x '$REMOTE_DIR/scripts/'*.sh 2>/dev/null || true && cd '$REMOTE_DIR' && ./scripts/deploy.sh"
}

watch_sync() {
  local last_hash=""
  echo "watch mode aktif. tekan Ctrl+C untuk berhenti."
  while true; do
    local current_hash
    current_hash="$(
      find "$ROOT_DIR" \
        \( -path "$ROOT_DIR/data" -o -path "$ROOT_DIR/data/*" -o -path "$ROOT_DIR/config" -o -path "$ROOT_DIR/config/*" -o -path "$ROOT_DIR/.git" -o -path "$ROOT_DIR/.git/*" \) -prune \
        -o -type f -print0 | sort -z | xargs -0 sha1sum | sha1sum | awk '{print $1}'
    )"
    if [[ -z "$last_hash" ]]; then
      last_hash="$current_hash"
    elif [[ "$current_hash" != "$last_hash" ]]; then
      last_hash="$current_hash"
      sync_once
    fi
    sleep 2
  done
}

load_state
require_rsync

SERVER_IP="${SERVER_IP:-}"
SSH_USER="${SSH_USER:-root}"
REMOTE_DIR="${REMOTE_DIR:-$REMOTE_DIR_DEFAULT}"

if [[ -z "$SERVER_IP" ]]; then
  echo "Konfigurasi target server belum ada."
  SERVER_IP="$(prompt 'IP server')"
  SSH_USER="$(prompt 'Username SSH' "$SSH_USER")"
  REMOTE_DIR="$(prompt 'Folder target di server' "$REMOTE_DIR")"
  save_state
fi

case "$MODE" in
  sync)
    sync_once
    ;;
  watch)
    echo "Sync awal..."
    sync_once
    watch_sync
    ;;
  config)
    SERVER_IP="$(prompt 'IP server' "$SERVER_IP")"
    SSH_USER="$(prompt 'Username SSH' "$SSH_USER")"
    REMOTE_DIR="$(prompt 'Folder target di server' "$REMOTE_DIR")"
    save_state
    echo "config tersimpan di $STATE_FILE"
    ;;
  *)
    echo "usage: $0 [sync|watch|config]" >&2
    exit 1
    ;;
esac
