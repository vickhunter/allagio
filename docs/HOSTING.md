# Hosting the MVP on Render and Neon

This is the non-Railway path. It does not change the Railway project, and it does not print secret values. Set every variable in the host's secret store. The names below are the only thing this document lists for secrets.

`Dockerfile.mvp` is the image. Its runtime stage sets `GIN_MODE=release`, so the production startup guard is on: an empty or `dev-secret` `API_KEY_SECRET`, or an empty or `*` `ALLOWED_ORIGINS`, refuses to boot.

## Render free web service

Create a Render web service from this repo and point it at `Dockerfile.mvp`. Render's default public port is `10000`. The process must listen on `0.0.0.0` and on `PORT`.

[Render web services](https://render.com/docs/web-services) terminate TLS at the load balancer and forward the request to the container over HTTP. [Render's DDoS writeup](https://render.com/articles/how-render-handles-ddos-attacks) (1 Apr 2026) says public traffic passes through Cloudflare, then the Render load balancer, and that the app sees the proxy's address unless it trusts that proxy and reads `X-Forwarded-For`.

A [free web service](https://render.com/docs/free) spins down after 15 minutes without inbound traffic and takes about a minute to spin back up. Browsers see a Render loading page during that minute. A spun-down `/robots.txt` is a disallow-all response and does not wake the service. Free Render Postgres expires 30 days after creation, so this path does not use it.

Clients and agents must call `GET /health` before any other request. If the connection fails, times out, or returns a non-JSON loading page, wait and retry with backoff (for example 5s, 10s, 20s, 40s) for about two minutes. That wait is the cold start. It is not a failed deploy.

Set the Render health check path to `/health`.

## Trusted proxies

`TRUSTED_PROXIES` is required. Empty means the process trusts nobody, so every visitor collapses onto the proxy address. Rate limits and click IP hashes then treat the whole internet as one client. `*`, `0.0.0.0/0`, and `::/0` are rejected by `scripts/mvp_release.py` because they would honor a client-spoofed `X-Forwarded-For`.

Gin does not take a hop count. It trusts the socket peer only when that address is in `TRUSTED_PROXIES`, then walks `X-Forwarded-For` from the right and stops at the first untrusted address. Do not copy Render's Express snippet that uses `trust proxy` 1 and the leftmost `X-Forwarded-For` value. A client can prepend that leftmost value.

Start from Cloudflare's published ranges, because Render says the edge in front of the load balancer is Cloudflare. Refresh them from <https://www.cloudflare.com/ips-v4> and <https://www.cloudflare.com/ips-v6> when Cloudflare changes the lists. Values fetched for this document:

```
173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,103.31.4.0/22,141.101.64.0/18,108.162.192.0/18,190.93.240.0/20,188.114.96.0/20,197.234.240.0/22,198.41.128.0/17,162.158.0.0/15,104.16.0.0/13,104.24.0.0/14,172.64.0.0/13,131.0.72.0/22,2400:cb00::/32,2606:4700::/32,2803:f800::/32,2405:b500::/32,2405:8100::/32,2a06:98c0::/29,2c0f:f248::/32
```

The socket peer is the Render load balancer, which may sit outside those ranges. If `ClientIP` stays on one address that is not in the list above, add that observed proxy range to `TRUSTED_PROXIES`. Do not widen the list to every address to make the header move.

## Neon Free, then Aiven Free

Use a Neon Free Postgres database. Neon offers a direct hostname and a pooled hostname whose name contains `-pooler` ([connection pooling](https://neon.com/docs/connect/connection-pooling)). `AUTO_MIGRATE` runs schema migrations through `DATABASE_URL` and needs the direct, non-pooled string. Put the pooled string nowhere in this service. Keep `sslmode=require` as Neon shows it in the console. Copy the string into the secret store. Do not commit it.

If Neon Free is unavailable, use an [Aiven Free PostgreSQL](https://aiven.io/docs/platform/concepts/free-plan) service and its direct service URI as `DATABASE_URL`. Aiven may power off a free service that stays idle. Power it back on from the Aiven console, then retry `GET /health`. Do not use a pooler URI for migrations.

## Environment names

Set these in Render. Values stay in the dashboard.

| Name | What to set |
| --- | --- |
| `DATABASE_URL` | Neon direct connection string, or the Aiven service URI |
| `ADMIN_API_KEY` | At least 32 characters |
| `API_KEY_SECRET` | At least 32 characters, and not `dev-secret` |
| `CONNECTION_ENCRYPTION_KEY` | Base64 of 32 bytes |
| `OIDC_ISSUER` | `https://accounts.google.com` |
| `OIDC_CLIENT_ID` | Google web client id |
| `OIDC_CLIENT_SECRET` | Google web client secret |
| `PUBLIC_BASE_URL` | `https://<service>.onrender.com` with no path or trailing slash. Readiness refuses private and loopback hosts. This origin is the only exception, so the server can assess its own demo product. No other host is allowed. |
| `ALLOWED_ORIGINS` | The same origin, and nothing else |
| `HOST` | `0.0.0.0` |
| `PORT` | Render's `PORT` (default `10000`) |
| `AUTO_MIGRATE` | `true` so a fresh database applies migrations on boot |
| `GIN_MODE` | `release` (also the image default) |
| `TRUSTED_PROXIES` | The Cloudflare ranges above, plus the Render proxy range if logs show one |
| `OPERATOR_OWNER_ID` | Leave unset until the first real owner login |
| `UI_ASSETS_PATH` | Image default `/app/assets` |
| `DOWNLOADS_PATH` | Image default `/app/downloads` |

Optional and unused until you connect them: `CLAUDE_API_KEY`, `CLAUDE_MODEL`, `STRIPE_WEBHOOK_SECRET`, `LEMONSQUEEZY_WEBHOOK_SECRET`, `POLAR_WEBHOOK_SECRET`.

`CLAIMS_CHECK` is optional. Leave it unset. A hosted server is `GIN_MODE=release`, including the image default, and that empty value is `stub`: an untrusted maker's offer stays `submitted` with `review_reason` `claims` and the message that the claims check is not live yet. Do not set `real` or `off-local` on Render. Both are refused while `GIN_MODE=release`, and the hosted default stays `stub`. Local `real` compares the listing card with the merchant page. `off-local` is the local and dev value that keeps the four listing checks.

The server serves a static demo product at `/demo`, `/demo/pricing`, `/demo/docs`, `/demo/privacy`, and `/demo/terms`. The landing page has a get-started link, and `?via=` is left on the URL. A checkout link is not rendered. The shipped server has no checkout URL for the demo and no environment variable that sets one. Assessing `https://<service>.onrender.com/demo` is allowed only because that origin is `PUBLIC_BASE_URL`.

Stripe connection on the owner page is test mode only. Live secret and restricted keys are rejected before any request to Stripe.

## Check and verify

From a clean, pushed commit whose `onboarding`, `affilio-api-tests`, and `mvp-release-package` checks are green:

```bash
python3 scripts/mvp_release.py check --host generic --env-file path/to/render.env
python3 scripts/mvp_release.py verify --base-url https://<service>.onrender.com
```

`--host generic` reads the env file and does not call the Railway CLI. `deploy` stays on `--host railway`. The env file is a local `KEY=VALUE` copy of the names above. Do not commit that file. The checker prints names when something is missing, and it does not print values.

Call `GET /health` and wait through the cold start before treating `verify` as a failure.
