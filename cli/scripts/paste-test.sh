#!/usr/bin/env bash
# Publish offer-rails to a local Verdaccio and run the README lines against
# this branch's API. The registry environment is the only difference.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CLI="$ROOT/cli"
API_DIR="$ROOT/affilio-api"
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/offer-rails-paste.XXXXXX")"
API_PORT="$(node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')"
VERD_PORT="$(node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')"
PGHOST="${OFFER_RAILS_PGHOST:-127.0.0.1}"
PGPORT="${OFFER_RAILS_PGPORT:-5432}"
PGUSER="${OFFER_RAILS_PGUSER:-affilio}"
PGPASSWORD="${OFFER_RAILS_PGPASSWORD:-password}"
export PGPASSWORD
BASE="http://127.0.0.1:${API_PORT}"
API_PID=""
VERD_PID=""
HEAD_PID=""
HEAD_PORT="$(node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})')"

cleanup() {
  if [[ -n "$API_PID" ]]; then kill "$API_PID" >/dev/null 2>&1 || true; fi
  if [[ -n "$VERD_PID" ]]; then kill "$VERD_PID" >/dev/null 2>&1 || true; fi
  if [[ -n "$HEAD_PID" ]]; then kill "$HEAD_PID" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

psql_base() {
  psql -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" "$@"
}

echo "workdir: $WORKDIR"
echo "api: $BASE"

if ! psql_base -d postgres -c 'SELECT 1' >/dev/null; then
  echo "PostgreSQL is not reachable at ${PGHOST}:${PGPORT} as ${PGUSER}" >&2
  exit 1
fi

psql_base -d postgres <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'affilio_paste' AND pid <> pg_backend_pid();
SQL
psql_base -d postgres -c 'DROP DATABASE IF EXISTS affilio_paste'
psql_base -d postgres -c 'CREATE DATABASE affilio_paste'

(
  cd "$API_DIR"
  GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go build -o "$WORKDIR/offer-rails-api" ./cmd/server
)

API_KEY_SECRET="$(openssl rand -hex 24)"
DATABASE_URL="postgresql://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/affilio_paste" \
  AUTO_MIGRATE=true \
  OFFER_RAILS_CI_SESSION=1 \
  CLAIMS_CHECK=stub \
  HOST=127.0.0.1 \
  PORT="$API_PORT" \
  API_KEY_SECRET="$API_KEY_SECRET" \
  "$WORKDIR/offer-rails-api" >"$WORKDIR/api.log" 2>&1 &
API_PID=$!

for _ in $(seq 1 60); do
  if curl -sf "$BASE/v1/health" >/dev/null; then
    break
  fi
  if ! kill -0 "$API_PID" >/dev/null 2>&1; then
    echo "API exited during startup" >&2
    cat "$WORKDIR/api.log" >&2
    exit 1
  fi
  sleep 0.5
done
curl -sf "$BASE/v1/health" >/dev/null

HEAD_PORT="$HEAD_PORT" python3 -c '
from http.server import BaseHTTPRequestHandler, HTTPServer
import os
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")
    def do_HEAD(self):
        self.send_response(200)
        self.end_headers()
    def log_message(self, fmt, *args):
        return
HTTPServer(("127.0.0.1", int(os.environ["HEAD_PORT"])), Handler).serve_forever()
' >/dev/null 2>&1 &
HEAD_PID=$!

SEED="$(node <<'NODE'
const crypto = require('crypto');
const token = crypto.randomBytes(32).toString('hex');
const hash = crypto.createHash('sha256').update(token).digest('hex');
const csrf = crypto.randomBytes(16).toString('hex');
const makerKey = crypto.randomBytes(16).toString('hex');
const subject = crypto.randomBytes(8).toString('hex');
const email = `paste-${subject}@example.com`;
process.stdout.write(JSON.stringify({ token, hash, csrf, makerKey, subject, email }));
NODE
)"
TOKEN="$(node -e 'console.log(JSON.parse(process.argv[1]).token)' "$SEED")"
HASH="$(node -e 'console.log(JSON.parse(process.argv[1]).hash)' "$SEED")"
CSRF="$(node -e 'console.log(JSON.parse(process.argv[1]).csrf)' "$SEED")"
MAKER_KEY="$(node -e 'console.log(JSON.parse(process.argv[1]).makerKey)' "$SEED")"
SUBJECT="$(node -e 'console.log(JSON.parse(process.argv[1]).subject)' "$SEED")"
EMAIL="$(node -e 'console.log(JSON.parse(process.argv[1]).email)' "$SEED")"

OFFER_ID="$(psql_base -d affilio_paste -At <<SQL
WITH maker AS (
  INSERT INTO makers (api_key, email, name, trusted)
  VALUES ('${MAKER_KEY}', '${EMAIL}', 'Paste Forge', true)
  RETURNING id
), product AS (
  INSERT INTO products (maker_id, name, website_url, product_type, primary_category, readiness_status)
  SELECT id, 'Paste Forge', 'https://pasteforge.example', 'saas', 'devtools', 'ready' FROM maker
  RETURNING id, maker_id
), offer AS (
  INSERT INTO offers (product_id, status, commission_type, commission_value, landing_url)
  SELECT id, 'draft', 'percentage', 30, 'https://pasteforge.example' FROM product
  RETURNING id
), owner AS (
  INSERT INTO owners (issuer, subject, email, name, source, maker_id, agent_only, human_login)
  SELECT 'https://offer-rails.paste', '${SUBJECT}', '${EMAIL}', 'Paste Owner', 'cli', maker_id, false, false FROM product
  RETURNING id
), session AS (
  INSERT INTO owner_sessions (token_hash, owner_id, csrf, expires_at)
  SELECT '${HASH}', id, '${CSRF}', NOW() + INTERVAL '2 hours' FROM owner
  RETURNING owner_id
)
SELECT offer.id FROM offer, session;
SQL
)"
if [[ -z "$OFFER_ID" ]]; then
  echo "seed did not create an offer" >&2
  exit 1
fi

echo "seeded offer: $OFFER_ID"

LANDING="http://127.0.0.1:${HEAD_PORT}/"

seed_untrusted() {
  local label="$1" product_name="$2" terms="$3"
  local seed token hash csrf maker_key subject email offer
  seed="$(node <<'NODE'
const crypto = require('crypto');
const token = crypto.randomBytes(32).toString('hex');
const hash = crypto.createHash('sha256').update(token).digest('hex');
const csrf = crypto.randomBytes(16).toString('hex');
const makerKey = crypto.randomBytes(16).toString('hex');
const subject = crypto.randomBytes(8).toString('hex');
const email = `review-${subject}@example.com`;
process.stdout.write(JSON.stringify({ token, hash, csrf, makerKey, subject, email }));
NODE
)"
  token="$(node -e 'console.log(JSON.parse(process.argv[1]).token)' "$seed")"
  hash="$(node -e 'console.log(JSON.parse(process.argv[1]).hash)' "$seed")"
  csrf="$(node -e 'console.log(JSON.parse(process.argv[1]).csrf)' "$seed")"
  maker_key="$(node -e 'console.log(JSON.parse(process.argv[1]).makerKey)' "$seed")"
  subject="$(node -e 'console.log(JSON.parse(process.argv[1]).subject)' "$seed")"
  email="$(node -e 'console.log(JSON.parse(process.argv[1]).email)' "$seed")"
  offer="$(psql_base -d affilio_paste -At <<SQL
WITH maker AS (
  INSERT INTO makers (api_key, email, name, trusted)
  VALUES ('${maker_key}', '${email}', '${product_name}', false)
  RETURNING id
), product AS (
  INSERT INTO products (maker_id, name, website_url, product_type, primary_category, readiness_status)
  SELECT id, '${product_name}', '${LANDING}', 'saas', 'devtools', 'ready' FROM maker
  RETURNING id, maker_id
), offer AS (
  INSERT INTO offers (product_id, status, commission_type, commission_value, landing_url, terms)
  SELECT id, 'draft', 'percentage', 20, '${LANDING}', '${terms}' FROM product
  RETURNING id
), owner AS (
  INSERT INTO owners (issuer, subject, email, name, source, maker_id, agent_only, human_login)
  SELECT 'https://offer-rails.paste', '${subject}', '${email}', '${product_name} Owner', 'cli', maker_id, false, false FROM product
  RETURNING id
), session AS (
  INSERT INTO owner_sessions (token_hash, owner_id, csrf, expires_at)
  SELECT '${hash}', id, '${csrf}', NOW() + INTERVAL '2 hours' FROM owner
  RETURNING owner_id
)
SELECT offer.id FROM offer, session;
SQL
)"
  if [[ -z "$offer" ]]; then
    echo "seed did not create the ${label} offer" >&2
    exit 1
  fi
  echo "seeded ${label} offer: $offer" >&2
  printf '%s\n' "$token" "$csrf" "$offer"
}

mapfile -t LIVE_FIELDS < <(seed_untrusted live "Paste Live" "Paste Live pays 20 percent to publishers.")
LIVE_TOKEN="${LIVE_FIELDS[0]}"
LIVE_CSRF="${LIVE_FIELDS[1]}"
LIVE_OFFER="${LIVE_FIELDS[2]}"
if [[ "${#LIVE_FIELDS[@]}" -ne 3 || -z "$LIVE_OFFER" ]]; then
  echo "live seed failed" >&2
  exit 1
fi

mapfile -t RELEVANCE_FIELDS < <(seed_untrusted relevance "Paste Review" "A handy tool for teams.")
RELEVANCE_TOKEN="${RELEVANCE_FIELDS[0]}"
RELEVANCE_CSRF="${RELEVANCE_FIELDS[1]}"
RELEVANCE_OFFER="${RELEVANCE_FIELDS[2]}"
if [[ "${#RELEVANCE_FIELDS[@]}" -ne 3 || -z "$RELEVANCE_OFFER" ]]; then
  echo "relevance seed failed" >&2
  exit 1
fi

(
  cd "$CLI"
  OFFER_RAILS_INTEGRATION=1 OFFER_RAILS_API_URL="$BASE" node --test test/integration.test.mjs
)

mkdir -p "$WORKDIR/verdaccio"
cat >"$WORKDIR/verdaccio/config.yaml" <<YAML
storage: ${WORKDIR}/verdaccio/storage
auth:
  htpasswd:
    file: ${WORKDIR}/verdaccio/htpasswd
    max_users: 1000
uplinks:
  npmjs:
    url: https://registry.npmjs.org/
    timeout: 30s
packages:
  "**":
    access: \$all
    publish: \$all
    unpublish: \$all
    proxy: npmjs
log:
  type: file
  path: ${WORKDIR}/verdaccio/verdaccio.log
  level: warn
YAML

npm install --prefix "$WORKDIR/verdaccio-app" verdaccio@5.33.0
node "$WORKDIR/verdaccio-app/node_modules/verdaccio/bin/verdaccio" \
  --listen "127.0.0.1:${VERD_PORT}" \
  --config "$WORKDIR/verdaccio/config.yaml" \
  >"$WORKDIR/verdaccio.out" 2>&1 &
VERD_PID=$!
for _ in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:${VERD_PORT}/-/ping" >/dev/null; then
    break
  fi
  if ! kill -0 "$VERD_PID" >/dev/null 2>&1; then
    echo "Verdaccio exited" >&2
    cat "$WORKDIR/verdaccio.out" >&2
    exit 1
  fi
  sleep 0.5
done
curl -sf "http://127.0.0.1:${VERD_PORT}/-/ping" >/dev/null

PUBLISH_NPMRC="$WORKDIR/publish-npmrc"
touch "$PUBLISH_NPMRC"
publish_pack() {
  (cd "$CLI" && NPM_CONFIG_USERCONFIG="$PUBLISH_NPMRC" npm publish --registry "http://127.0.0.1:${VERD_PORT}")
}
if ! publish_pack >/dev/null; then
  ADD="$(curl -sS -X PUT "http://127.0.0.1:${VERD_PORT}/-/user/org.couchdb.user:orcli" \
    -H 'Content-Type: application/json' \
    -d '{"name":"orcli","password":"orcli-pass-123","email":"orcli@example.com"}')"
  AUTH_TOKEN="$(node -e 'const j=JSON.parse(process.argv[1]); if(!j.token){console.error(process.argv[1]); process.exit(1)} console.log(j.token)' "$ADD")"
  printf 'registry=http://127.0.0.1:%s/\n//127.0.0.1:%s/:_authToken=%s\n' "$VERD_PORT" "$VERD_PORT" "$AUTH_TOKEN" >"$PUBLISH_NPMRC"
  publish_pack
fi

mapfile -t LINES < <(awk '/<!-- paste:begin -->/{p=1;next} /<!-- paste:end -->/{p=0} p && $1=="npx"{print}' "$CLI/README.md")
if [[ "${#LINES[@]}" -ne 6 ]]; then
  echo "expected 6 README lines, found ${#LINES[@]}" >&2
  printf ' %s\n' "${LINES[@]}" >&2
  exit 1
fi

CLEAN="$WORKDIR/clean"
mkdir -p "$CLEAN/home"
export HOME="$CLEAN/home"
export npm_config_cache="$WORKDIR/npm-cache"
export npm_config_registry="http://127.0.0.1:${VERD_PORT}"
export CI=true
export OFFER_RAILS_API_URL="$BASE"
unset NPM_CONFIG_USERCONFIG || true
cd "$CLEAN"

echo "=== README lines ==="
printf '%s\n' "${LINES[@]}"

bash -c "${LINES[0]}" >"$WORKDIR/login.out" 2>"$WORKDIR/login.err" &
LOGIN_PID=$!
USER_CODE=""
for _ in $(seq 1 40); do
  if grep -q '^user_code:' "$WORKDIR/login.out"; then
    USER_CODE="$(awk '/^user_code:/{print $2}' "$WORKDIR/login.out")"
    break
  fi
  if ! kill -0 "$LOGIN_PID" >/dev/null 2>&1; then
    echo "login exited before a user code" >&2
    cat "$WORKDIR/login.out" "$WORKDIR/login.err" >&2
    exit 1
  fi
  sleep 0.25
done
if [[ -z "$USER_CODE" ]]; then
  echo "no user code" >&2
  cat "$WORKDIR/login.out" "$WORKDIR/login.err" >&2
  exit 1
fi

APPROVE="$(curl -sS -w '\n%{http_code}' \
  -H "Origin: ${BASE}" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: ${CSRF}" \
  -H "Cookie: dtr_owner=${TOKEN}" \
  -d "{\"user_code\":\"${USER_CODE}\",\"decision\":\"approve\",\"scope\":\"offer\",\"project_id\":\"${OFFER_ID}\"}" \
  "${BASE}/v1/device/approve")"
APPROVE_CODE="$(printf '%s\n' "$APPROVE" | tail -n 1)"
if [[ "$APPROVE_CODE" != "200" ]]; then
  echo "device approve failed: $APPROVE" >&2
  cat "$WORKDIR/api.log" >&2
  exit 1
fi

for _ in $(seq 1 40); do
  if ! kill -0 "$LOGIN_PID" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if kill -0 "$LOGIN_PID" >/dev/null 2>&1; then
  echo "login did not finish" >&2
  cat "$WORKDIR/login.out" "$WORKDIR/login.err" >&2
  kill "$LOGIN_PID" || true
  exit 1
fi
wait "$LOGIN_PID" || true

printf 'y\n' | bash -c "${LINES[1]}" >"$WORKDIR/scan.out" 2>"$WORKDIR/scan.err" || true
bash -c "${LINES[2]}" >"$WORKDIR/list.out" 2>"$WORKDIR/list.err" || true
bash -c "${LINES[3]}" >"$WORKDIR/digest.out" 2>"$WORKDIR/digest.err" || true
bash -c "${LINES[4]}" >"$WORKDIR/undo.out" 2>"$WORKDIR/undo.err" || true
bash -c "${LINES[5]}" >"$WORKDIR/kill.out" 2>"$WORKDIR/kill.err" || true

device_login() {
  local dir="$1" token="$2" csrf="$3" offer="$4" label="$5"
  mkdir -p "$dir"
  local out="$WORKDIR/${label}-login.out"
  local err="$WORKDIR/${label}-login.err"
  (
    cd "$dir"
    bash -c "${LINES[0]}" >"$out" 2>"$err"
  ) &
  local pid=$!
  local code=""
  for _ in $(seq 1 40); do
    if grep -q '^user_code:' "$out" 2>/dev/null; then
      code="$(awk '/^user_code:/{print $2}' "$out")"
      break
    fi
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "${label} login exited before a user code" >&2
      cat "$out" "$err" >&2
      exit 1
    fi
    sleep 0.25
  done
  if [[ -z "$code" ]]; then
    echo "no ${label} user code" >&2
    cat "$out" "$err" >&2
    exit 1
  fi
  local approve approve_code
  approve="$(curl -sS -w '\n%{http_code}' \
    -H "Origin: ${BASE}" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: ${csrf}" \
    -H "Cookie: dtr_owner=${token}" \
    -d "{\"user_code\":\"${code}\",\"decision\":\"approve\",\"scope\":\"offer\",\"project_id\":\"${offer}\"}" \
    "${BASE}/v1/device/approve")"
  approve_code="$(printf '%s\n' "$approve" | tail -n 1)"
  if [[ "$approve_code" != "200" ]]; then
    echo "${label} device approve failed: $approve" >&2
    exit 1
  fi
  for _ in $(seq 1 40); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done
  if kill -0 "$pid" >/dev/null 2>&1; then
    echo "${label} login did not finish" >&2
    kill "$pid" || true
    exit 1
  fi
  wait "$pid" || true
}

run_list() {
  local dir="$1" label="$2"
  (
    cd "$dir"
    bash -c "${LINES[2]}" >"$WORKDIR/${label}-list.out" 2>"$WORKDIR/${label}-list.err"
  ) || true
}

device_login "$WORKDIR/live" "$LIVE_TOKEN" "$LIVE_CSRF" "$LIVE_OFFER" live
run_list "$WORKDIR/live" live
LIVE_STATUS="$(psql_base -d affilio_paste -At -c "SELECT status FROM offers WHERE id='${LIVE_OFFER}'")"

device_login "$WORKDIR/relevance" "$RELEVANCE_TOKEN" "$RELEVANCE_CSRF" "$RELEVANCE_OFFER" relevance
run_list "$WORKDIR/relevance" relevance
RELEVANCE_STATUS="$(psql_base -d affilio_paste -At -c "SELECT status FROM offers WHERE id='${RELEVANCE_OFFER}'")"

fail=0
grep -q 'A person approved the device code.' "$WORKDIR/login.out" || fail=1
grep -q '^listing_card:' "$WORKDIR/scan.out" || fail=1
grep -q 'status: listed' "$WORKDIR/list.out" || fail=1
grep -q 'live: true' "$WORKDIR/list.out" || fail=1
grep -q '^mode: manual$' "$WORKDIR/list.out" || fail=1
grep -q '^mode_preselected: yolo$' "$WORKDIR/list.out" || fail=1
grep -q 'in review' "$WORKDIR/list.out" && fail=1
grep -q '^mode: manual$' "$WORKDIR/digest.out" || fail=1
grep -q '^mode_preselected: yolo$' "$WORKDIR/digest.out" || fail=1
grep -q 'status: in review' "$WORKDIR/live-list.out" || fail=1
grep -q '^check: claims$' "$WORKDIR/live-list.out" || fail=1
grep -q '^message: The claims check is not live yet.$' "$WORKDIR/live-list.out" || fail=1
grep -q '^next: Submit again after this check passes.$' "$WORKDIR/live-list.out" || fail=1
grep -q '^mode: manual$' "$WORKDIR/live-list.out" || fail=1
grep -q '^mode_preselected: yolo$' "$WORKDIR/live-list.out" || fail=1
grep -q 'status: listed' "$WORKDIR/live-list.out" && fail=1
[[ "$LIVE_STATUS" == "submitted" ]] || fail=1
grep -q 'status: in review' "$WORKDIR/relevance-list.out" || fail=1
grep -q '^check: claims$' "$WORKDIR/relevance-list.out" || fail=1
grep -q '^message: The claims check is not live yet.$' "$WORKDIR/relevance-list.out" || fail=1
grep -q '^next: Submit again after this check passes.$' "$WORKDIR/relevance-list.out" || fail=1
grep -q '^mode: manual$' "$WORKDIR/relevance-list.out" || fail=1
grep -q '^mode_preselected: yolo$' "$WORKDIR/relevance-list.out" || fail=1
grep -q 'status: listed' "$WORKDIR/relevance-list.out" && fail=1
[[ "$RELEVANCE_STATUS" == "submitted" ]] || fail=1
grep -q 'summary: Paste Forge' "$WORKDIR/digest.out" || fail=1
grep -q 'live_now: true' "$WORKDIR/digest.out" || fail=1
grep -q '^undo_token:' "$WORKDIR/digest.out" || fail=1
grep -q 'status: undone' "$WORKDIR/undo.out" || fail=1
grep -q 'kind: offer' "$WORKDIR/undo.out" || fail=1
grep -q 'kill_switch: true' "$WORKDIR/kill.out" || fail=1

echo "=== paste-test output ==="
echo "-- login --"
cat "$WORKDIR/login.out"
echo "-- scan --"
cat "$WORKDIR/scan.out"
echo "-- list --"
cat "$WORKDIR/list.out"
echo "-- digest --"
cat "$WORKDIR/digest.out"
echo "-- undo --"
cat "$WORKDIR/undo.out"
echo "-- kill --"
cat "$WORKDIR/kill.out"
echo "-- untrusted maker, checks passing --"
echo "server status: ${LIVE_STATUS}"
cat "$WORKDIR/live-list.out"
echo "-- untrusted maker, stub holds before relevance --"
echo "server status: ${RELEVANCE_STATUS}"
cat "$WORKDIR/relevance-list.out"
if [[ "$fail" -ne 0 ]]; then
  echo "paste test failed" >&2
  echo "-- stderr --" >&2
  cat "$WORKDIR/login.err" "$WORKDIR/scan.err" "$WORKDIR/list.err" "$WORKDIR/digest.err" "$WORKDIR/undo.err" "$WORKDIR/kill.err" "$WORKDIR/live-login.err" "$WORKDIR/live-list.err" "$WORKDIR/relevance-login.err" "$WORKDIR/relevance-list.err" >&2
  echo "-- api log --" >&2
  tail -n 80 "$WORKDIR/api.log" >&2
  exit 1
fi

echo "=== mcp stdio ==="
if ! OFFER_RAILS_MCP_SPAWN=1 OFFER_RAILS_PASTE_DB=affilio_paste node --test "$CLI/test/mcp-stdio.test.mjs" >"$WORKDIR/mcp.out" 2>"$WORKDIR/mcp.err"; then
  echo "mcp stdio failed" >&2
  cat "$WORKDIR/mcp.out" "$WORKDIR/mcp.err" >&2
  exit 1
fi
cat "$WORKDIR/mcp.out"
echo "mcp stdio passed"
echo "paste test passed"
