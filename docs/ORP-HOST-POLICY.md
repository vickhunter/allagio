# Allagio host policy for ORP revoke and dispute

This note is Allagio host policy. It is not part of frozen ORP 0.3. It does not amend `docs/protocol/SPEC.md`, `docs/protocol/GAPS.md`, `docs/protocol/schema/0.3`, `docs/protocol/schema/examples`, or the bundled copies of those files. Those stay byte-identical to commit `8a6c6fa`, which `TestORP03MatchesFrozenCommit` pins.

## Obligation revoke

Only the obligor may revoke an obligation on this host. That is Allagio's implementation policy. It is not a requirement of frozen ORP 0.3. SPEC §15.1 does not say this.

A counterparty who believes the work was a breach, or a brand-safety problem, opens a dispute. That party does not revoke the other party's obligation. A `verified` or `revoked` obligation stays as stored. An `execute` that lost the race does not write over it.

## Dispute withdraw

SPEC §15.2 says the opener may withdraw. This host names the opener from stored dispute messages. Message ids are unique per sender.

A deal that is `disputed` and whose stored history is missing or ambiguous stays `disputed`. Withdraw answers HTTP 409 `state_conflict` with `The host cannot tell who opened this dispute.` The host does not guess the opener and does not repair the history. A later deploy must not backfill, rewrite, or delete those rows in order to make withdraw succeed.

When the history names one opener, a different party is refused. That refusal is not used for a missing or ambiguous history.

## What the sender sees

A successful `open`, `respond`, or `withdraw` stores two kinds of inbox row for the sender when that sender has an owner id.

`GET /orp/inbox` returns both, unchanged. One is the host deal snapshot: `type` is `deal`, `from` is the host, and `deal` is the deal id. The other is a copy of the dispute message the sender just sent. That copy is the stored Go value, so its names are `From`, `Type`, `Deal`, and `Body`, not the wire names `from`, `type`, `deal`, and `body`. `From` is the sender's own party id. `Body.action` is `open`, `respond`, or `withdraw`.

The owner deal-event list reads `message->>'deal'`. It includes the host deal snapshot. It does not include the dispute copy, because that copy keeps the deal id under `Deal`.

The host does not remove the sender's copy, and it does not merge it with the copy a counterparty receives. `check_deals` returns `GET /orp/inbox` as stored. No client in this repository skips a message because `From` is the caller, and none refuses to answer its own echo. A consumer that acts on every inbox row must compare `From` with its own party id and not treat that row as a new counterparty action.

## Before the next deploy

Before this build is deployed, count the hosted deals that are `disputed` and whose opener history is missing or ambiguous. The count is read-only: `SELECT` only. No `UPDATE`, no `DELETE`, no snapshot restore, and no repair.

A deal is in that count when its state is `disputed` and the stored dispute messages do not leave exactly one unmatched `open` from a party on the deal. `respond` does not name the opener. Two `open`s with no `withdraw` between them, a message the host cannot read, or no `open` at all, are missing or ambiguous.

This run did not query the hosted database and did not deploy.
