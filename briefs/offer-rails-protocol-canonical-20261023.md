# Offer Rails Protocol: canonical 0.3 freeze (D-099)

Date: 2026-10-23. Base: `main` at `27e0a63`. Docs only: no deploy, no hosted change, no `CLAIMS_CHECK` change, no secrets.

**Canonical spec:** [`docs/protocol/SPEC.md`](../docs/protocol/SPEC.md), with schemas in [`docs/protocol/schema/0.3/`](../docs/protocol/schema/0.3/).
**Superseded:** draft PR #68 as a wire format, in full. Draft PR #71's `schema/` directory, which is carried forward here with the changes below.

## Decision

The 21 October critic panel found that #68 and #71 were two protocols (GAP-06). They disagreed on the envelope, the version string, party ids, hashes, the deal taxonomy, and who may send receipts and consent. One had to win on the wire, because implementers cannot negotiate between two drafts.

**#71's wire format wins.** The reasons:

1. **Its schemas are enforceable.** Every object is closed and checked in Ajv strict mode, with valid and invalid examples. #68 said "ignore unknown fields". That makes privacy a promise, where a closed schema makes it a check, because a message with an `email` field is simply invalid.
2. **It is not affiliate-only.** #71 composes terms from obligations (`place`, `repost`, `comment`, `pay`, `credit`, `share`), so any combination of digital spaces is one deal shape. #68 had five fixed `deal_type` shapes, and each new growth type would have needed a new shape.
3. **Trust sits in the right place.** In #71 only the host sends `consent` and receipts, and an execute is a claim. #68 let agents send consent and let obligors submit their own receipts. The panel called those forgeable (GAP-02).
4. **Ids and versions carry no personal data.** `orp:<host>/<random>` and `"0.3"` are opaque. #68's `orp:party:maker_<name>` leaks a name into every message, and its SemVer `0.3.0` invites patch-level drift.
5. **Hashes can be reproduced.** `sha256-` over RFC 8785 JCS gives one answer in every language. #68's `sha256:<hex>` named no canonical form.

**#68 contributed ideas, not wire.** These #68 ideas are adopted in the #71 shape:

| #68 idea | Where it lives in 0.3 |
|---|---|
| Spaces as first-class objects | new `space` message (`put`, `withdraw`, `listed`) |
| Named growth deal types | `kind` hints: `affiliate`, `cross_sell`, `link_exchange`, `traffic_swap`, `repost` |
| `via` with a `ref` alias | `Tracking.param` and `Tracking.alias` |
| `private_test` and test clicks | the `test` click class and test pools |
| `clicks_by_source` | outcome row `sources` |
| "Afternoon core" | conformance level L1 |
| Test names `C1-UNIVERSAL-LINK-NO-ADAPTER` and `C2-NO-EMAIL-NO-SIGNUP` | kept as L1 tests |

**Other changes to #71** (SPEC Appendix A):

- `ask` renamed to `request`, so the message never reads like the `ask` step mode.
- A new `payout` message for what is owed and paid. Pay obligations get a `hold`.
- A `checks.claims` capability and per-offer `checks`, plus a `claims_unchecked` escalation reason.
- `suggest` can carry `join`, so joining mid-task is one call.
- `attestation` reputation weight drops from 0.5 to 0.
- The behavior rules are written down: tracking T1 to T12, approval order, consent H1 to H5, proof P1 to P7, settlement S1 to S5, persistent idempotency, and text hygiene.

`npm test` in `docs/protocol/schema/` checks every schema, every example, every `terms_hash`, and every JSON block in SPEC.md. 8 of 8 tests pass.

**DCP v0.1** (`docs/dcp/`) stays the description of the hosted API. SPEC Appendix C lists where hosted differs from ORP 0.3.

## Blocker gaps after the freeze

"In scope" means the spec now states the rule. It does not mean hosted implements it. None of these is fixed in production by this change.

| Gap | Tag | What the spec does / what is still open |
|---|---|---|
| GAP-01 Universal link gated on plugin-ack | **in scope for freeze**; implementation deferred | T6: a host must count human clicks from the first request on any surface, with no adapter, plugin-ack, receipt or execute. T7: the first human click verifies the placement. Hosted `/v1/go/<placement_id>` already records a hit before plugin-ack, but `placement_counts_as_live` still needs the plugin to report `placed`. |
| GAP-02 Forgeable receipts and seller-reported conversions | **in scope for freeze**; implementation deferred | P2: only the host issues receipts, except counterparty `attestation`, and obligor self-receipts are forbidden. P3: attestation adds no reputation. P4: payer-reported conversions can release only the payer's own money, and only when the terms allow it. P5: refunds weigh at least as much as sales. One disagreement with the panel: it wanted no money on attestation at all. The spec lets a payer pay on its own report, because it is the payer's money and the payee agreed in the terms. It never earns reputation. The nonce and challenge scheme and mandatory party keys are not specified. |
| GAP-03 Zero-signup promise broken | **in scope for freeze**; hosted deferred | `join` accepts `{}` and refuses email, domain or name. `suggest` with `join` returns a token in one call. Claim and payout details are collected later, at the host. Hosted has `POST /v1/agent-tokens {}` (#69), but no ORP `suggest`, no `/.well-known/orp.json`, and no ORP envelope. |
| GAP-04 Identity not bound, sybils cheap | **partial**; deferred | Host-scoped random ids. Optional party keys at join, served at `{endpoint}/keys/<opaque>`, and once registered they are required on binding messages (E9). Pairwise 1/k and counterparty-standing weights. Reputation stays `null` until the party is claimed by a verified person or domain (SHOULD). Still open: DID or domain JWKS binding, and graph-level sybil detection. |
| GAP-05 Claims check is a stub while auto modes exist | **in scope for freeze**; real checker deferred | Hosts publish `checks.claims` and every result carries `checks`. A host without a `real` check must escalate auto-placement with `claims_unchecked`, whatever the party's mode. This matches hosted's `ask` default for placement publish (#73). `CLAIMS_CHECK` is unchanged. A real `PageClaimChecker` on hosted is still open. |
| GAP-06 Two disagreeing drafts | **fixed by this freeze** (document level) | One spec, one schema set, #68 superseded, and #68-shaped messages fail validation (`examples/invalid/envelope-68-shape.json`, `version-semver.json`, `party-id-role-prefix-68.json`). Still open: the behavioral conformance suite (L0 to L3) is named but not written, so no host can claim a level yet. |
| GAP-07 "No personal data" claim false; no privacy, terms or DPA | **in scope for freeze** (wording and data inventory); legal deferred | The claim is now "no personal data exchanged between parties" (§7). The spec lists what crosses between parties and what the host may hold, and requires `policies.terms` and `policies.privacy` for L1. Draft privacy and terms pages exist (D-057). Still open: DPA, Art. 13 notice, DPIA, lawful-basis analysis, and counsel review. |
| GAP-08 Settlement outside ORP | **partial**; deferred | `payout` records (held, payable, sent, paid, reversed, failed), a default `P30D` hold for pay per conversion, the `settle` step, payee confirmation, and `not_paid` disputes. ORP still never holds money. Still open: settlement profiles (Stripe Connect, x402), escrow, KYC, tax, payout SLA, and the per-maker paid-history panel. |

## Other gaps the spec now touches

These are majors and minors, not blockers. Each is only addressed in the text.

- GAP-09: the `test` click class and test pools are specified. A headless-render or manual-claim listing path is not.
- GAP-10: disclosure is required on place, comment and quote, and `found.disclosure` fails a receipt. Locale-aware platform markers are not specified.
- GAP-11: P6 requires rechecks (day 1, weekly, at the end) and holds money until the last one passes. Not built.
- GAP-12: only human clicks count. Velocity and entropy checks are left to the host.
- GAP-15: signatures are still optional, except once a party registers a key. Making them mandatory is a 0.4 candidate.
- GAP-16: the MCP binding is specified. A2A and x402 bindings are not.
- GAP-19: T3 requires random per-click ids, and T11 says direct links never earn per-click or per-conversion pay.
- GAP-20: text hygiene is a reject rule. There is no shared sanitizer or test corpus yet.
- GAP-21: `max_chars` is set per obligation. There is no global cap.
- GAP-22: outcome `sources` covers clicks by source.
- GAP-25: idempotency keys are kept at least 7 days in persistent storage, with a distinct `replay` error.

Untouched: GAP-13 (only affiliate is live on hosted), GAP-14 (llms.txt), GAP-17 (DPIA and retention), GAP-18 (channel compliance), GAP-23 (k_min and the long tail), GAP-24 (distributoor naming), GAP-26 (connector smoke test).

## Follow-ups

1. Update the #71 TypeScript SDK to the canonical schemas. The changes are the `request` rename, `space` and `payout`, `via` click ids instead of `orp=<code>`, attestation weight 0, `claims_unchecked`, and suggest-with-join.
2. Write the behavioral conformance suite for L0 to L3 and run it against the SDK's reference host.
3. Plan hosted ORP 0.3 as separate decisions: `/.well-known/orp.json`, the envelope endpoint, `suggest`, and a real claims check. Each needs its own approval. This freeze does not authorize them.
4. Before publishing under Apache-2.0, add a LICENSE and NOTICE for `docs/protocol/`, choose the canonical schema host (`offerrails.com/orp/schema/0.3/`), and get a legal read of §7.
