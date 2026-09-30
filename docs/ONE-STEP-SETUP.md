# Setup through the user's existing agent

The design follows the [PostHog wizard](https://github.com/PostHog/wizard): one
entry point detects the project and carries integration knowledge in skills.
Distributoor uses the builder's existing agent. The installer bootstraps the
connection; its skill directs checkout edits, tests, assessment and reporting.

## Builders

`packages/distributoor-setup` runs locally or from an npm archive. It has no
runtime dependencies and requires Node 20+. The public package and hosted pilot
are not released yet; do not advertise the planned public npx command as working.

Online setup verifies maker OAuth discovery before writing. Codex uses temporary
CLI configuration, explicit maker browser OAuth and its normal coding session.
A new verified identity gets its own maker automatically, without a legacy key.
Matching email never claims an existing account. Cursor and Claude get native
project MCP configuration and discoverable skills. Cursor still needs the user
to open its agent and invoke the skill; its full desktop journey remains a pilot
check. Non-interactive runs install files without starting another agent.

The skill detects the actual checkout, reuses products on retries, preserves
metadata, runs project checks and reassesses the deployed site. The owner supplies
commission terms and connects Stripe in the owner form. Its report separates
installation, readiness, verified payment configuration, a Stripe-delivered test
sale/refund and listing. No real builder checkout has been instrumented by our
installer verification.

## WordPress publishers

Fresh activation opens **Affilio → Settings**. **Connect and find opportunities**
creates the site account, verifies the API and schedules published-post analysis
in batches of ten. This action explains which content goes to Affilio. No posts
are uploaded before it, no signup or API-key copying is needed, and insertion
remains an owner action.

The plugin creates a random 32-byte installation credential and stores it locally
with autoload disabled. The backend stores its hash. Registration retries with
the same key/domain return the same site, including after a lost response. A key
reused for another domain is rejected; a domain alone never claims an existing
site account. Domain ownership itself is not independently verified by this
legacy registration route. Developer API overrides remain available; the new
plugin requires this branch's backend support.

Settings show connection, scan count, completion and retry. WordPress cron runs
the work and resumes a failed batch. Existing manual credentials stay intact.
Sites with WP cron disabled need their normal external cron runner. Installation
and analysis are not proof of revenue. Native WordPress connection remains
separate from hosted agent identity and grants no agent owner-approval authority.

The missing editor proxy file that prevented activation is restored. Its routes
check access to the individual post and recommendation ownership. Existing
WordPress affiliate screens continue using legacy publisher APIs.

Verification: seven installer tests, portable archive execution, full Go tests,
handler/database race tests, vet and PHP setup checks passed. In an isolated
WordPress 6.6/PHP 8.2 fixture, Playwright verified Connect → scan → dashboard,
settings and disconnected state (three passed; the optional manual-key test
was skipped). The local fixture ran the scheduled scan explicitly through WP
CLI because its synthetic domain does not resolve for native loopback cron.
Real owner sign-in and a real builder agent's checkout integration remain pending.

## Release and pilot

1. Deploy to the isolated pilot with its own database and OIDC configuration.
2. Publish the reviewed npm archive under an owned namespace and distribute the
   reviewed WordPress ZIP. The repo remains private; npm publication and WordPress
   directory submission have not happened.
3. Use a real builder repository and an independent WordPress site to verify
   client login, checkout integration, recommendations and test sale/refund.
4. Measure setup time, owner steps, failed installs and time to first attributable
   conversion before inviting all 50 developers.

Provider references: [Cursor MCP](https://prod.cursor.com/help/customization/mcp),
[Cursor skills](https://cursor.com/docs/skills),
[Claude project MCP](https://code.claude.com/docs/en/mcp).
