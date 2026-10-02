# Distributoor MVP delivery

October 1, 2026. The release target is a usable product. A presentation, sample
product or recruited publisher is not a prerequisite for releasing the software.

## Implemented product

- Builders sign in, create/edit products, check deployed readiness, connect
  Stripe, create a referral offer and submit it for operator review.
- Their existing coding agent connects through hosted MCP and portable skills.
  The setup command installs the integration knowledge and hands off checkout
  wiring to that agent. It does not claim that installing files wires checkout.
- WordPress administrators install the ZIP, connect the site without copying a
  key, and link its publisher account through a ten-minute, single-use browser
  confirmation. Scoped agent credentials cannot claim ownership.
- A publisher chooses a published article, finds up to five relevant verified
  offers, prepares copy, reviews its exact terms in the owner workspace, approves
  it and inserts the approved recommendation in WordPress.
- The plugin preserves existing content, includes a sponsored/nofollow tracked
  link and one disclosure, and records publication evidence. A receipt retry
  does not insert the recommendation twice.
- Stripe-delivered paid checkout and refund events produce attributed commission
  records. Test/live modes and refund holds remain separate. Merchants report
  direct external payments; publishers separately confirm receipt.
- Operators review offers and batch cleared live commissions. Money transfers,
  service-fee collection and recurring subscription invoices are not automated.

The first version supports WordPress articles and the first paid one-time Stripe
checkout in USD, EUR or GBP. Listing offers does not guarantee traffic or income.

## Hosting and current blocker

The isolated Railway project is `distributoor-mvp`:

| Resource | ID |
| --- | --- |
| Project | `cf3a1d9d-d8f8-44bf-94e1-fcb590055ae0` |
| Production environment | `2534a1db-d231-45ae-868e-680c48b11dfb` |
| Application service | `c3a5be47-e442-4ff4-8dda-ee808944c233` |
| PostgreSQL service | `b9989f40-0521-415a-a752-0abbdaab4779` |

Application origin: `https://distributoor-production.up.railway.app`.
PostgreSQL 15 is running with persistent storage. The app image built on Railway,
but startup failed closed because `OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` are
missing. The URL is **not yet a usable release**. Existing Affilio production
was not changed.

To activate real owner sign-in:

1. Create or reuse a Google OAuth **Web application**. Add the exact redirect URI
   `https://distributoor-production.up.railway.app/auth/callback`.
2. Configure its consent/audience settings to allow the intended early users.
   A testing-only app requires those users to be allowed by Google.
3. Set `OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` on the application service in
   Railway. `OIDC_ISSUER=https://accounts.google.com` is already configured.
   Put secrets in Railway, never chat, source code or an agent client.
4. Redeploy and verify `/health`, real sign-in, owner account creation and MCP
   authentication before calling the MVP delivered.
5. After the operator's first real login, copy their Owner ID from **Setup &
   agents**, set `OPERATOR_OWNER_ID` to that exact value, and redeploy. This enables
   offer review and live payout batching for that identity only. The separate
   admin API key remains available for operator tooling.

## Install and ship

Build from repository root with `Dockerfile.mvp`. It bundles the Go API, canonical
UI under `affilio-wp/admin/build/`, installer archive and WordPress ZIP. Production
uses `AUTO_MIGRATE=true` only on the isolated, history-tracked database. Migration
files are immutable after application; add a new numbered migration for changes.

When the service is healthy, run inside a builder's product repository:

```sh
npm exec --yes --package=https://distributoor-production.up.railway.app/downloads/distributoor-setup-0.1.0.tgz -- distributoor-setup --api-url https://distributoor-production.up.railway.app
```

Publishers download `/downloads/affilio.zip` from that same origin, upload and
activate it in WordPress, open **Affilio → Settings** to connect, then open
**Affilio → Distribution**. The download embeds the service's actual API origin.
The private GitHub repository need not become public. The npm registry and
WordPress directory can be added later.

Release tooling targets only this isolated service. From the repository root:

```sh
python3 scripts/mvp_release.py check
python3 scripts/mvp_release.py deploy
```

The preflight reads Railway variables privately, reports missing names without
printing values, and refuses to upload an uncommitted, unpushed or failing
revision. Required checks are `affilio-api-tests`, `onboarding` and
`mvp-release-package`. The package check builds `Dockerfile.mvp`, verifies its
runtime assets, notices and archives, and runs the packaged installer in a fresh
product directory. It does not publish the internal WordPress ZIP: the public
download handler embeds the correct service origin before serving it.
All three workflows also allow manual runs against a release branch when a
commit falls outside their automatic path filters.

`deploy` requests an upload; it does not claim the deployment is healthy. Wait
for the exact deployment to report **SUCCESS** in Railway, then run:

```sh
python3 scripts/mvp_release.py verify
```

This checks public health, hosted assets, both installation downloads, plugin API
origin, OAuth discovery, anonymous owner/MCP rejection and Google's login
redirect/cookie configuration. Complete a real Google login separately; a correct
redirect does not prove that Google accepts the client credentials or audience.

The service currently receives CLI source deployments. GitHub automatic deploys
are not configured. Upload and Docker ignore files exclude local secrets, env
files, dependencies and test artifacts.

## Verification and release checks

Backend tests, handler/database/migration race checks and Go vet pass. Migration
tests cover fresh and concurrent startup, repeated application, checksum changes
and refusal to migrate an untracked existing schema. Builder tests and both PHP
suites pass. Docker image and installer/WordPress archive contents were checked.

Chromium drives the actual owner backend through product creation/editing, draft
offer creation, refusal before readiness, placement matching/approval, tracked
link commit, payout instructions, merchant payment reporting and publisher
receipt confirmation. Mobile layout and unsafe product text are covered. These
checks use isolated identities and data; they do not prove real Google login,
customer purchases or merchant funds received.

A real WordPress 6.6/PHP 8.2 instance also passed browser-driven installation
connection, owner handoff, offer matching, exact approval, insertion into a
published article, persisted backend publication receipt and a reader referral
redirect with a placement-attributed click. The fixture uses
local HTTPS and isolated owner sessions; no customer article was published.
Set `AFFILIO_WORKSPACE_DOWNLOADS` to the extracted image's downloads directory
when running `TestWordPressDistributionBrowser` to install the actual HTTP-served
ZIP and verify its embedded API origin. Its isolated WP-CLI container must mount
the QA ZIP at `/release/affilio.zip`, with no source plugin bind mount.

Use the real Stripe integration to validate each merchant's checkout: preserve
the `via` click UUID, set Checkout Session `client_reference_id` and
`metadata.dtr_product_id`, use the configured one-time price, and deliver paid
checkout/refund events to the product-specific webhook. A signed test fixture
does not prove a deployed merchant checkout. See [HOSTED-PILOT.md](HOSTED-PILOT.md).

After release, the first iteration is onboarding the first builders and
publishers, fixing their setup failures, and observing an unrelated attributed
buyer plus an actual commission receipt. Subscription billing, additional
publishing channels and ranking changes should follow evidence from that loop.

## Landing illustration and UI builds

The landing page uses original illustration work inspired by Blume's visual clarity and ThreeUI's interactive presentation. It bundles Three.js locally, retains the SVG on mobile/reduced-motion/data-saving browsers and preserves the primary actions without JavaScript or WebGL. The owner workspace shares copyable installer commands with clipboard fallback. No ThreeUI assets or app framework are shipped.

The Docker build and CI compile the optional scene automatically. For local development:

```sh
cd affilio-wp/admin/build
npm ci --ignore-scripts
npm run build
```

`landing-scene.bundle.js` and its generated legal notice are build outputs. The full Three.js MIT notice lives in `THIRD_PARTY_NOTICES.md` and ships in the hosted image. The WordPress ZIP does not include hosted landing assets.
