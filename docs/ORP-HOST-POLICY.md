# Allagio host policy for ORP revoke and dispute

This note is Allagio host policy. It is not part of frozen ORP 0.3. It does not amend `docs/protocol/SPEC.md`, `docs/protocol/GAPS.md`, `docs/protocol/schema/0.3`, `docs/protocol/schema/examples`, or the bundled copies of those files. Those stay byte-identical to commit `8a6c6fa`, which `TestORP03MatchesFrozenCommit` pins.

## Obligation revoke

Only the obligor may revoke an obligation on this host. That is Allagio's implementation policy. It is not a requirement of frozen ORP 0.3. SPEC §15.1 does not say this.

A counterparty who believes the work was a breach, or a brand-safety problem, opens a dispute. That party does not revoke the other party's obligation. A `verified` or `revoked` obligation stays as stored. An `execute` that lost the race does not write over it.

## Dispute withdraw

SPEC §15.2 says the opener may withdraw. This host names the opener from stored dispute messages. Message ids are unique per sender.

A deal that is `disputed` and whose stored history is missing or ambiguous stays `disputed`. Withdraw answers HTTP 409 `state_conflict` with `The host cannot tell who opened this dispute.` The host does not guess the opener and does not repair the history. A later deploy must not backfill, rewrite, or delete those rows in order to make withdraw succeed. The cases are listed under [Missing or ambiguous history](#missing-or-ambiguous-history). Every one of them is that 409.

When the history names one opener and the caller is someone else, withdraw answers HTTP 403 `Only the opener may withdraw.` The deal stays `disputed`. That 403 is not used for a missing or ambiguous history.

## What the sender sees

A successful `open`, `respond`, or `withdraw` does not always store two inbox rows.

The host deal snapshot uses the wire names: `type` is `deal`, `from` is the host, and `deal` is the deal id. It is written only for a party whose stored owner is non-empty. On a 0.3 deal that owner is `offerer_owner` or `acceptor_owner`. On a 0.4 deal it is the seat's `owner`. A seat, offerer, or acceptor with an empty stored owner gets no snapshot.

The dispute copy is the stored Go value of the message the caller sent. Its names are `From`, `Type`, `Deal`, and `Body`. `From` is the caller's party id, and `Body.action` is the action. It is written only when the caller's grant owner id is non-empty, and it is stored under that grant owner, not under the seat's stored owner. A sender whose seat owner is empty and whose grant owner is set gets this echo and does not get a snapshot. A caller with an empty grant owner id does not get the echo. The host does not promise both rows.

The same Go-shaped copy is what a counterparty receives when that counterparty's stored owner is non-empty. That forward predates this host's sender echo. `GET /orp/inbox` returns every stored row for that owner id, including the snapshot, the sender's own echo, and any forwarded counterparty copy. It does not drop or merge them.

The owner deal-event list selects `message->>'deal'`. It includes the host snapshot. It includes neither the sender's echo nor a forwarded counterparty dispute copy, because both keep the deal id under `Deal`. That filter predates the sender echo.

`check_deals` returns `GET /orp/inbox` as stored. No client in this repository skips a row because `From` is the caller, and none refuses to answer its own echo. A consumer that acts on inbox rows must compare `From` with its own party id and not treat that row as a counterparty action.

## Missing or ambiguous history

The replay reads `orp_inbox` rows whose `type` or `Type` is `dispute` and whose `deal` or `Deal` is this deal, ordered by inbox `id`. Other inbox rows are ignored. `respond` is skipped and is not itself a rejection. A later copy of the same sender and message id with the same action is skipped, even when other fields differ. The host does not compare the rest of the body.

The history is missing or ambiguous, and withdraw uses the cannot-tell 409 above, when any of these is true:

- The query returns no row.
- A row is not JSON, or is not an object.
- `id` and `ID` are both non-empty and differ, or neither is a non-empty string.
- `from` and `From` are both non-empty and differ, or neither is a non-empty string.
- `type` and `Type` are both non-empty and differ, or the value that is used is not `dispute`.
- `body` and `Body` are both objects and their `action` strings differ, or neither is an object.
- `action` is missing or not a non-empty string.
- `from` is not exactly one party on the deal. On a 0.3 deal it is neither the offerer nor the acceptor. On a 0.4 deal the parties document does not decode, or that id is not on exactly one seat. A message from a non-party is this case.
- The same sender and message id appear again with a different action.
- `action` is not `open`, `withdraw`, or `respond`. `respond` was already skipped. Any other action, including `resolve`, rejects the history.
- A `withdraw` is seen while no dispute is open. That includes a withdraw with no earlier open, and another withdraw after one already closed the open.
- An `open` is seen while a dispute is already open.
- The rows that remain include no applied `open`. A history of only `respond` ends here.

If that replay produces an opener who is not exactly one party on the deal, the same cannot-tell 409 is used.

A history that applied an `open` and a later `withdraw`, and hit none of the rejections above, is not missing or ambiguous. Withdraw then answers HTTP 409 `The deal is in the wrong state.` The deal stays `disputed`. The host still does not guess or repair. A missing deal row uses that same wrong-state 409. A database error is HTTP 500. Neither of those is the cannot-tell result.

## Before the next deploy

Before this build is deployed, count the hosted deals in state `disputed` for which the replay above is the cannot-tell result. The count is read-only: `SELECT` only. No `UPDATE`, no `DELETE`, no snapshot restore, and no repair.

This run did not query the hosted database and did not deploy.
