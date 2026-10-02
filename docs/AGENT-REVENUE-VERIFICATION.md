# Agent revenue verification · 2026-09-30

Result: the local technical POC passes. Commercial effectiveness and real provider/client integrations remain unverified.

## Checks completed

| Check | Result |
|---|---|
| Full Go suite on Go **1.23.12**, PostgreSQL 15 | Passed |
| `go test -race ./... -count=1` | Passed |
| Repository `make ci-test`, expanded to all packages | Passed; **62.6% total coverage**, 25% required |
| `go vet ./...` | Passed |
| Formatting of changed Go files / `git diff --check` | Passed |
| `node --check` for WordPress Earnings | Passed |
| Shell syntax, Python compilation, plugin/report JSON | Passed |
| Launcher restart while port occupied | Refused before resetting the demo database |
| Local report credential scan / MCP config permissions | No credentials in report; config owner-readable only |
| Browser preview | Relevant email content matched; gardening context returned no suitable offer |

Tests use the dedicated disposable database on port 55439; the runnable demo has a separate container and `affilio_poc` database on port 55440. No production database was accessed.

## Behavioral evidence

- Two real stdio MCP processes use separate maker and scoped publisher credentials. They complete product/offer setup, contextual discovery, proposal preparation, approval, commit, local artifact receipt, matching maker/publisher earnings and refund reversal.
- A $49 signed paid-event fixture creates $9.80 held commission. A half refund leaves $4.90; a full refund leaves $0. The fresh runnable report keeps these balances labeled `test`.
- Unpaid/free checkouts do not book sales. Async success books once; a second event for the same payment cannot book again. Wrong test/live mode and product metadata are rejected.
- Each product's signing secret and encrypted ciphertext are bound to its connection. Cross-product credit/refund attempts fail.
- Refund-before-sale stays pending and recovers through the worker. Repeated cumulative refunds and changed redelivery payloads cannot double-reverse or replace the first accepted event.
- Recovery after financial commit but before webhook acknowledgement creates no duplicate commission. Minor-unit proration handles the largest supported payment without integer overflow.
- Concurrent payout batches create one reservation. A refund cancels an affected pending batch; a refund after confirmed receipt preserves historical payment and records the remaining debt separately.
- Ordinary payout batches exclude test commissions. Batches and balances distinguish maker, currency and payment mode. Maker-reported payment does not become confirmed receipt automatically.
- `confirm=true` cannot replace owner approval. Cross-owner proposal reads, legacy-join bypasses and agent payout confirmation are refused. Revoked keys stop authenticating; revoked placements stop new attribution. Removing the placement query parameter does not bypass permission.

## Review and limits

Source review focused on authority, canonical event storage, retry recovery, refund rounding, reservation races, test/live separation and direct merchant payout grouping. Findings were fixed and verified before completion. This was a local review using the caveman-review skill; GitHub Bugbot was not run because no PR was opened.

The demo seeds readiness and simulates owner approval. Publication evidence is a local Markdown artifact and is labeled publisher-reported. Payment evidence is a synthetic signed payload, not a Stripe test checkout, live purchase, independently reconciled provider report or bank transfer. The browser-tested surface is the local proof page; WordPress's full running integration was not exercised.

The POC verifies its own stdio transport, not an actual Cursor/OpenClaw/Grok/Muse/dots connection or hosted MCP OAuth conformance. Before a commercial pilot: verify merchant/account/price ownership and checkout propagation, add dispute/funding and fee-collection operations, and use a real publisher's approved surface and received commission as the success criterion. The 10% platform fee is proposed terms; no fee invoice or collection was performed.
