# Allagio host policy for ORP revoke and dispute

This note is Allagio host policy. It is not part of frozen ORP 0.3. It does not amend `docs/protocol/SPEC.md`, `docs/protocol/GAPS.md`, `docs/protocol/schema/0.3`, `docs/protocol/schema/examples`, or the bundled copies of those files. Those stay byte-identical to commit `8a6c6fa`, which `TestORP03MatchesFrozenCommit` pins.

## Obligation revoke

Only the obligor may revoke an obligation on this host. That is Allagio's implementation policy. It is not a requirement of frozen ORP 0.3. SPEC §15.1 does not say this.

A counterparty who believes the work was a breach, or a brand-safety problem, opens a dispute. That party does not revoke the other party's obligation. A `verified` or `revoked` obligation stays as stored. An `execute` that lost the race does not write over it.

## Dispute withdraw

SPEC §15.2 says the opener may withdraw. This host names the opener from stored dispute messages. Message ids are unique per sender.

A deal that is `disputed` and whose stored history is missing or ambiguous stays `disputed`. Withdraw answers HTTP 409 `state_conflict` with `The host cannot tell who opened this dispute.` The host does not guess the opener and does not repair the history. A later deploy must not backfill, rewrite, or delete those rows in order to make withdraw succeed.

When the history names one opener, a different party is refused. That refusal is not used for a missing or ambiguous history.

## Before the next deploy

Before this build is deployed, count the hosted deals that are `disputed` and whose opener history is missing or ambiguous. The count is read-only: `SELECT` only. No `UPDATE`, no `DELETE`, no snapshot restore, and no repair.

A deal is in that count when its state is `disputed` and the stored dispute messages do not leave exactly one unmatched `open` from a party on the deal. `respond` does not name the opener. Two `open`s with no `withdraw` between them, a message the host cannot read, or no `open` at all, are missing or ambiguous.

This run did not query the hosted database and did not deploy.
