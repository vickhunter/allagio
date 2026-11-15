# Security policy

## Reporting a vulnerability

Email **motricalavictor@gmail.com** with the details: what you found, how to
reproduce it, and what an attacker could do with it. Please do not open a public
issue for a live vulnerability.

You can expect a first response within a few days. If the report is confirmed,
it gets fixed before disclosure; credit is given if you want it.

## Scope

- The hosted service at `allagio.dev` (API, MCP, ORP host, owner dashboard).
- This repository when self-hosted: `affilio-api/` (Go backend) and
  `affilio-wp/` (WordPress plugin).

Out of scope: vulnerabilities in third-party dependencies that have no exploit
path through Allagio, and issues requiring you to already hold an admin key.

## Notes for researchers

- `X-API-Key` (site), `X-Maker-Key` (maker), `X-Admin-Key` (admin) and `orat_`
  join bearers are the credential classes. Anything that bypasses them, mints
  them, or reads another party's deals/messages is in scope and interesting.
- The admin key must never reach a public frontend — a finding where it leaks
  is a real bug.
- Signed ORP envelopes are Ed25519 over JCS-canonicalized JSON; host key public
  in `/.well-known/orp.json`. Forgery of a `sig` or of a deal certificate is
  a high-severity finding.
