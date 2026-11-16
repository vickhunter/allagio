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

# Go 1.27's vet driver exits 0 on diagnostics from x/tools older than 0.52,
# and it ignores a tool built by an older Go ("no package vetx file").
# cmd/outboundvet uses x/tools v0.52. Build that tool with the same Go that
# runs vet when that Go is 1.26 or newer; otherwise build it with Go 1.26,
# which Go 1.23 and 1.24 still accept. A printed finding fails this script
# even if go vet's own status is 0.
go_version="$(go env GOVERSION)"
echo "go: ${go_version} ($(go version))"
major_minor="$(printf '%s' "$go_version" | sed -n 's/^go\([0-9][0-9]*\)\.\([0-9][0-9]*\).*/\1 \2/p')"
major="${major_minor%% *}"
minor="${major_minor##* }"
if [ -z "$major" ] || [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 23 ]; }; then
	echo "scripts/check.sh needs Go 1.23 or newer (have ${go_version})" >&2
	exit 1
fi

# x/tools v0.52 needs Go 1.26 to compile. Go 1.27's vet driver also needs the
# tool to be built by that same Go, or it reports "no package vetx file" and
# never reaches the finding. Older drivers (1.23, 1.24) still read a tool
# built by Go 1.26 and exit 1 on a finding.
build_toolchain=go1.26.0
if [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 26 ]; }; then
	build_toolchain="$go_version"
fi

tool="$(mktemp)"
vet_log="$(mktemp)"
trap 'rm -f "$tool" "$vet_log"' EXIT
(
	cd cmd/outboundvet
	GOTOOLCHAIN="$build_toolchain" go build -o "$tool" .
)
set +e
go vet -vettool="$tool" ./... >"$vet_log" 2>&1
vet_status=$?
set -e
cat "$vet_log"
if grep -E 'raw http\.|raw outbound |raw tls\.|raw httputil\.' "$vet_log" >/dev/null; then
	echo "outbound vet reported findings (go vet exit ${vet_status})" >&2
	exit 1
fi
if [ "$vet_status" -ne 0 ]; then
	exit "$vet_status"
fi

(
	cd cmd/outboundvet
	GOTOOLCHAIN="$build_toolchain" go test -count=1 .
)

# These packages do not open a database. Handler tests that do are skipped
# here; the SSRF table does not need Postgres and is selected by name.
go test ./internal/outbound ./internal/preview ./internal/config ./internal/orp \
	./internal/services/... ./internal/dcp ./internal/entry ./internal/middleware \
	./internal/database ./internal/version ./cmd/server
go test ./internal/handlers -count=1 -timeout 180s -run 'TestOutboundSSRFTargetsFail|TestNotifyURLPolicy|TestSurfaceURLProblem'
