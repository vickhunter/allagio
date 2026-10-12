# CLAUDE.md — Affilio

## Project Overview

Affilio is an AI-powered affiliate monetization tool for WordPress. It consists of a **Go backend API** (`affilio-api/`) and a **WordPress plugin** (`affilio-wp/`). The system scans blog posts, detects existing affiliate links, recommends relevant affiliate programs, and enables one-click link insertion with FTC disclosure.

**Core flow:** Scan → Detect → Recommend → Insert → Track

## Repository Layout

```
affilio/
├── affilio-api/                    # Go backend (Gin + PostgreSQL)
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── config/                 # Env-driven configuration
│   │   ├── database/               # DB connection, queries, types
│   │   ├── handlers/               # HTTP handlers + tests
│   │   ├── middleware/             # Auth (API key, admin key) + CORS
│   │   ├── migrations/            # SQL migrations (001-009)
│   │   └── services/              # Business logic (AI, matching, scoring)
│   ├── Makefile                    # Build/test targets
│   ├── Dockerfile                  # Production image
│   ├── docker-compose.yml          # Local dev (PostgreSQL + API)
│   └── docker-compose.test.yml     # Test DB
│
├── affilio-wp/                     # WordPress plugin (PHP)
│   ├── affilio.php                 # Plugin entry point
│   ├── includes/                   # PHP classes (API, Admin, Scanner, Inserter, Tracker)
│   ├── admin/build/                # Compiled JS (dashboard, sidebar, discovery)
│   ├── admin/css/                  # Admin styles
│   ├── public/js/                  # Frontend tracking beacon
│   └── e2e/                        # Playwright E2E tests
│
├── scripts/                        # Utility scripts
├── .github/workflows/              # CI (GitHub Actions)
├── STATUS.md                       # Implementation status & changelog
├── MVP_CHECKLIST.md                # Feature checklist (P0-P4)
├── AGENTS.md                       # Agent development instructions
└── DEPLOYMENT.md                   # Production runbook (in affilio-api/)
```

## Tech Stack

| Layer       | Technology                          |
|-------------|-------------------------------------|
| Backend     | Go 1.23, Gin v1.9.1                |
| Database    | PostgreSQL 15+ (pgx v5 driver)     |
| AI          | Claude API (Anthropic) for classification & content generation |
| Frontend    | WordPress plugin (PHP), Gutenberg/React (compiled JS) |
| E2E Tests   | Playwright                          |
| CI          | GitHub Actions                      |
| Deployment  | Docker, Railway                     |

## Development Setup

### Backend (Go API)

```bash
cd affilio-api
docker compose up -d                # Start PostgreSQL on :5432
# Apply migrations in order: 001_initial.sql through 009_scale_fraud_trust.sql
go run ./cmd/server                 # Starts API on :8080
```

### Environment Variables

| Variable           | Required | Default                                         | Description                    |
|--------------------|----------|--------------------------------------------------|--------------------------------|
| `PORT`             | No       | `8080`                                           | API listen port                |
| `DATABASE_URL`     | No       | `postgresql://affilio:password@localhost:5432/affilio` | PostgreSQL connection string   |
| `CLAUDE_API_KEY`   | Yes*     | (empty)                                          | Anthropic API key for AI features |
| `CLAUDE_MODEL`     | No       | `claude-sonnet-4-5-20250929`                    | Claude model to use            |
| `ALLOWED_ORIGINS`  | No       | `*`                                              | CORS allowlist (comma-separated) |
| `API_KEY_SECRET`   | No       | `dev-secret`                                     | Secret for site API key generation |
| `ADMIN_API_KEY`    | No       | (empty)                                          | Admin-only endpoint auth key   |
| `STRIPE_WEBHOOK_SECRET` | No  | (empty)                                          | Stripe webhook signature secret (Distributoor conversions) |
| `LEMONSQUEEZY_WEBHOOK_SECRET` | No | (empty)                                     | Lemon Squeezy webhook signature secret |
| `POLAR_WEBHOOK_SECRET` | No   | (empty)                                          | Polar webhook signature secret |
| `CLAIMS_CHECK`     | No       | `stub` when `GIN_MODE=release`, otherwise `off-local` | `stub` holds an untrusted offer in review. `off-local` is local and dev only and keeps the four listing checks. `real` is local-only until the page checker is installed. A hosted server refuses `real` and `off-local`. |

*AI features fall back to safe defaults when `CLAUDE_API_KEY` is empty.

### WordPress Plugin

1. Place `affilio-wp/` in WordPress plugins directory
2. Activate the plugin
3. Go to Affilio Settings → set API key
4. Dashboard → "Scan All Posts" to begin

## Testing

### Unit & Integration Tests (Go)

```bash
cd affilio-api
make test-db-up     # Start test PostgreSQL on :5433
make test           # Run handler tests
make ci-test        # Run all tests with 25% coverage gate
make test-db-down   # Teardown test DB
```

**Important:** Tests require `GOTOOLCHAIN=local` and custom cache paths (set automatically by Makefile). The CI coverage minimum is **25%** (enforced in CI).

### E2E Tests (Playwright)

```bash
cd affilio-wp/e2e
npm install
WP_ADMIN_USER=admin WP_ADMIN_PASS=password \
  AFFILIO_E2E_API_KEY=<key> npm test
```

### CI Pipeline

GitHub Actions (`.github/workflows/affilio-api-tests.yml`):
- Triggers on changes to `affilio-api/**`
- Spins up PostgreSQL 15 service
- Runs `make ci-test` with coverage gate
- Uploads coverage artifact
- Branch protection requires `affilio-api-tests` to pass

## Architecture

### Backend Request Flow

1. **Entry** (`cmd/server/main.go`) → Gin router, DB pool, service initialization
2. **Middleware** → CORS validation → API key or admin key auth
3. **Handlers** → Parse request, call services, return JSON
4. **Services** → Business logic (matching, scoring, AI classification/generation)
5. **Database** → Direct pgx pool queries (`internal/database/queries.go`)

### API Routes (all under `/v1`)

**Public** (requires `X-API-Key` header):
- `POST /scan/full`, `POST /scan/single`, `GET /scan/status` — Article scanning
- `GET /articles`, `GET /articles/:id` — Article management
- `GET|PATCH /recommendations/:id` — Recommendation management
- `POST /generate/content`, `POST /insert/confirm` — Content generation & insertion
- `POST /detect/links` — Affiliate link detection
- `GET|POST|DELETE /user-programs/*` — User program management
- `POST /track/click`, `GET /track/*` — Analytics (summary, trends, attribution, funnel, cohorts, etc.)
- `GET /marketplace/offers`, `POST /marketplace/offers/:id/join` — Distributoor offer marketplace
- `GET /payouts/balance`, `GET /payouts/history` — Publisher commission ledger

**Maker** (requires `X-Maker-Key` header, Distributoor supply side):
- `GET|POST|PATCH /maker/products[/:id]`, `POST /maker/products/:id/assess`, `GET /maker/products/:id/assessments`
- `GET|POST|PATCH /maker/offers[/:id]`, `POST /maker/offers/:id/submit`, `GET /maker/offers/:id/matches`
- `GET /maker/earnings`

**Admin** (requires `X-Admin-Key` header):
- `GET /program-discovery/queue` — Discovery queue
- `POST /program-discovery/queue/:id/review|promote` — Review/promote programs
- `POST /sites/rotate-key` — Rotate site API key
- `GET /offer-review/queue`, `POST /offer-review/:id/approve|reject` — Distributoor offer review
- `POST /payout-batches/run`, `POST /payout-batches/:id/mark-paid` — Manual payout batches
- `POST /offer-review/makers/rotate-key` — Rotate maker API key
- `POST /offer-review/makers/:id/trust` — Trusted-maker fast path (submit lists directly)
- `GET /offer-review/flagged-conversions`, `POST /offer-review/conversions/:id/clear-flag` — Fraud flag review

**Unauthenticated:**
- `GET /health` — Health check
- `POST /v1/sites/register` — Site registration (rate limited: 8 req/min per IP)
- `POST /v1/makers/register` — Maker registration (rate limited: 8 req/min per IP)
- `GET /v1/go/:code` — Tracked redirect link (rate limited: 120 req/min per IP)
- `POST /v1/webhooks/:provider` — Payment provider webhooks (HMAC-verified: stripe, lemonsqueezy, polar)
- `POST /v1/mcp` — Hosted MCP transport for DCP verbs (per-request auth: `X-Maker-Key`/Bearer for maker tools, `X-API-Key` for publisher tools)

### Recommendation Matching

- User-specific programs are prioritized over curated catalog
- Relevance scoring: exact substring match (60-80 pts) + semantic token overlap (45-95 pts) + category bonus (+5 pts)
- Max 5 recommendations per article, deduplicated
- Status transitions are enforced: `pending → approved → inserted`, any state → `dismissed`

### Database

PostgreSQL with pgcrypto extension. Key tables: `affiliate_programs`, `sites`, `articles`, `recommendations`, `clicks`, `link_detections`, `user_programs`, `scan_jobs`, `program_discovery_queue`, `program_discovery_reviews`. Distributoor tables: `makers`, `products`, `readiness_assessments`, `offers`, `campaigns`, `conversions`, `ledger_entries`, `payouts`, `webhook_events`.

Migrations are in `affilio-api/internal/migrations/` and must be applied in numeric order (001-009).

## Conventions

### Code Style

- **Go:** Standard `gofmt` conventions. PascalCase exports, lowercase packages.
- **PHP:** WordPress coding standards. Uses nonces, capability checks, and `sanitize_text_field()` / `esc_url_raw()` for input handling.
- **JS:** Plain JS compiled to `admin/build/`. No source files tracked.

### Naming

| Domain         | Convention           | Example                          |
|----------------|---------------------|----------------------------------|
| Go packages    | lowercase            | `handlers`, `services`           |
| Go types       | PascalCase           | `ScanHandler`, `Recommendation`  |
| HTTP handlers  | Verb-based           | `ScanFull()`, `GenerateContent()` |
| DB tables      | snake_case           | `affiliate_programs`, `scan_jobs` |
| PHP classes    | `Affilio_` prefix    | `Affilio_Admin`, `Affilio_API`   |
| REST routes    | kebab-case           | `/user-programs`, `/track/recommendation-tuning` |
| Env vars       | UPPERCASE_SNAKE      | `CLAUDE_API_KEY`, `ALLOWED_ORIGINS` |

### Development Rules

- Do not remove existing functionality unless explicitly requested
- Keep changes minimal and additive
- If you add a new endpoint, add a matching UI change or document it in `STATUS.md`
- Always update `STATUS.md` when adding features or changing behavior
- Never expose the admin API key to the public frontend
- Any link insertion must include FTC disclosure once per post
- Recommendations must include a reason and relevance score; max 5 per article

### Security

- Site auth via `X-API-Key` header (validated against DB with rotation fallback)
- Admin auth via `X-Admin-Key` header (string comparison)
- CORS allowlist via `ALLOWED_ORIGINS` env var
- WordPress nonces and capability checks on all plugin actions
- Rate limiting on site registration and click tracking
- Input sanitization on all user-facing inputs

## Key Files Reference

| File | Purpose |
|------|---------|
| `affilio-api/cmd/server/main.go` | API entry point, route registration |
| `affilio-api/internal/config/config.go` | Environment variable loading |
| `affilio-api/internal/database/queries.go` | All SQL queries |
| `affilio-api/internal/handlers/scan.go` | Article scanning handler |
| `affilio-api/internal/handlers/recommendations.go` | Recommendation CRUD |
| `affilio-api/internal/services/matcher.go` | Program matching/scoring logic |
| `affilio-api/internal/services/ai_classifier.go` | Claude-powered link classification |
| `affilio-api/internal/services/ai_generator.go` | Claude content generation |
| `affilio-api/internal/services/scorer.go` | Monetization scoring |
| `affilio-api/internal/middleware/auth.go` | API key authentication |
| `affilio-wp/affilio.php` | WordPress plugin entry |
| `affilio-wp/includes/class-affilio-api.php` | PHP API client |
| `affilio-wp/includes/class-affilio-admin.php` | Admin dashboard |
| `affilio-wp/includes/class-affilio-inserter.php` | Link insertion logic |

## Makefile Targets (affilio-api/)

```
make test-db-up      # Start test PostgreSQL (port 5433)
make test-db-down    # Stop and clean test DB
make test            # Run handler tests
make ci-test         # Run tests with 25% coverage gate
make status          # Show MVP checklist status
make branch-protect  # Setup GitHub branch protection
```
