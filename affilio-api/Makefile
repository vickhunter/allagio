COVERAGE_MIN ?= 25

test-db-up:
	docker compose -f docker-compose.test.yml up -d

test-db-down:
	docker compose -f docker-compose.test.yml down -v

test:
	GOTOOLCHAIN=local GO111MODULE=on GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache AFFILIO_TEST_DATABASE_URL=postgresql://affilio:password@localhost:5433/affilio_test go test ./internal/handlers

ci-test:
	GOTOOLCHAIN=local GO111MODULE=on GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -coverpkg=./internal/... -covermode=atomic -coverprofile=coverage.out
	GOTOOLCHAIN=local GO111MODULE=on GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go tool cover -func=coverage.out | awk -v min=$(COVERAGE_MIN) '/^total:/{gsub("%","",$$3); if (($$3+0) < min) {printf("coverage gate failed: %.1f%% < %s%%\\n", $$3+0, min); exit 1} else {printf("coverage gate passed: %.1f%% >= %s%%\\n", $$3+0, min)}}'

status:
	../scripts/mvp_status.sh

branch-protect:
	../scripts/setup_branch_protection.sh
