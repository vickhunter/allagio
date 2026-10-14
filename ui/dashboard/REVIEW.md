# D-032 UI self design review (against /workspace/shared/design-rules.md)

Point of view used: Offer Rails is a quiet rail the owner's agent rides on. The human signs once, sees exactly what will happen, and can stop or undo anything in one tap. One value applied everywhere: **calm control**. Everything is prefilled, nothing hides, every automatic action names the rule that allowed it.

## Rules followed
- Brand POV / one value / form matches signal: copy and layout say "your agent did it, you stay in control". Plain mode labels only; hard checks always visible; approved_by in plain words on every item.
- Same product, no arbitrary colors: every color is a token from the owner workspace (`--paper --panel --ink --muted --line --accent --action --green --fail*`); same Archivo + IBM Plex Mono subsets, same mono "station flags", same button shapes. Red = failure or destructive, green = passed / approved automatically, blue = the one primary action, accent = focus ring and box position marker.
- Anticipatory details: the approved device code in the top bar, "found in your repo / links" per site, mini diagrams for box position, "Since Sat 3 Oct" period, scan-boundary sentence under the listing card, return target named after the source ("return to Cursor"), real zero vs "No earnings data yet".
- Troubleshooting built in: failed readiness signal shows the reason and "your agent can add one"; save error keeps answers; empty scan fields say what to do; signed-out Resume says how to fix it.
- A bit of life: the rail-line in the setup header with a small dot riding from agent to project (off under prefers-reduced-motion).
- Full flows, not components only: setup -> success; digest -> undo -> toast -> redo/confirm; pause -> confirm -> paused banner -> resume -> confirm.
- Reviewed by using it at 390 and 1440 (Playwright walk + screenshots), not screen by screen.

## Rules knowingly bent, and why
1. **Motion "frame by frame, iterate until smooth"**: the rail dot and toast drain bar got two passes, not dozens. Both are small, non-essential and disabled for reduced motion. Worth one more pass with the owner's eye.
2. **"Do not default to charts" / DataFast inspiration**: DataFast shows sparklines per project; I used numbers only. No time series exists in the spec payload yet, and fake sparklines would be dishonest. Add one per project once `GET /v1/digest` returns daily clicks.
3. **"Explore generative, alive interactions"**: kept deliberately conventional (form + list) because both screens are trust screens for an auto-publishing product; surprise here costs trust.
4. **Brand name**: the snapshot app still says "distributoor"; these screens say "offer rails" in the same wordmark style. Flag for Victor/AI Engineer when wiring.
5. **Max 4 questions with two roles**: ticking "I also publish / I also have a product" adds the second role using the scan's values without extra questions (editable later in the dashboard), so the page never exceeds 4 questions. The second role's answers are not reviewed on this page.
6. **Undo always works vs toast** (v2): Undo now revokes on the server on the tap (no client hold, `keepalive` so closing the tab still revokes). Redo is a server restore offered only while the toast is open (6 s); the server has the final word (409/403/422 explained in plain copy). After the window the Redo button disappears.
7. **Toast covers content at 390** for its 6 s; it is small and dismisses itself. Accepted to keep the no-modal rule.
8. **frontend-ui.md libraries not used**: they are React/Tailwind; the brief asks for plain HTML/CSS/JS that drops into Go templates. Zero runtime dependencies.
9. **axe-core (vendor/axe.min.js, 650 KB, MPL-2.0) is committed** for offline tests only; it is never loaded by the pages.

## Not done / for later
- The approval "Change" page (policy form, caps, min relevance) is out of scope; the links point to a proposed `/owner/settings/approval`.
- No dark mode (the workspace has none).
- Real-copy check in Italian at 140 chars done with an English 140-char string; Italian placements tested at normal length.

## v2 review (server-backed undo/restore, 2026-10-10)
- Honesty over comfort: the item never claims "Box removed" until the plugin confirms (`plugin_state`). While pending it says "Removed · leaving your post shortly"; offline says clicks already stop at the link (true: the go link stops as soon as the revoke lands) and the box comes out on reconnect.
- No optimistic UI for pause, resume or restore: the state changes only after a 2xx. Undo shows "Removing…" for the round trip, which is the one place I'd consider optimism later; I kept it honest because a failed revoke must not look done.
- Every server refusal has its own sentence; raw JSON is never shown (tests assert no braces or quotes in the note). 409 is split by `reason` (window_expired / paused / neutral fallback) so "too late" is never said when it wasn't about time.
- Undone boxes: dropped the 50 % opacity (failed axe contrast) for a dashed border + muted line-through, which still reads "gone" and passes contrast.
- Bent: failure copy shows twice (in the toast for a few seconds and as a note on the item). Deliberate: the toast disappears, the note stays where the person is looking.
- (v3) superseded: `placed` is now decided; null is never treated as placed.
- Bent: polling `GET /v1/placements/:id` every 4 s is the simplest honest source for plugin progress; a push channel would be better if one exists.

## v3 review (plugin_state enum, 422 check_text, polling, live counts)
- No assumptions: null or unknown `plugin_state` says "Status unknown" (before, null after a restore counted as placed; removed).
- `failed` (proposed) gets its own amber warning note, distinct from the neutral offline note: offline fixes itself, failed needs the owner, so the UI also stops polling on failed.
- 422 shows the server's `check_text` verbatim; local fallback sentences exist per check id only for a missing text.
- Polling respects the person's machine: 4 s for a minute, 30 s after, stop at 10 min with "Still waiting for your site", nothing while the tab is hidden.
- Bent: amber (#6b4500 on #fbf2dc) is a new color not in the workspace tokens. Justified: the workspace has only "fail" red, and a failed post update is a warning the owner can fix, not an error of ours. Contrast passes axe.

## Acceptance checks (UI, run by tests/run_tests.py at 1440 and 390)
- Max 4 prefilled questions on setup; plain mode labels only; server `mode_preselected` respected; Continue emits setup-submit.
- Undo sends the revoke on tap (no hold) and survives closing the tab.
- Redo outcomes 200 / 409 (window_expired, paused, neutral) / 403 / 422 (check_text, fallbacks) each show their plain sentence; never raw JSON.
- plugin_state pending / placed / removed / offline / failed / unknown each show their copy.
- Polling: 4 s x 15, then 30 s, stop at 10 min with "Still waiting for your site"; no polling while hidden, resumes on visible (fake clock).
- **"Live now" (site and project) and account "Live items" update right after a successful undo and a successful restore; a refused restore changes nothing** (test `digest_live_now_updates`).
- Earnings: null = "No earnings data yet", zero with data = €0.00.
- axe 0 violations; no horizontal overflow; 0 requests to /v1/go/.

## v4 review (failed + Retry, 2026-10-10)
- The failed note now says what failed: "Removed · box still in your post" (removal) or "Not back in your post yet" (restore), replacing the vaguer "post not updated". A failed removal also says clicks already stop at the link (spec 03:35).
- The server's `plugin_failed_reason` is shown word for word in small text; nothing when null. Raw JSON is never shown (plainString guard).
- Retry sits inside the amber note, next to the problem it fixes, labelled by the failed action. 44 px tall, full width at 390, busy label + disabled + aria-busy, one request at a time.
- Retry never touches status, so it can never move "Live now" and a retried removal can never read Live (impossible pairs show "Status unknown").
- 409 `not_failed` tells the truth ("This box already changed…") and refetches instead of guessing.
- (v4.1 fixed) The busy Retry button no longer fades: full-contrast text (#4d3200 on #fbf2dc), dashed darker border and a small spinner (static under reduced motion).
- (v4.1 decided) On a failed restore the item button is labelled "Leave it removed" instead of "Undo" (see v4.1).

Acceptance checks added (v4): failed note + reason + Retry label per failed_action; retry busy + single request; retried removal -> removed with Live now unchanged; retried removal reporting placed -> Status unknown; failed-restore retry 422 shows check_text word for word and stays failed; retry 409 not_failed shows the message and refetched status; unknown 409/422 show server text.

## v4.1 review
- A failed restore is not live: it is excluded from "Live now", the account live total (and, server side, earnings). Items emit a +1/-1 when they start or stop counting, so restore -> failed -> retry -> placed reads 2 -> 3 -> 2 -> 2 -> 3 for the site. The note says "Clicks stay off until it's back in your post."
- Undo next to Retry on a failed restore: **kept, relabelled "Leave it removed"**, not hidden. Why: on a failed restore the box is not in the post and clicks are off, so "Undo" was ambiguous (undo what? the restore that never landed?). The real choice the owner faces is "try again" vs "give up and leave it removed", and both should be visible; hiding it would leave Retry as the only exit and push people to retry forever. It calls the same revoke `{reason:"undo"}`, changes no count (the item already did not count) and opens no Redo toast (Retry was the way back).
- Busy Retry: full-contrast text + spinner instead of 55 % opacity.
- 409 text is read from `message` (always present), 422 from `check_text`; the `reason_text` fallback is gone.

Acceptance checks added (v4.1): failed restore never counts in Live now (restore -> failed back to the undone value; retry -> pending unchanged; placed -> +1); "Leave it removed" sends revoke reason undo with no count change and no toast; busy Retry keeps opacity 1 and full-contrast text with a spinner.

## v4.2
- A failed restore preview now uses the not-live look (dashed border, panel background, muted text and link) so it never reads as live; no strikethrough, so the copy stays readable. Failed removals keep the removed look with strikethrough. Test `failed_restore_preview_not_live` checks computed styles against a live box.

## v4.3 (no optimistic state, 2026-10-10)
- Rule: nothing reads removed or placed until the server confirms. Pending copy: "Leaving your post shortly" (undo, retried removal), "Taking it out of your post" (Leave it removed), "Going back into your post" (redo, retried restore). Offline: "Clicks off · site offline" / "Going back · site offline". Failed removal: "Clicks off · box still in your post".
- Copy changed on purpose from earlier dictated strings: "Removed · leaving…", "Removed · site offline", "Removed · box still in…", "The box stays removed." and the toast "Removed from <site>." all claimed removal before the plugin confirmed it. Refusal suffix is now "It stays undone." (true: the server keeps it undone), 403 says "undid", toast says "Undone. Clicks are off; the box is leaving <site>."
- "Leave it removed" shows only for a failed restore; a failed removal (including one that failed after Leave it removed) offers only "Try removing again".
- "Live now" rises only on confirmed `placed` (a pending restore no longer counts), drops at once on undo / leave it removed.
- Pause / resume: confirm button reads "Pausing…" / "Resuming…" (aria-busy) and modes/banner change only on a 2xx.
- Wiring to D-032a deliberately not done: waits for AI Engineer's parity table (see INTERFACE.md).

Acceptance checks added (v4.3): `no_optimistic_audit` (MutationObserver log of every state/note/toast/mode change across undo, redo, retry removal, retry restore, pause, resume; fails on "removed"/"Live" before the mocked server confirms; verified it fails when the old "Removed · leaving" copy is put back), `no_optimistic_leave`, `leave_then_failed_removal`, `leave_never_answered_offline`; live-count tests now require `placed` before +1.

## v5 review (wired to PR #21 @ d17b5dd, plus the Product Lead review's expected changes)
- Wired against the parity table and handler code only; `PARITY.md` lists every field. Where the PR contradicts an agreed rule (off-enum 409, no item status, single earnings number) the rule was kept and the gap is an open item, not a UI change to fit.
- Honesty: with no `status` on digest items the UI says "Status unknown" and asks the server once, instead of guessing from `plugin_state: pending`. The d17b5dd earnings total (currencies and maker commission added together) is never shown as a total; each section shows its own figure.
- Money: one line per currency, stacked, same type scale; the maker's figure is called "Commission to pay" so it can't be read as income. Tested at 390 with two currencies, no overflow.
- Rule line: "Approved by your rule: <server text>" in the server's own words; hidden entirely when the server gives no reason, rather than a vague "Approved".
- Long lists: 140 items, 6 shown + "Show all 140 items"; status reads queued 4 at a time so a long list can't flood the API.
- Setup: when automatic modes aren't available they stay visible but disabled, each with one plain sentence tied to it by `aria-describedby`; nothing disabled is ever preselected.
- Bent: real mode drops fields the PR doesn't send (post title, position, approved_by, rules version) instead of filling them; items look sparser than the fixtures until those fields exist.
- Bent: paused state costs one mode read per project on load (no paused flag in the digest yet).

## v6 review (wired to PR #21 @ fb493f0, PR names win, plus the must-fix items)
- Fewer requests, more truth on load. The per-project mode reads and the per-item status reads are gone, because `paused`, `plugin_state` and `live_now` come with the digest. A pending item no longer says "Status unknown". It says what is happening ("Waiting for your post to update") and never placed or removed until the poll answers.
- Item cards now show the post title, position, link text, label and clicks from the server, so real mode looks like the fixtures again.
- The approval reason is said once at the top and quietly under each "Ask me every time" label. In setup, auto/yolo stay visible but disabled, and the server's sentence is tied to each by `aria-describedby`.
- Totals: the grid was a fixed 5 columns on a grey background, so a 6th tile wrapped and left an empty grey cell at 1440. It is now a wrapping row where tiles grow to fill each line and draw their own hairlines. `totals_grid_layout` checks 4–8 tiles at both widths: every row fills the full width and nothing grey shows behind.
- Hard checks come only from `GET /v1/policy/hard-checks`. If that call fails, setup says so in one line instead of showing an invented list.
- Must-fix (a)–(e) wired as expected behaviour. The key-context line is deliberately plain ("need the dashboard") rather than explaining sessions.
- Bent: under the claim-check stub, saving setup stores manual over a saved auto/yolo (open item); resume shows manual, not `applied[].mode`, while the stub is on (open item).

## v7 review (design system from the PostHog + Amp study, approved 10 Oct 2026)
- Added DESIGN.md (the reusable template) and tokens as CSS variables: 3 surfaces plus an inverse, warm neutrals, and one accent each for setup, digest and approvals. Every text pair is AA (table in DESIGN.md §1.3). The v6 muted grey (4.39 on surface-2) was retired.
- Keyboard-first digest: J/K/Enter, R presses only a visible Undo or "Leave it removed" (never Redo, a listing, or a failed removal), A only explains. ⌘K/Ctrl+K palette with keys, ? help, 3px focus ring, and keys are ignored while typing.
- Keycap on setup Continue only, with a reassurance line. Skip for now leads to one Continue setup pill (lesson 2). Skeletons use the real layout classes. "All clear." empty state with one action and BLIP. Undo and kill-switch outcomes say what was and wasn't reverted (lesson 5). 409 and 422 text stays verbatim.
- Side panel (a full-screen sheet at 390) and a modes modal with a blurred backdrop. Both trap focus, close on Esc and return focus.
- Rules kept: no new endpoints, no optimistic states, per-currency rules untouched, go links never clicked (preview link text in the panel is plain text), and session-only actions stay session-only from the keyboard and the palette.
- Not applied, and why: the Approve keycap and a working A key (nothing on D-032 lists items awaiting a decision; inventing a queue would be wiring against guesses); the real next-digest time in API mode (no schedule field; the fixture carries a proposed `next_digest_at`); a settings modal beyond approval modes (no settings read on the digest); the landing hero (no landing page in this repo, spec in DESIGN.md §9).
- Test note: axe now waits for finite entrance animations to finish, so the 700ms first-load fade isn't measured mid-fade. Two v6 asserts that pinned exact colours were updated to the new token values.
- Result (10 Oct 17:40): 98/98 at 1440 and 98/98 at 390 (84 v6 + 14 v7 design tests), contract test 10/10, axe 0 violations on every axe-checked screen, 0 requests to /v1/go/. Before/after pairs: proof/d032-ui/v7-design/*-before-*/ *-after-* (before = v6 code via tests/design_shots.py).

## v8 review (wired to PR #21 @ 6042dcb, plugin key UI, 12 Oct 2026)
- Re-wired to 6042dcb (the brief said f7bf688; the head moved to 6042dcb during the work and that is what was read). PR body saved as `PR21-body-6042dcb.md`; fixtures and `tests/contract_spec.json` from `plugin_keys.go`, `agent_account.go`, `agent_entry.go`, `entry/policy.go` at that ref.
- "Showing the latest 50 of 212" uses `total_items`; no note when everything is shown. Pause/resume read `effective_mode` and quote the row's `approval_message`. A saved setup shows its mode read-only with one line, because a save keeps the stored mode.
- Plugin key, shown once: monospace read-only field, Copy with a lasting "Copied ✓", an amber line "We won't show this key again", where to paste it, and a keycap Continue gated on copy or "I saved it". It's the only primary on that screen, and the gate line tells you why Continue is disabled. The key is cleared from the field on Continue and is never stored, emitted or put in a URL.
- Settings panel per site: prefix and date only, plus Generate (only with no active key), Rotate (inline confirm, then the same shown-once block) and Revoke (danger modal: what stops working, what doesn't; focus starts on Cancel). 401/403 show "Sign in again to continue. … Nothing was changed." 409 has its own sentence and moves focus to Rotate, because the server sends `{error}`, not `{reason,message}`.
- Two wiring fixes found by the live smoke: owner writes now send `X-CSRF-Token` (from `window.OfferRails.csrf` or `<meta name="csrf-token">`; the host page must provide it), and first-time setup forwards `?install_path=` to GET /v1/setup/settings. Without it a fresh owner's PUT got 400.
- Bent: the panel and the revoke modal can't stack (ui-kit allows one overlay at a time), so the modal replaces the panel and the panel comes back on close. Closing the panel mid shown-once drops the key; Rotate is the recovery.
- Bent: amber warning on the shown-once line reuses the caution token (7.61 on caution-soft), not the setup accent. It is a warning, not an action.
- Result (12 Oct 15:40): 116/116 at 1440 and 116/116 at 390 (98 v7 + 18 v8), axe 0 on every axe-checked screen; contract 12/12 vs 6042dcb; live smoke 26/26 vs :18125 (6042dcb), 0 go-link requests.

## v9 review (PR #21 @ 1b85f07, 2026-10-13)
- Done: CSRF only from `<meta name="csrf-token">`; no `install_path` sent (server default); plugin-key errors by `reason` (reauth_required / key_exists / forbidden_key_type) with server message, our text as fallback; listing "In review" screen for `submitted` + `review_reason` (claims = "checking the offer's claims against the merchant page"), server message verbatim, next step, no time promised; `undone_by` in the digest; `surface_url`/`undone_by` optional in the spec; mode preselected only from `mode_preselected`.
- New at 1b85f07 (Site Keeper): empty `post_title` -> UI falls back to the `surface_url` path, else "Untitled post" (mocked test `post_title_fallback`).
- Tests: mocked Playwright 1440 + 390, contract vs 1b85f07, axe 0, 0 requests to /v1/go (route-aborted, never clicked).
- Live (acceptance = :18127 on 1b85f07, CLAIMS_CHECK=stub): `tests/live_smoke_stub.py` 9/9 (register returns plugin_key, owner Origin + X-CSRF-Token POSTs, untrusted maker -> submitted / claims / "The claims check is not live yet.", trusted maker lists, in-review screen at 390 + 1440); `tests/live_smoke_2d3aed7.py` (SMOKE_PORT=18127) 34/34 plugin-key + setup. The owner digest at 1b85f07 still has no review fields, so the in-review screen is rendered from the maker's real offer read added to the real digest (stated in the script).
- YOLO pending (Product Lead, design offer-rails.md:147): `mode: manual` + `mode_preselected: yolo` = YOLO is picked and turns on after the first approved placement. Setup shows YOLO checked with "Picked. It turns on after your first approved placement." plus the server's approval_message when sent, else "Until then, you approve each new box yourself." Mocked tests `yolo_pending_server_reason` + `yolo_pending_fallback` at 390 and 1440; older setup tests updated to this rule.
- Live :18127 (`tests/live_smoke_yolo_1b85f07.py`, recorded only): PUT mode `yolo` -> 200, then GET returns `mode: manual` **and `mode_preselected: manual`** (approval_reason claim_check_unavailable present on this run). With preselected manual the UI has no way to know YOLO was picked, so the owner screen shows "Ask me every time" checked (shots live-18127/live-setup-after-yolo-save-*.png). Server flag: per the design `mode_preselected` should stay `yolo` (or the server should return the owner's pick) until the first approved placement. Site Keeper also saw the case with no approval_reason; the UI fallback covers that once preselected is yolo.
- Server-side, noted only: `/go` with an unknown id redirects to the API root and drops the port (Site Keeper).
- Still open: owner digest lacks `status/review_reason/message` on offer items; 3 open parity rows unchanged.

## v9 @ f249a77 (2026-10-14)
- Contract 13/13 vs f249a77 (offer items now require status/review_reason/message; 8 fixtures gained them). Mocked Playwright 136/136 at 1440 and 390 (new: `in_review_from_v1_digest`, `yolo_pending_f249a77`), axe 0, 0 /v1/go.
- UI changes: held offer taken from `/v1/digest` into its own offer section when the account digest leaves it out; `approval_reason: yolo_after_first_placement` shows "Picked." + the server sentence and no longer locks auto.
- Live :18128 (`tests/live_smoke_f249a77.py`): 14/18. Pass: plugin_key on register, CSRF meta, trusted maker lists + shows Listed, untrusted maker submitted/claims/"The claims check is not live yet.", `/v1/digest` review fields, in-review screen from real digests at 390 + 1440, yolo save -> manual/yolo/yolo_after_first_placement, YOLO picked with server reason at 390 + 1440, unknown `/v1/go/<id>` 404 no redirect (plain HTTP). Fail (server): post_title empty (x3: API + 2 widths show "Untitled post"); account digest omits held offers. `tests/live_smoke_yolo_1b85f07.py` (SMOKE_PORT=18128): YOLO checked + server sentence at both widths. `live_smoke_stub.py` not run on :18128 (it injects; this server no longer needs that).
- Known server bugs seen: maker-email sign-in does not link the maker, so a maker owner sees an empty digest until the dashboard bind (`POST /owner/accounts` role=maker + maker key), which the smoke uses. Rome-time digest test: no visible effect in these runs.
