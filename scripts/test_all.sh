#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
DEVICE_ID="${DEVICE_ID:-demo-device-001}"
SECRET="${SECRET:-}"

if [[ -z "$SECRET" ]]; then
  echo "SECRET env is required" >&2
  exit 1
fi

sign() {
  local method="$1"
  local path="$2"
  local ts
  ts="$(date +%s)"
  local nonce
  nonce="$(openssl rand -hex 16)"
  local payload
  payload="$(printf '%s\n%s\n%s\n%s\n%s' "$method" "$path" "$ts" "$DEVICE_ID" "$nonce")"
  local sig
  sig="$(printf '%s' "$payload" | openssl dgst -sha256 -hmac "$SECRET" -binary | xxd -p -c 256)"
  printf '%s\n%s\n%s' "$ts" "$nonce" "$sig"
}

request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local auth
  mapfile -t auth < <(sign "$method" "$path")
  if [[ -n "$body" ]]; then
    curl -fsS -X "$method" "$BASE_URL$path" \
      -H "Content-Type: application/json" \
      -H "X-Device-ID: $DEVICE_ID" \
      -H "X-Timestamp: ${auth[0]}" \
      -H "X-Nonce: ${auth[1]}" \
      -H "X-Signature: ${auth[2]}" \
      -d "$body"
  else
    curl -fsS -X "$method" "$BASE_URL$path" \
      -H "X-Device-ID: $DEVICE_ID" \
      -H "X-Timestamp: ${auth[0]}" \
      -H "X-Nonce: ${auth[1]}" \
      -H "X-Signature: ${auth[2]}"
  fi
}

request_head() {
  local path="$1"
  local auth
  mapfile -t auth < <(sign HEAD "$path")
  curl -fsSI "$BASE_URL$path" \
    -H "X-Device-ID: $DEVICE_ID" \
    -H "X-Timestamp: ${auth[0]}" \
    -H "X-Nonce: ${auth[1]}" \
    -H "X-Signature: ${auth[2]}"
}

report() {
  local title="$1"
  local filter="$2"
  local json="$3"
  echo "==> $title"
  jq "$filter" <<<"$json"
  echo
}

health="$(curl -fsS "$BASE_URL/health")"
report "GET /health" '.' "$health"

home_json="$(request GET "/v1/home?page=1")"
report "GET /v1/home?page=1" '{page, ongoing_count: (.ongoing|length), baru_upload_count: (.baru_upload|length), rekomendasi_count: (.rekomendasi|length)}' "$home_json"

search_json="$(request GET "/v1/search?keyword=Naruto")"
report "GET /v1/search?keyword=Naruto" '{count: .data[0].jumlah, first: .data[0].result[0] | {id, url, judul}}' "$search_json"

series_slug="$(jq -r '.data[0].result[0].url' <<<"$search_json")"
series_id="$(jq -r '.data[0].result[0].id' <<<"$search_json")"

list_json="$(request GET "/v1/list")"
report "GET /v1/list" '{keys: (keys[0:5]), sample_a: .A[0:3]}' "$list_json"

ongoing_json="$(request GET "/v1/ongoing?page=1")"
report "GET /v1/ongoing?page=1" '{count: length, first: .[0] | {id, url, judul, lastch}}' "$ongoing_json"

baru_json="$(request GET "/v1/baru-upload?page=1")"
report "GET /v1/baru-upload?page=1" '{count: length, first: .[0] | {id, url, judul}}' "$baru_json"

rek_json="$(request GET "/v1/rekomendasi")"
report "GET /v1/rekomendasi" '{count: length, first: .[0] | {id, url, judul}}' "$rek_json"

movie_json="$(request GET "/v1/movie")"
report "GET /v1/movie" '{count: length, first: .[0]}' "$movie_json"

jadwal_json="$(request GET "/v1/jadwal")"
report "GET /v1/jadwal" '{generatedAt, first_day: .data[0] | {day, date, total: (.animeList|length)}}' "$jadwal_json"

genre_json="$(request GET "/v1/genre?page=1&url=Action")"
report "GET /v1/genre?page=1&url=Action" '{count: length, first: .[0] | {id, link, anime_name}}' "$genre_json"

detail_json="$(request GET "/v1/detail/$series_slug")"
report "GET /v1/detail/$series_slug" '{judul: .data[0].judul, total_chapter: (.data[0].chapter|length), first_chapter: .data[0].chapter[0]}' "$detail_json"

episode_slug="$(jq -r '.data[0].chapter[0].url' <<<"$detail_json")"
cover_url="$(jq -r '.data[0].cover' <<<"$detail_json")"
cover_url_encoded="$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$cover_url")"

simple_json="$(request GET "/v1/simple/$series_id")"
report "GET /v1/simple/$series_id" '.' "$simple_json"

chapter_json="$(request GET "/v1/chapter/$episode_slug")"
report "GET /v1/chapter/$episode_slug" '{episode_id: .data[0].episode_id, reso: .data[0].reso}' "$chapter_json"

stream_json="$(request GET "/v1/stream/$episode_slug")"
report "GET /v1/stream/$episode_slug" '{validated_at, stream_keys: (.data[0].streams | keys), stream_counts: (.data[0].streams | with_entries(.value = (.value|length)))}' "$stream_json"

echo "==> GET /v1/image?url=<cover>"
request_head "/v1/image?url=$cover_url_encoded" | sed -n '1,8p'
echo

history_post="$(request POST "/v1/history" "{\"anime_url\":\"$series_slug\",\"episode_id\":\"$episode_slug\",\"last_position\":321}")"
report "POST /v1/history" '.' "$history_post"

history_get="$(request GET "/v1/history")"
report "GET /v1/history" '{count: (.data|length), first: .data[0]}' "$history_get"

history_delete="$(request DELETE "/v1/history/$series_slug")"
report "DELETE /v1/history/$series_slug" '.' "$history_delete"

history_get_after_delete="$(request GET "/v1/history")"
report "GET /v1/history after delete" '{count: (.data|length)}' "$history_get_after_delete"

history_post="$(request POST "/v1/history" "{\"anime_url\":\"$series_slug\",\"episode_id\":\"$episode_slug\",\"last_position\":321}")"
report "POST /v1/history restore" '.' "$history_post"

watchlist_post="$(request POST "/v1/watchlist" "{\"anime_url\":\"$series_slug\",\"title\":\"$(jq -r '.data[0].judul' <<<"$detail_json")\",\"cover\":\"$cover_url\",\"latest_episode\":\"$episode_slug\"}")"
report "POST /v1/watchlist" '.' "$watchlist_post"

watchlist_get="$(request GET "/v1/watchlist")"
report "GET /v1/watchlist" '{count: (.data|length), first: .data[0]}' "$watchlist_get"

continue_get="$(request GET "/v1/continue-watching")"
report "GET /v1/continue-watching" '{count: (.data|length), first: .data[0]}' "$continue_get"
