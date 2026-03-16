#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE_DIR_DEFAULT="/opt/anime-backend"

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

echo "Deploy Anime Backend ke VPS"
echo

SERVER_IP="$(prompt 'IP server')"
SSH_USER="$(prompt 'Username SSH' 'root')"
REMOTE_DIR="$(prompt 'Folder target di server' "$REMOTE_DIR_DEFAULT")"

echo
read -r -s -p "Password SSH (boleh kosong kalau pakai key): " SSH_PASSWORD
echo

if [[ -z "$SERVER_IP" || -z "$SSH_USER" || -z "$REMOTE_DIR" ]]; then
  echo "IP, username, dan folder target wajib diisi" >&2
  exit 1
fi

SSH_BASE=(ssh -o StrictHostKeyChecking=accept-new)
SCP_BASE=(scp -o StrictHostKeyChecking=accept-new)

if [[ -n "$SSH_PASSWORD" ]]; then
  if command -v sshpass >/dev/null 2>&1; then
    SSH_BASE=(sshpass -p "$SSH_PASSWORD" ssh -o StrictHostKeyChecking=accept-new)
    SCP_BASE=(sshpass -p "$SSH_PASSWORD" scp -o StrictHostKeyChecking=accept-new)
  else
    echo "sshpass tidak ditemukan. Lanjut pakai ssh/scp biasa, jadi password akan diminta ulang oleh sistem." >&2
  fi
fi

echo
echo "[1/5] Buat folder target di server"
"${SSH_BASE[@]}" "$SSH_USER@$SERVER_IP" "mkdir -p '$REMOTE_DIR'"

echo "[2/5] Upload project backend"
"${SCP_BASE[@]}" -r "$ROOT_DIR/"* "$SSH_USER@$SERVER_IP:$REMOTE_DIR/"
if [[ -f "$ROOT_DIR/.env" ]]; then
  "${SCP_BASE[@]}" "$ROOT_DIR/.env" "$SSH_USER@$SERVER_IP:$REMOTE_DIR/.env"
fi

echo "[3/5] Set permission script"
"${SSH_BASE[@]}" "$SSH_USER@$SERVER_IP" "chmod +x '$REMOTE_DIR/scripts/deploy.sh' '$REMOTE_DIR/scripts/create_device.sh' '$REMOTE_DIR/scripts/test_all.sh' '$REMOTE_DIR/scripts/deploy2server.sh' 2>/dev/null || true"

echo "[4/5] Jalankan deploy di server"
"${SSH_BASE[@]}" "$SSH_USER@$SERVER_IP" "cd '$REMOTE_DIR' && ./scripts/deploy.sh"

echo "[5/5] Selesai"
echo "Server: $SSH_USER@$SERVER_IP"
echo "Folder: $REMOTE_DIR"
