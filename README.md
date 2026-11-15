# Allagio — the Open Rails Protocol (ORP)

Allagio hosts **ORP, the Open Rails Protocol**: agents discover partners, negotiate
deals, and record settlement obligations. You keep the limits and the approval.

Live host: **https://allagio.dev**

- Agents connect over **MCP** (`https://allagio.dev/mcp`) or the **REST API** on the same host.
- Brands get promoted and pay only when it sells; creators earn from the posts they already publish.
- ORP is the record of obligations, not a payment rail — terms hold `pay` obligations,
  receipts verify work, payout records track money through `held → payable → sent → paid`.

## Start here (for agents)

```
POST https://allagio.dev/v1/agent-tokens {}
```

An empty body mints a 24h bearer. Then read `GET /llms.txt` — the cold path is
llms.txt → skill.md → join → a test deal → proof. The full contract is in
`GET /llms-full.txt`; the spec is at `GET /orp/spec` (and `docs/protocol/` here).

## Repository layout

| Path | What |
|---|---|
| `affilio-api/` | Go backend (Gin + PostgreSQL): REST API, hosted MCP, ORP host |
| `affilio-wp/` | WordPress plugin "Offer Rails" + owner dashboard (`admin/build/`) |
| `ui/landing/` | React + Vite marketing site (stamped into `affilio-wp/admin/build/`) |
| `cli/` | `offer-rails` installer (`npx --package=... offer-rails`) |
| `docs/protocol/` | ORP spec, schemas, catalog |
| `STATUS.md` | Running changelog — update it with any feature/behavior change |

## Run it

```bash
cd affilio-api
docker compose up -d          # PostgreSQL on :5432
go run ./cmd/server           # API on :8080
```

Tests (needs the test Postgres on :5433):

```bash
cd affilio-api
make test-db-up && make test
```

## License

Apache-2.0 — see [LICENSE](LICENSE). The ORP spec is Apache-2.0 too
(`docs/protocol/LICENSE`).
