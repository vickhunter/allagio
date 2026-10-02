#!/usr/bin/env bash
set -euo pipefail
umask 077
task_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$task_root"
task_state="$task_root/.local/agent-poc"
mkdir -p "$task_state"
export GO111MODULE=on GOTOOLCHAIN=local
export HOST=127.0.0.1 PORT=8086
export DATABASE_URL=postgresql://affilio:affilio_local_poc@127.0.0.1:55440/affilio_poc
export ADMIN_API_KEY="$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
export CONNECTION_ENCRYPTION_KEY="$(python3 -c 'import base64,secrets; print(base64.b64encode(secrets.token_bytes(32)).decode())')"
export POC_REPORT_PATH="$task_state/report.html"
python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
    try:
        sock.bind(("127.0.0.1",8086))
    except OSError:
        raise SystemExit("Port 8086 is in use. Stop the previous demo before restarting; the database has not been reset.")
PY
docker compose -f docker-compose.poc.yml up -d --wait
docker compose -f docker-compose.poc.yml exec -T db psql -U affilio -d affilio_poc -v ON_ERROR_STOP=1 -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO PUBLIC;' >"$task_state/setup.log" 2>&1
for task_migration in affilio-api/internal/migrations/*.sql; do
  docker compose -f docker-compose.poc.yml exec -T db psql -U affilio -d affilio_poc -v ON_ERROR_STOP=1 <"$task_migration" >>"$task_state/setup.log" 2>&1
done
cd affilio-api
go build -o "$task_state/server" ./cmd/server
go build -o "$task_state/distributoor-mcp" ./cmd/distributoor-mcp
"$task_state/server" >"$task_state/server.log" 2>&1 &
task_server_pid=$!
trap 'kill "$task_server_pid" 2>/dev/null || true' EXIT INT TERM
cd "$task_root"
python3 scripts/agent-poc.py "$task_state"
printf 'POC ready: http://127.0.0.1:8086/pilot\nStop with Ctrl+C. Demo database: docker compose -f docker-compose.poc.yml down\n'
wait "$task_server_pid"
