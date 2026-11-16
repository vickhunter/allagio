#!/usr/bin/env bash
# Run the outbound-HTTP lint and the Go tests that do not need Postgres.
# Opt-in: scripts/install-hooks.sh wires this to pre-push. It is not part of
# the Railway build or GitHub Actions.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/affilio-api"

export GO111MODULE=on
export GOCACHE="${GOCACHE:-/tmp/go-cache}"
export GOMODCACHE="${GOMODCACHE:-/tmp/go-mod-cache}"

tool="$(mktemp)"
trap 'rm -f "$tool"' EXIT
(
	cd cmd/outboundvet
	go build -o "$tool" .
)
go vet -vettool="$tool" ./...

# These packages do not open a database. Handler tests that do are skipped
# here; the SSRF table does not need Postgres and is selected by name.
go test ./internal/outbound ./internal/preview ./internal/config ./internal/orp \
	./internal/services/... ./internal/dcp ./internal/entry ./internal/middleware \
	./internal/database ./internal/version ./cmd/server
go test ./internal/handlers -count=1 -timeout 180s -run 'TestOutboundSSRFTargetsFail|TestNotifyURLPolicy'
