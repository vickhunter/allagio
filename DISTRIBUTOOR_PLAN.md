# Distributoor — Architecture Plan

**Status:** Draft v1 · **Scope:** Supply-side extension of Affilio ("offer rails")
**Audience:** Anyone building or evaluating the Distributoor extension of this repo.

---

## 1. What Distributoor is

Distributoor is an **AI-native distribution network for software makers**. It takes a
product built by an AI-native/vibe-coding maker — a SaaS app, game, Shopify app, or
developer tool — and gets it distributed and monetized through publishers and
affiliates, with agents doing the operational work.

Affilio already owns the **demand side** of this network: WordPress publishers whose
content is scanned, whose affiliate opportunities are detected, and who insert
tracked links with FTC disclosure. What Affilio lacks is supply: the
`affiliate_programs` catalog is seeded manually (`002_seed_programs.sql`) or grown
through the discovery queue when publishers happen to link to unknown domains.

Distributoor completes the marketplace by adding the **supply side**:

| Side | Actor | Today | With Distributoor |
|------|-------|-------|-------------------|
| Demand | Publisher (WP site) | Scans posts, gets recommendations, inserts links | Unchanged, plus a marketplace of live offers with real payouts |
| Supply | Maker | Doesn't exist | Registers products, passes readiness gates, publishes offers, pays affiliates |

The "offer rails" are the pipeline an offer travels:

```
Maker product ──▶ Readiness assessment ──▶ Offer ──▶ Listed in catalog
                                                        │
Publisher article ◀── Recommendation ◀── Matching ◀────┘
       │
       ▼
Insert (FTC disclosure) ──▶ Click ──▶ Conversion ──▶ Ledger ──▶ Payout
```

Everything left of "Listed in catalog" is new. Everything right of it already exists
in Affilio through the click — Distributoor extends tracking past the click into
conversions and payouts.

---

## 2. Core decision: extend affilio-api, don't build a second service

Distributoor ships **inside `affilio-api`** as new packages, handlers, and
migrations — a modular monolith — not as a separate service.

Rationale:

- **The catalog is the join point.** An offer only has value once it lands in
  `affiliate_programs`, where `services/matcher.go` and the recommendation flow
  already pick it up. A separate service would need to sync that table constantly.
- **Attribution spans both sides.** A conversion joins a maker's webhook event to a
  publisher's `clicks` row. One database makes that a query; two services make it a
  distributed-systems problem we don't have the traffic to justify.
- **Everything is already in place**: Gin routing with per-group auth middleware,
  pgx pool, AI services with safe no-key fallbacks, migration convention, the
  25% coverage CI gate.

Boundary discipline so a later extraction stays possible: all new tables are
prefixed by domain (no FKs from existing tables into new ones except the one
`affiliate_programs.offer_id` column), new code lives in clearly separated files,
and makers never share auth with sites or admins.

---

## 3. Domain model

New objects (all new tables; existing tables in *italics*):

```
Maker ──▶ Product ──▶ ReadinessAssessment (checks[])
              │
              └──▶ Offer ──promote──▶ *affiliate_programs* row
                     │
                     └──▶ Campaign (offer × *site*)
                              │
              *clicks* ◀──────┘
                 │
                 ▼
             Conversion (from provider webhook)
                 │
                 ▼
             LedgerEntry ──▶ Payout
```

### Migration `007_distributoor_core.sql`

```sql
CREATE TABLE makers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    api_key VARCHAR(64) UNIQUE NOT NULL,        -- same pattern as sites.api_key
    api_key_previous VARCHAR(64),               -- rotation support from day one (005 pattern)
    api_key_rotated_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    maker_id UUID REFERENCES makers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    tagline VARCHAR(512),
    website_url VARCHAR(512) NOT NULL,
    docs_url VARCHAR(512),
    pricing_url VARCHAR(512),
    product_type VARCHAR(50) NOT NULL,          -- saas | game | shopify_app | dev_tool | other
    payment_provider VARCHAR(50),               -- stripe | lemonsqueezy | polar | none
    primary_category VARCHAR(100),              -- same taxonomy as affiliate_programs
    tags TEXT[],
    keywords TEXT[],
    readiness_score INTEGER DEFAULT 0,          -- 0-100, denormalized from latest assessment
    readiness_status VARCHAR(20) DEFAULT 'unassessed', -- unassessed | failing | ready
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE readiness_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'running',       -- running | complete | failed
    score INTEGER,
    checks JSONB DEFAULT '[]',                  -- [{key, kind, status, score, evidence, remediation}]
    ai_summary TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    finished_at TIMESTAMP
);

CREATE TABLE offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID REFERENCES products(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'draft',         -- draft | submitted | listed | paused | retired
    commission_type VARCHAR(50) NOT NULL,       -- percentage | flat | recurring
    commission_value DECIMAL(10,2) NOT NULL,
    commission_unit VARCHAR(10),
    commission_currency VARCHAR(3) DEFAULT 'USD',
    cookie_duration_days INTEGER DEFAULT 30,
    refund_window_days INTEGER DEFAULT 30,      -- hold period before ledger entries clear
    landing_url VARCHAR(512) NOT NULL,
    ref_param VARCHAR(50) DEFAULT 'via',        -- query param the product honors
    terms TEXT,
    creative_assets JSONB DEFAULT '[]',
    program_id UUID REFERENCES affiliate_programs(id), -- set on promotion, the rail into demand side
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id UUID REFERENCES offers(id) ON DELETE CASCADE,
    site_id UUID REFERENCES sites(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'active',        -- active | paused | ended
    tracking_code VARCHAR(32) UNIQUE NOT NULL,  -- short code used in /v1/go/:code
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(offer_id, site_id)
);
```

`affiliate_programs` gains two columns (same migration):

```sql
ALTER TABLE affiliate_programs ADD COLUMN IF NOT EXISTS source VARCHAR(20) DEFAULT 'curated';
    -- curated | discovery | distributoor
ALTER TABLE affiliate_programs ADD COLUMN IF NOT EXISTS offer_id UUID REFERENCES offers(id);
```

This is deliberately the same shape as the discovery-queue **promote** flow
(`handlers/program_discovery.go`): an offer that passes review is *promoted into*
the catalog, and from that moment the entire existing pipeline — matcher, max-5
recommendations, status guardrails from `006`, insertion, click tracking — works on
it with zero changes.

### Migration `008_attribution_payouts.sql`

```sql
CREATE TABLE conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id UUID REFERENCES offers(id),
    campaign_id UUID REFERENCES campaigns(id),
    click_id UUID REFERENCES clicks(id),        -- attribution join; nullable (unattributed)
    provider VARCHAR(50) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,    -- idempotency
    provider_object_id VARCHAR(255),            -- checkout session / order id
    kind VARCHAR(20) NOT NULL,                  -- sale | renewal | refund
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    commission_amount DECIMAL(10,2),
    occurred_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider, provider_event_id)
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id),
    conversion_id UUID REFERENCES conversions(id),
    kind VARCHAR(20) NOT NULL,                  -- commission | clawback | adjustment | payout
    amount DECIMAL(10,2) NOT NULL,              -- negative for clawback/payout
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(20) DEFAULT 'pending',       -- pending | cleared | reversed
    clears_at TIMESTAMP,                        -- occurred_at + offer.refund_window_days
    payout_id UUID,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID REFERENCES sites(id),
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    method VARCHAR(50),                         -- manual | stripe_connect | provider_native
    provider_reference VARCHAR(255),
    status VARCHAR(20) DEFAULT 'pending',       -- pending | paid | failed
    created_at TIMESTAMP DEFAULT NOW(),
    paid_at TIMESTAMP
);

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    processed_at TIMESTAMP,
    error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider, provider_event_id)         -- replay/duplicate protection
);
```

Raw events are stored before processing so attribution bugs can be replayed, and the
unique constraint makes webhook ingestion idempotent.

---

## 4. Readiness engine

**Purpose:** answer "is this product actually distributable?" before any publisher
wastes traffic on it. This is a hard gate: an offer cannot leave `draft` until its
product's latest assessment passes all P0 checks.

`services/readiness.go` runs a **check catalog**, two kinds of checks:

### Deterministic probes (no AI, always run)

| Check | P0? | How |
|---|---|---|
| `site_reachable` | ✅ | GET landing_url, expect 2xx, TLS valid |
| `checkout_present` | ✅ | Detect stripe.com / lemonsqueezy / polar.sh / buy.polar.sh in checkout flow or pricing page |
| `ref_param_survives` | ✅ | Fetch `landing_url?via=dtr-probe`, follow redirects, verify the param isn't stripped (evidence for cookie/attribution viability) |
| `webhook_configured` | ✅ (for payout phase) | Provider webhook registered and test event received |
| `pricing_page` | — | pricing_url resolves and mentions a price |
| `docs_present` | — | docs_url resolves, non-trivial content length |
| `privacy_policy` | — | Discoverable privacy/terms links |
| `onboarding_flow` | — | Signup/CTA reachable from landing page |

### AI rubric checks (Claude, same pattern as `ai_classifier.go` / `ai_generator.go`)

Fetched page content is scored against a rubric: onboarding clarity, docs quality,
value-prop legibility for a publisher audience, category/keyword extraction (which
feeds the offer's matcher fields directly). **Follows the existing convention: when
`CLAUDE_API_KEY` is empty, AI checks return neutral defaults and only deterministic
probes gate readiness.** AI extraction output populates `products.keywords` /
`tags` / `primary_category` — the exact fields `matcher.go` scores on — so
assessment quality directly becomes matching quality.

Every check returns `{key, status: pass|warn|fail, score, evidence, remediation}`.
The `remediation` string is written for an *agent* to act on ("add a
`?via=` passthrough to your signup redirect; here's the snippet"), because the
consumer of a failed assessment is usually the maker's coding agent, not the maker.

---

## 5. Matching: reusing the demand side, adding the reverse direction

Nothing changes for article→program matching: a promoted offer is an
`affiliate_programs` row with good keywords/tags, and `matcher.go` (substring
60–80 pts, token overlap 45–95 pts, category +5, max 5 recs, user-program priority)
already ranks it into recommendations.

New in Distributoor is **offer→publisher matching** for the marketplace and for
maker-side analytics:

- `services/site_profiler.go` aggregates a per-site topic profile from data that
  already exists: scanned `articles.analysis`, category distribution of that site's
  recommendations, and historic CTR from `clicks`.
- Offer→site relevance reuses the same token-overlap scorer against the site
  profile instead of a single article — one scoring implementation, two directions.
- Ranked results power `GET /v1/marketplace/offers` (publisher side: "offers that
  fit my site, with real commission terms") and `GET /v1/maker/offers/:id/matches`
  (maker side: "publishers worth inviting"). Accepting a match creates a `campaign`
  with a fresh `tracking_code`.

---

## 6. Attribution & payouts

### Design constraint discovered up front

None of the three named providers gives us affiliate rails to lean on:

- **Stripe** has no native affiliate system (third parties like Rewardful/Tolt fill
  that gap). Attribution must ride on `client_reference_id` / metadata + webhooks.
- **Lemon Squeezy** was acquired by Stripe (2024) and is being folded into Stripe
  Managed Payments during 2026; its built-in affiliate hub may not survive the
  migration. Safe to consume its webhooks, unsafe to depend on its affiliate hub.
- **Polar** deliberately ships **no** built-in affiliate program; partner
  integration is its sanctioned path (which arguably makes Distributoor exactly the
  partner-shaped thing Polar wants to exist).

**Conclusion: Distributoor owns the attribution ledger. Providers are
conversion-signal adapters behind one interface, nothing more.**

### Click → conversion flow

```
Publisher post
   │  href = https://api.affilio.../v1/go/{tracking_code}
   ▼
GET /v1/go/:code            (public, rate-limited, no auth — like /sites/register)
   │  1. insert clicks row (existing table; campaign context via recommendation_id/program_id)
   │  2. 302 → offer.landing_url + ?{ref_param}={click_id-short}
   ▼
Maker's product             (readiness check `ref_param_survives` guaranteed this works)
   │  drop-in snippet persists the ref value (localStorage + cookie)
   │  and passes it into checkout:
   │    Stripe: client_reference_id / metadata.dtr
   │    Lemon Squeezy: checkout[custom][dtr]
   │    Polar: checkout metadata.dtr
   ▼
POST /v1/webhooks/:provider (public, HMAC signature verified per provider, idempotent)
   │  store webhook_events → parse via ProviderAdapter interface
   │  → conversions row, joined to clicks via the dtr value
   ▼
ledger_entries: commission, status=pending, clears_at = occurred_at + refund_window
   │  refund webhook within window → clawback entry, conversion kind=refund
   ▼
Payout batch (cron/admin): sum cleared balance per site ≥ min_payout → payouts row
```

```go
// internal/services/providers/provider.go
type ProviderAdapter interface {
    VerifySignature(headers http.Header, body []byte, secret string) bool
    ParseEvent(body []byte) (*ConversionEvent, error) // sale | renewal | refund, amount, dtr ref
}
// stripe.go, lemonsqueezy.go, polar.go implement it. Adding a provider = one file + tests.
```

### Payout execution is phased deliberately

1. **Phase 1 — ledger only, manual payouts.** Correct balances, admin export,
   manual transfer, `payouts` row records it. No money-movement code, no KYC burden.
2. **Phase 2 — Stripe Connect** transfers for publishers who onboard to Connect
   Express; `method=stripe_connect`.
3. **Provider-native stays optional** (`method=provider_native`) for makers who run
   e.g. a surviving LS affiliate hub — Distributoor then only mirrors state.

This ordering is the single biggest de-risking decision in the plan: attribution
correctness is provable with tests; money movement is a compliance project.

### Fraud controls (day one, cheap)

- Self-referral: flag conversions whose click `ip_hash` matches recent maker-side
  traffic or whose click→conversion time is implausibly short.
- Velocity caps per campaign; auto-pause on anomaly.
- Refund-window holds (already in the ledger design) neutralize buy-refund fraud.
- `ref_param_survives` probe evidence stored, so disputes have a paper trail.

---

## 7. Agentic layer

Target user is an AI-native maker, so the primary interface is *their agent*, not a
dashboard. Three surfaces, all thin wrappers over the same HTTP API:

### CLI — `affilio-api/cmd/distributoor`

```
distributoor init                 # register maker + product (writes .distributoor.toml)
distributoor assess               # run readiness, print checklist w/ remediation
distributoor offer create|submit  # define terms, submit for listing
distributoor snippet              # print the ref-capture JS + provider metadata wiring
distributoor status               # campaigns, clicks, conversions, pending balance
```

Same repo, same Go module, imports nothing from `internal/` except a small API
client package — keeps the monolith boundary clean.

### MCP server — `affilio-api/cmd/distributoor-mcp`

Stdio MCP server exposing the maker API as tools:
`register_product`, `run_assessment`, `get_assessment` (returns remediation strings
written for agents), `create_offer`, `submit_offer`, `list_matches`,
`invite_publisher`, `get_earnings`. Auth via `DISTRIBUTOOR_API_KEY` env var (maker
key). This is what makes "create or update accounts, projects, campaigns, and
listings through agents" real: a maker tells Claude "get my app listed on
Distributoor" and the agent drives these tools end to end.

### Markdown skills — `.claude/skills/`

- `distributoor-onboard` — walks an agent through registration → assessment →
  fixing failed checks in the maker's own codebase → resubmission. This is the
  killer loop: the assessment's remediation output is *input to a coding agent*
  that edits the product until it passes.
- `distributoor-instrument` — wiring the ref-capture snippet and provider webhooks
  (per-provider instructions for Stripe / LS / Polar).
- `distributoor-launch` — offer terms guidance, creative assets, publisher
  invitations.

Agent-safety rails: all maker-side writes accept an `Idempotency-Key` header
(agents retry), and destructive operations (retire offer, delete product) require
an explicit `confirm=true` — protecting makers from their own agents.

---

## 8. API surface

New route groups in `cmd/server/main.go`, following the existing middleware pattern:

```
Public (rate-limited, no auth):
  POST /v1/makers/register            # like /sites/register (8 req/min/IP)
  GET  /v1/go/:code                   # click redirect
  POST /v1/webhooks/:provider         # HMAC-verified per provider

Maker (X-Maker-Key middleware — new, mirrors APIKeyAuth):
  GET|POST|PATCH /v1/maker/products[/:id]
  POST /v1/maker/products/:id/assess
  GET  /v1/maker/products/:id/assessments[/:aid]
  GET|POST|PATCH /v1/maker/offers[/:id]
  POST /v1/maker/offers/:id/submit
  GET  /v1/maker/offers/:id/matches
  POST /v1/maker/campaigns/:id/invite
  GET  /v1/maker/earnings

Publisher (existing X-API-Key middleware):
  GET  /v1/marketplace/offers          # ranked for this site
  POST /v1/marketplace/offers/:id/join # creates campaign + tracking_code
  GET  /v1/payouts/balance
  GET  /v1/payouts/history

Admin (existing X-Admin-Key middleware):
  GET  /v1/offer-review/queue          # submitted offers
  POST /v1/offer-review/:id/approve    # promote → affiliate_programs (source='distributoor')
  POST /v1/offer-review/:id/reject
  POST /v1/payout-batches/run
  POST /v1/makers/rotate-key
```

Human review before listing is kept on purpose (same philosophy as the discovery
queue): the AI assessment gates *quality*, the admin gates *trust* — automate the
first, keep a human on the second until fraud tooling matures.

New env vars: `STRIPE_WEBHOOK_SECRET`, `LEMONSQUEEZY_WEBHOOK_SECRET`,
`POLAR_WEBHOOK_SECRET`, `MAKER_KEY_SECRET` (all optional; features degrade off,
matching the `CLAUDE_API_KEY` convention).

---

## 9. Phased roadmap

| Phase | Ships | Reuses | Proves |
|---|---|---|---|
| **P0 — Offer rails skeleton** | Migration 007, maker registration + auth, product/offer CRUD, admin review → promote into `affiliate_programs` | Discovery promote pattern, matcher, recommendations, WP insertion, click tracking — all untouched | A maker-submitted offer appears as a recommendation in a real WP post |
| **P1 — Readiness engine** | Deterministic probes, AI rubric + keyword extraction, remediation output, listing gate | `ai_classifier.go` HTTP/fallback pattern, discovery enricher patterns | Assessment quality → matching quality; agents can fix failing checks |
| **P2 — Tracked links + conversions** | `/v1/go/:code`, campaigns, ref snippet, webhook ingestion (Stripe first, then LS, Polar), conversions table | `clicks` table + tracking handlers | End-to-end attribution: post → click → purchase → conversion row |
| **P3 — Ledger + payouts (manual)** | Ledger entries, refund clawbacks, holds, balances, admin payout batches, publisher earnings UI in WP plugin | Tracking analytics endpoints style | Publishers see real money attributable to real inserts |
| **P4 — Agentic layer** | CLI, MCP server, three Markdown skills, idempotency keys | The whole API | A maker's Claude session takes a product from zero → listed → earning |
| **P5 — Scale & automation** | Offer→publisher matching v2 (site profiles), Stripe Connect payouts, fraud scoring, auto-approve trusted makers | Scorer/tuning services | Marketplace runs without an admin in the loop |

P0–P2 are the MVP: they prove the thesis (a maker's product can reach a publisher's
audience with working attribution) before any money-movement complexity exists.

**Cold-start note:** demand side is seeded by definition — every existing Affilio
publisher is a distribution endpoint on day one. Supply side is seeded by dogfooding
(Affilio itself, plus maker communities where vibe-coded products launch). This is
the structural advantage over building Distributoor standalone.

---

## 10. Risks & open questions

1. **Marketplace trust** — a bad offer (broken product, unpaid commissions) burns
   publisher trust fast. Mitigation: readiness gate + human review + escrow-style
   holds; makers prepay or connect billing before listing goes live (open question:
   deposit vs. invoice-in-arrears).
2. **Provider churn** — the LS→Stripe migration mid-2026 shows conversion-signal
   sources will keep shifting. The `ProviderAdapter` interface plus raw
   `webhook_events` storage is the hedge.
3. **Payout compliance** — moving money to publishers across borders is a
   KYC/tax problem. Deferred via manual payouts (P3) then Stripe Connect (P5),
   which carries the compliance burden.
4. **Attribution leakage** — SPAs, cross-device, ad blockers eat ref params. The
   `ref_param_survives` probe plus first-party snippet limits it; accept that some
   conversions land unattributed (`click_id NULL`) and report them honestly.
5. **Tenancy** — makers and publishers share one Postgres and one API deployment.
   Fine at this scale; the table prefixing and single FK touchpoint keep an
   extraction path open.
6. **FTC compliance stays non-negotiable** — Distributoor-sourced insertions ride
   the existing disclosure-once-per-post rule; offer terms must not incentivize
   removing it.
7. **Open question — take rate**: fee on conversions (% of commission), listing
   fees, or SaaS subscription for makers. Affects whether the ledger needs a
   platform-fee entry kind (cheap to add now: `kind='platform_fee'`).

---

## 11. New files at a glance

```
affilio-api/
├── cmd/
│   ├── distributoor/main.go            # maker CLI
│   └── distributoor-mcp/main.go        # MCP server (stdio)
├── internal/
│   ├── handlers/
│   │   ├── makers.go, products.go, offers.go, campaigns.go
│   │   ├── marketplace.go, redirects.go, webhooks.go
│   │   ├── offer_review.go, payouts.go   (+ _test.go for each)
│   ├── middleware/maker_auth.go
│   ├── migrations/
│   │   ├── 007_distributoor_core.sql
│   │   └── 008_attribution_payouts.sql
│   └── services/
│       ├── readiness.go, site_profiler.go, attribution.go, ledger.go
│       └── providers/{provider,stripe,lemonsqueezy,polar}.go
.claude/skills/
├── distributoor-onboard/SKILL.md
├── distributoor-instrument/SKILL.md
└── distributoor-launch/SKILL.md
```

Existing code modified: `cmd/server/main.go` (route registration),
`internal/config/config.go` (new env vars), `affiliate_programs` (+2 columns).
Nothing existing is removed or behavior-changed — additive throughout, per the
repo's development rules.
