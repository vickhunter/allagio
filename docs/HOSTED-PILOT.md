# Hosted Distributoor pilot

The next pilot supports hosted MCP, signed-in owner review and a Stripe configuration check. It is implemented on `feat/agent-revenue-poc`; it has not been deployed. The existing Railway production service remains on `main`.

## Connect once

1. Add the pilot's `/mcp` URL to the agent client.
2. Sign in with the configured OIDC provider. Google is supported by standard OIDC; GitHub user login would require a separate OAuth adapter.
3. Choose maker or publisher. New accounts need no API key. Existing accounts require their owner key once; neither email matching nor a scoped publisher agent key can claim an account.
4. Review the requesting client's name, callback and role permissions, then allow access for seven days.
5. A publisher agent prepares a placement and returns `/owner/placements/{id}`. The owner reviews exact copy, disclosure, commission, refund hold, service fee, currency and payment mode. After approval the agent commits the tracked link and publishes using the owner's existing tool.

The owner workspace lists proposals, agent connections and payments. Disconnecting an agent invalidates its access and refresh tokens. Human receipt confirmation remains a separate action after a merchant-reported live payment.

## Authentication contract

- `/mcp`: official Go MCP SDK v1.0.0, Streamable HTTP with JSON responses and stateless requests; tested using its HTTP client. The supported wire protocol is 2025-06-18; this does not claim conformance with the 2026 protocol revision.
- Discovery: `/.well-known/oauth-protected-resource/mcp` and `/.well-known/oauth-authorization-server`.
- Dynamic registration: `/oauth/register`. Public clients use exact HTTPS or numeric loopback redirect URIs and S256 PKCE.
- `/oauth/authorize` requires owner consent. Codes are hashed, expire after five minutes and can be exchanged once. Resource parameters must match the canonical `/mcp` URL.
- Opaque access tokens expire after one hour. Refresh tokens rotate, last at most seven days and revoke the grant on reuse. Tokens and browser session secrets are stored as SHA-256 hashes; issuer tokens are never forwarded to tools.
- Verified OIDC issuer + subject establishes identity. Email must be verified, but does not establish ownership of an existing maker or publisher account.
- New OAuth maker accounts have no legacy API key. Legacy makers keep their existing key authentication, and an operator can explicitly issue a key if a separate integration needs it.
- Owner sessions last eight hours. Production cookies use Secure, HttpOnly and the `__Host-` prefix; owner mutations require the session's CSRF token and exact Origin.
- Publisher OAuth tokens cannot approve, revoke placements, confirm funds, alter WordPress settings or use legacy marketplace joins. Maker OAuth tokens cannot configure payment connections or perform operator actions.
- Legacy stdio and `/v1/mcp` key-based clients remain available. DCP discovery distinguishes the legacy HTTP JSON-RPC endpoint from the new hosted transport.

The consent screen identifies the callback because a dynamically registered client supplies its own display name. This pilot supports one maker account and one publisher account per identity. Publisher domain ownership and publication evidence remain publisher-reported; this is not domain verification.

## Configure the isolated service

Use a **new pilot service and a separate database**, with GitHub repository `vickhunter/affilio`, branch `feat/agent-revenue-poc`, root directory `/affilio-api` and its existing Go 1.23 Dockerfile. Do not change the existing production service's branch or database. Do not enable automatic deployments before checking the branch and environment.

See `affilio-api/hosted-pilot.env.example`. Set a real public HTTPS origin. Register an OIDC web application with the exact callback `<PUBLIC_BASE_URL>/auth/callback`; configure its issuer, client ID and client secret through the platform's secret settings. Generate a base64-encoded 32-byte connection encryption key and a random operator admin key. Never put either key in the agent client or this repository.

Apply migrations 001–014 in order to the **empty pilot database**, with `psql -v ON_ERROR_STOP=1`, before starting the service. Existing databases must apply only missing migrations once; this repository does not have a migration-history runner. The server fails startup when hosted sign-in is configured incompletely. With no `PUBLIC_BASE_URL`, hosted routes remain disabled and the local synthetic POC still runs.

Check `/health`, protected-resource metadata and the unauthenticated `/mcp` 401 challenge. Then complete sign-in, create an account and connect two separate clients. Do not reuse production API keys for this check.

## Client checks

Codex CLI 0.157.1 completed dynamic registration, discovery, PKCE, owner consent and token exchange against a local fixture owner session. Its temporary keyring entry was removed, and no saved MCP configuration was changed. This verifies client interoperability; real Google login, public HTTPS and Cursor still need the deployed pilot.

For an activated pilot, the installed CLI supports:

```sh
codex mcp add distributoor --url https://your-pilot-domain.example/mcp --oauth-client-registration dcr --oauth-resource https://your-pilot-domain.example/mcp
codex mcp login distributoor --scopes publisher --oauth-client-registration dcr
```

Use `--scopes maker` for a builder. A client omitting scope receives publisher access by default, or maker access when that is the owner's only linked account. A client requesting both roles must have both accounts linked and approve both permissions. Desktop app behavior must be checked separately from CLI login.

For Cursor, configure the hosted URL in its MCP settings and complete its browser login. Confirm discovery, the returned owner approval URL, refusal to commit a pending proposal, successful approved commit and refusal after disconnection. Do not declare Cursor compatibility solely from the SDK or Codex result.

## Merchant connection

The signed-in maker opens a product's **Connect Stripe** page. A restricted verification key is used for three read-only Stripe requests: authenticating account, price and webhook endpoint. It is not stored. The connection records account, price and endpoint IDs, mode, currency, configuration verification time and an encrypted webhook signing secret.

Only active, positive, one-time prices in USD/EUR/GBP are accepted. The endpoint must be enabled, point to the exact pilot product webhook URL, match the price's test/live mode and include paid checkout, asynchronous success and refund events. Live accounts must have charges enabled. Hosted opportunity discovery excludes operator-seeded connections that have not passed this check.

The owner can upgrade a legacy connection or rotate its signing secret/endpoint after verification. Account, price, mode and currency cannot change during rotation, and a disabled connection cannot be re-enabled through the owner form.

Configuration verification does **not** prove that the entered signing secret works, that the price appears in a checkout, or that money arrived. This stage does not create checkouts, retain a merchant API credential, reconcile Stripe invoices or move funds. The maker's checkout server must:

1. Preserve the `via` click UUID from the attributed redirect across the buyer journey.
2. Set `client_reference_id` to that click UUID on its Stripe Checkout Session.
3. Set session metadata `dtr_product_id` to its Distributoor product UUID.
4. Use the configured Stripe account, one-time price and currency.
5. Deliver Stripe's real paid-checkout and refund events to `/v1/webhooks/stripe/{product_id}`.

Use the maker's trusted checkout integration, with its secret server-side. Do not let the client choose arbitrary product IDs, prices or payment amounts. Run a Stripe-delivered test checkout and refund first; a manually signed fixture cannot prove this integration. The current contract covers the first paid checkout, not recurring subscription invoices.

## First commercial experiment

Start with three makers and three independently owned developer tutorials/blogs from the roughly 50 meetup developers. Confirm consent and existing traffic before matching. Each maker needs a working paid product and an approved offer; each publisher needs an article where the product solves a real reader problem. No participant outreach has been sent.

Measure elapsed time from agent connection to approved placement, relevant versus rejected suggestions, published placements, attributed clicks, unrelated paid buyers, refunds, cleared commissions, merchant payment reports and separately confirmed publisher receipts. Compare placement against a similar untreated article or a documented baseline when traffic permits. Preserve negative results instead of treating suggested earnings as achieved revenue.

The first success is one unrelated buyer, a Stripe-delivered attributed payment, matching maker/publisher ledgers, a refund window observed and a publisher receiving the merchant's commission. A test-mode record is not that success. Subscription support, external affiliate-program reconciliation, autonomous bidding, disputes/funding, service-fee collection and JEV/CLM ranking stay outside this first experiment.

## Verification

The test suite uses an isolated PostgreSQL 15 database. It covers signed OIDC tokens, nonce/issuer/audience/expiry/email verification, login replay, CSRF, identity versus email ownership, DCR redirect validation, PKCE failure, concurrent authorization-code exchange, token audience, refresh reuse revocation, publisher/maker separation, real HTTP MCP discovery/calls, separate owner approval, connection verification and immutable rotation fields.

Go 1.23.12 full tests, race tests and `go vet` passed. The CI coverage gate passed at 62.4% against its 25% minimum. The optional installed-Codex test runs when `AFFILIO_CODEX_BINARY` is set. It changes no MCP configuration, creates a temporary uniquely named keyring entry and checks logout cleanup. Never point `AFFILIO_TEST_DATABASE_URL` at production: existing integration fixtures reset the public schema.

Protocol references: [MCP authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization), [Streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). Stripe references: [authenticating account implementation](https://github.com/stripe/stripe-go/blob/master/account/client.go), [price retrieval](https://docs.stripe.com/api/prices/retrieve), [webhook endpoint retrieval](https://docs.stripe.com/api/webhook_endpoints/retrieve).
