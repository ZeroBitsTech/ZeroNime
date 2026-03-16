#!/usr/bin/env bash
set -euo pipefail

DEVICE_ID="${1:-}"
DEVICE_NAME="${2:-}"
DB_CONTAINER="${DB_CONTAINER:-anime-db}"
DB_USER="${DB_USER:-zeronime}"
DB_NAME="${DB_NAME:-animedb}"

if [[ -z "$DEVICE_ID" || -z "$DEVICE_NAME" ]]; then
  echo "usage: $0 <device_id> <device_name>" >&2
  exit 1
fi

SECRET="$(openssl rand -hex 32)"

docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" <<SQL
INSERT INTO users (device_id, name, is_active, secret_key, created_at)
VALUES ('$DEVICE_ID', '$DEVICE_NAME', true, '$SECRET', NOW())
ON CONFLICT (device_id) DO UPDATE
SET name = EXCLUDED.name,
    is_active = EXCLUDED.is_active,
    secret_key = EXCLUDED.secret_key;
SQL

echo "device_id=$DEVICE_ID"
echo "device_name=$DEVICE_NAME"
echo "secret=$SECRET"
echo "note=secret only shown once, store it securely"
