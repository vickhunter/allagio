# D-032 UI ↔ D-032a backend parity (PR #21, vickhunter/affilio @ f249a77)

**v9 @ f249a77:** PR body saved as `PR21-body-f249a77.md` (head `f249a77c9b4d42985878c776079a09e8ea35c2ea`, via the GitHub connector) because it changes shapes the UI reads: every digest offer item has `status`, `review_reason`, `message`; a saved yolo answers `mode: manual`, `mode_preselected: yolo`, `approval_reason: yolo_after_first_placement` (+ message) until the first approved placement; `GET /v1/go/<unknown id>` is an HTML 404 (no redirect). Contract test 13/13 against this body; spec offer item keys now include the three review fields. Live acceptance: `tests/live_smoke_f249a77.py` against local f249a77 on :18128 (no injection: every screen shows what the server sends).

**v9 @ 1b85f07 (earlier):** PR body saved as `PR21-body-1b85f07.md` (head `1b85f07dc64cdc4d09505fe0ca375b9c5f761124`, via the GitHub connector). 1b85f07 adds local `CLAIMS_CHECK=real` (page claims vs the merchant page); hosted still defaults to `stub`. No API shape change for the UI: a real-check miss is still `review_reason` `claims` (or `head`) with a server message, which the UI shows word for word. Contract test now runs against this body (13/13). Live acceptance target: local 1b85f07 on :18127 (stub).

**v9 (first pass):** re-checked at head `2d3aed70cdbef2299b1c2d31cd5a85b714bf2614` (`PR21-body-2d3aed7.md`; commits 580edd1, 80824ab, 2d3aed7 after 6042dcb). Read through the GitHub connector, nothing cloned: `handlers/plugin_keys.go` (writePluginKeyError), `agent_entry.go` (digestJSON `undone_by`, setup default install path), `listing_review.go` and `offers.go` (review_reason/message), the head commit diff. Changes vs 6042dcb that touch the UI: owner pages render `<meta name="csrf-token">` (no window global); setup GET defaults `install_path`; plugin-key errors are `{error, reason, message}`; digest items carry `undone_by` (owner id or `""`); the poll adds `surface_url` + `placement.surface_url`; 409 table gains `one_per_post` (16 reasons); maker listing routes carry `review_reason` + `message`.

v8 basis: checked against the PR #21 body at head `6042dcb071a993889cc0643585978d5bb1cc35ec` (`PR21-body-6042dcb.md`) and the code at that ref: `handlers/agent_account.go`, `agent_entry.go`, `plugin_keys.go`, `plugin_key_issue_test.go`, `entry/policy.go`, read through the GitHub connector (copies in `../src-6042dcb-partial/`; nothing cloned). Diffed against the git-blob-verified `../src-f7bf688/`: the only API changes after f7bf688 are the plugin-key routes, `register` returning `plugin_key`, decide/restore refusing any credential header (fix B) and restore/retry checking claims in the same transaction (fix C). PR #21 is still a draft (not merged, not deployed). The fb493f0 table is in git history (v6/v7).

Mocked: real mode (`?api=real`) is tested with Playwright routes answering from `fixtures/api/*.json`; `tests/contract_test.py` checks them key by key against 2d3aed7 (13 checks).
Live: `tests/live_smoke_2d3aed7.py` runs the plugin-key + setup flows against the local 2d3aed7 API (:18125, database d033_2d3aed7). Results in REVIEW.md v9. (`tests/live_smoke_6042dcb.py` is the v8 run, kept.)

How to read **match**: `yes` = same name, shape and meaning. **waits on server** = the UI is built and tested against the shape; the server part is not there or not proven yet. **open** = the PR is missing it or it would break an agreed rule; the rule is kept.

**Summary (v9 @ f249a77): 91 rows. 87 match, 1 waits on server (empty `post_title` on digest items), 3 open (listing undo has no redo, maker earnings not called, one `site_url` per account).** At 1b85f07 it was 85 / 3 / 3: listing in review and the YOLO pick now match. v8 was 86 rows: 82 / 1 (`undone_by`) / 3. `undone_by` now matches; added rows: setup install-path default, poll `surface_url`, listing in review. The plugin-key rows were planned as "waits on server"; 6042dcb ships the routes, so they use the real names and match (no placeholder names left).

## Endpoints

| field | mine | theirs 6042dcb | match | note |
|---|---|---|---|---|
| Undo | `POST /v1/placements/:id/revoke` `{"reason":"undo"}` on tap, keepalive | same | yes | |
| Undo 200 body | item shows "Leaving your post shortly" | `{status:"revoked", revoked_by_side, undo, undone_at}`, no `plugin_state` | yes | No `plugin_state`, so pending is assumed (the PR says a revoke sets it to pending). The box is never shown removed until a poll says `removed`. |
| Redo | `POST /v1/placements/:id/restore` `{}` while the toast is open | same; 6 s server window | yes | |
| Redo 200 body | "Going back into your post" | `{status, approved_by, restored_by, actor}` | yes | Shown as `placed` only once the poll says so. |
| Redo 409 `window_expired` / `paused` | "Too late to redo. *message* It stays undone." / "Can't redo. *message* It stays undone." | `{reason, message}` from `RestoreRefusals` | yes | The server's message is always shown. |
| Redo 409 `not_undone` | "Can't redo. The placement was not undone. Showing the latest status." Then `GET /v1/placements/:id` to resync | `{reason:"not_undone", message:"The placement was not undone."}` | yes | Fixed in fb493f0 (was a bare `{error}`). `not_undone` is now an allowed reason. |
| Every 409 | `{reason, message}`, message never empty | 16 reasons in the 409 table at 2d3aed7 (adds `one_per_post` "Keep one box on this post."), each with a fixed message | yes | Contract test: every fixture 409 equals the table. |
| Redo 403 | "Only the person who undid this box can bring it back." | `{error}` 403 | yes | |
| Redo / Retry 422 | `check_text` shown as is | `{check_id, check_text}` from `HardCheckCatalog` (9) | yes | |
| Retry | `POST /v1/placements/:id/retry` `{}`, only when `failed`; repeats only `failed_action` | same | yes | |
| Retry of a failed restore → 422 | inline on the card: "Can't put it back. *check_text* It stays removed." Retry and Undo/Redo disappear, not counted live | 6042dcb: retry re-runs every hard check, incl. `claims_from_offer` in the same transaction (fix C) | yes | Under the production stub this is 422 `claims_from_offer` "Use only claims from the offer." (fixtures `retry-422-claims_from_offer`, `restore-422-claims_from_offer`). |
| Retry 200 body | `plugin_state` from the body | `{plugin_state:"pending", plugin_failed_reason:"", failed_action:""}` | yes | |
| Retry 409 `not_failed` | "This box already changed. Showing the latest status." + re-read | `{reason:"not_failed", message}` | yes | |
| Retry for makers | Retry hidden on maker rows: "Only the publisher can try again." | maker project key: read allowed, retry 403 | yes | |
| Redo/restore with a key | hidden in key contexts with "Putting a box back, Redo, approving and plugin keys need the dashboard" | 403 `{error:"decide and restore require a signed-in dashboard session"}` for any credential header, incl. legacy site and maker keys (fix B) | yes | Was "expected after must-fix (c)". |
| Plugin progress | `GET /v1/placements/:id` every 4 s for 60 s, then 30 s, stop at 10 min, visible tab only | `{id, status, plugin_state, plugin_failed_reason, failed_action, policy_rule_id, policy_rule_text, fallback_url}` | yes | |
| Poll rate limit | 4 s / 30 s schedule kept | one call every 2 s per placement | yes | Fixed in fb493f0 (was 4 s, equal to my cadence). |
| Poll who | publisher and maker rows | site key, owner session, or a maker project key for its own offer | yes | Fixed in fb493f0. |
| First-load status reads | none | `plugin_state` + `live_now` come with each digest item | yes | v6 dropped the per-item `GET` on load. Only pending/offline items poll, on the normal schedule. |
| Pause everything | `POST /v1/account/pause` `{"scope":"all"}` | same → `{scope, id, applied:[{kind, id, mode, kill_switch, paused}]}` | yes | |
| Resume | `POST /v1/account/resume` `{"scope":"all"}`, session only | same; 403 with any key | yes | |
| Mode after pause/resume | `applied[].effective_mode`; the outcome line quotes the row's `approval_message` | each `applied[]` row has `mode` (saved), `effective_mode`, `approval_reason`, `approval_message` (`withEffectiveMode`) | yes | Must-fix 15. Tests `real_resume_effective_mode`, `real_pause_resume`. |
| Digest data | `GET /v1/account/digest` | `{totals, projects[], earnings_note, session, paused, went_live_since_last, change_url, site_url, available_modes, effective_mode, approval_reason?, approval_message?}` | yes | |
| Digest "since" | `GET /v1/digest` (`since` only) | `{since, items, went_live, session, …}` | yes | |
| Digest opened | `POST /v1/events` `{"kind":"digest_opened"}` after "since" was read | same → 201 | yes | |
| Paused state on load | `paused.account` → banner; `paused.site` / `paused.offer` → "Paused" pill on those sections | `paused:{account, site, offer}` in the digest | yes | v6 dropped the per-project `GET /v1/{sites,offers}/:id/mode` reads. |
| Digest page size | "Showing the latest 50 of N. See them all in the dashboard" when `total_items` > items shown (link = `change_url`) | `projects[].total_items` (same WHERE as the items), items `LIMIT 50` | yes | Paging inside the digest is still not offered (no cursor); the dashboard link is the way to the rest. |
| Undo a listing | listing rows show "Listed · date" with no Undo button | `POST /v1/digest/undo`, **no redo in v1** | no | **Open item.** Offering it would break "every Undo has a Redo". Listed this way on purpose. |
| Hard checks | `GET /v1/policy/hard-checks`, rendered in the order returned; error line if it fails | `{checks:[{check_id, check_text}]}` (9) | yes | No hardcoded list of 7 or 9 (the contract test checks the JS). |
| Setup load | `GET /v1/setup/settings` (`?role=maker` for a maker) | same | yes | |
| Setup save | one `PUT /v1/setup/settings` per role, exactly 13 keys | strict keys `role, install_path, mode, mode_shown, promote, avoid_topics, project_name, listing_url, commission, listing_price, sites_channels, never_topics, box_position` | yes | |
| Saving keeps the mode | a saved setup shows its mode read-only ("Saving here keeps your saved approval mode"), PUT still sends the 13 keys | `PutSetup` keeps `owner_setup.mode` when a row exists (must-fix 14) | yes | A first-time setup still picks a mode. |
| Maker earnings | not called | `GET /v1/maker/earnings` `{offers[].commission, offers[].currency, totals[] {commission, currency, mode}}` | no | **Open item.** The digest's offer `earnings[]` is enough for D-032. The names are recorded in INTERFACE.md for the dashboard. |
| Go link | never called or clicked (tests abort `**/v1/go/**`, 0 requests) | `GET /v1/go/:placement_id` | yes | `api.js` refuses `/v1/go/` paths. |

## Digest fields (`GET /v1/account/digest`)

| field | mine | theirs 6042dcb | match | note |
|---|---|---|---|---|
| Live items (account) | `totals.went_live` | same (`placement_counts_as_live`) | yes | |
| Live now (section) | `projects[].went_live`; re-read after every confirmed change | same | yes | |
| Live now (item) | counts only `live_now=true`; after a local change, my rule (confirmed `placed` + not paused) | `live_now` = `placement_counts_as_live` | yes | Same rule, computed by the server. |
| Went live since last digest | "N items went live since last digest" under Live items | `went_live_since_last` | yes | Shown again (it was hidden in v5). |
| Earnings (publisher, total) | one line per currency; `null` or `[]` → "No earnings data yet" | `totals.earnings[] {currency, amount}` or `null` | yes | Currencies are never added together. |
| Earnings (site) | same | `projects[].earnings[]` | yes | A zero amount shows as €0.00, not "No data". |
| Maker amount (offer section) | "Commission to pay" + "Not income", per currency | `projects[].earnings[] {currency, amount}` on offer projects | yes | The `maker_commission` placeholder tile is gone. |
| Clicks / top sources | `clicks`, `clicks_by_source` | same | yes | |
| Approval mode label | `projects[].mode` (effective) | same; `effective_mode` at the top | yes | |
| Why auto is off | `approval_message` as a note at the top and under each "Ask me every time" label | `approval_reason:"claim_check_unavailable"`, `approval_message` | yes | |
| Change link | `change_url` (http(s) or `/` only) | `change_url` | yes | Was `#preview` in v5. |
| Site address | `site_url` caption when there is exactly one site | `site_url` (one per account) | no | **Open item.** With several sites a single URL can't be placed, so none is shown. Ask: `projects[].url`. |
| Item: post | `post_title` in quotes | `post_title` | yes | |
| Item: position | plain label per value (8 values) | `position` | yes | |
| Item: link text / label / clicks | box preview + "N clicks" | `link_text`, `label`, `clicks` | yes | |
| Item: who approved | "Approved by you" / "Approved by your rule: *policy_rule_text*" + "rules vN"; other `approved_by` as plain text | `approved_by`, `policy_rule_id`, `policy_rule_text`, `rules_version` | yes | The rule id is never shown. |
| Item: post title | `post_title`; if empty, the path of `surface_url`; if neither, "Untitled post"; never empty quotes | f249a77 says `post_title` falls back to the article title; live :18128 placement (prepare with `surface_url`, approve, commit, ack placed) still has `post_title` `""` and no `surface_url` on the digest item | waits on server | UI shows "Untitled post" live. Ask: fill `post_title` (or send `surface_url` on digest items). |
| Item: who undid | "Undone by you / your agent / …" for word values; an owner id reads "Undone by an owner on this account" (row + details panel); `""` shows nothing | 2d3aed7 digestJSON `undone_by`: the owner id when undone, `""` otherwise; optional in the spec | yes | The digest has no viewer owner id, so the UI cannot say "you" for an id; it never shows the raw id. Ask: send `undone_by_self: bool` or `"owner"` for the viewer. |
| Item: plugin state | `plugin_state` drives the state line; null/unknown → "Status unknown" | `plugin_state` (never null), `plugin_failed_reason`, `failed_action` | yes | |
| Item: pending/offline on load | "Waiting for your post to update" / "Site offline · waiting for it to reconnect", no toggle, until the poll gives the direction | no `status` on digest items | yes | No placed/removed claim before the server says so. |
| Item: expiry | shown only for `status:"prepared"` with `expires_at`; never for committed/placed | a committed placement never expires (must-fix 1); digest items carry no `expires_at` | yes | |
| Paused (account / site / offer) | banner / section pill | `paused.{account,site,offer}` | yes | |
| Available modes | `available_modes` | same | yes | |
| Session | `session.signed_in` gates Resume and (c) | same | yes | |
| Key/agent context | without a session: Undo, Pause, Stop all stay; Redo, Retry of a failed restore, Resume and plugin-key actions hidden/disabled, with the line "Putting a box back, Redo, approving and plugin keys need the dashboard" | decide/restore 403 for every key (fix B); plugin-key routes session-only | yes | |

## Plugin key (Product Lead must-fix A; routes from `handlers/plugin_keys.go` @ 6042dcb, all session-only)

| field | mine | theirs 6042dcb | match | note |
|---|---|---|---|---|
| Shown once on connect | setup, after save, publisher: `GET /v1/account/projects` → site id, `GET …/plugin-key`; no active key → `POST /v1/sites/:id/plugin-key` → shown-once step | `register` returns `plugin_key` once; WP connect page prints it once; `POST /v1/sites/:id/plugin-key` 201 `{plugin_key, key_prefix}` | yes | Active key already (the usual case after register) → no step, "Your plugin already has its key (starts …)". |
| Shown-once UI | monospace read-only field, Copy (→ "Copied ✓"), "We won't show this key again", where to paste (Offer Rails › Settings › Plugin key), keycap Continue only after copy or "I saved it" | n/a | yes | Raw key never stored, emitted, logged or put in a URL; cleared from the field on Continue. |
| Generate (existing sites) | Settings panel → Generate key (only when no active key) | `POST /v1/sites/:id/plugin-key` 201 / 409 `{error, reason:"key_exists", message:"A plugin key already exists."}` | yes | v9: chosen by `reason`; shows the server `message` then "To get a new one, rotate it…", focus on Rotate. A body with only `error` falls back to the v8 sentence. |
| Prefix list | "Active key starting `abcd1234` · created …"; revoked key named with its date | `GET /v1/sites/:id/plugin-key` → `{keys:[{id, key_prefix, created_at, revoked, revoked_at?}]}` | yes | Never the secret. |
| Rotate | inline confirm → new key with the same shown-once UI | `POST /v1/sites/:id/plugin-key/rotate` → 200 `{plugin_key, key_prefix}` | yes | |
| Revoke | danger confirm modal: what stops working (plugin can't confirm placements; new boxes won't count as live until a new key is pasted) and what doesn't | `POST /v1/sites/:id/plugin-key/revoke` → 200 `{revoked}` | yes | |
| Fresh sign-in / refusals | by `reason`: `reauth_required` → "Sign in again to continue." + server message + "Sign in again"; `forbidden_key_type` → "This needs the dashboard, not a key." + server message + "Open the dashboard"; `forbidden` → sign in again + message; always "Nothing was changed." | 2d3aed7 writePluginKeyError: 401 `reauth_required` (no session, or older than 15 min), 403 `forbidden_key_type` (site/maker/project/account key, bearer), 403 `forbidden`, each with `error` kept | yes | No reason (older body) → v8 text by status. |
| CSRF on owner writes | `X-CSRF-Token` read only from `<meta name="csrf-token">` on every non-GET; never on reads; `window.OfferRails.csrf` ignored | 2d3aed7: owner pages render the meta, no window global; POSTs need `Origin` + `X-CSRF-Token` | yes | v9 removed the window fallback (test `csrf_window_global_ignored`). |
| Key contexts | panel shows "Generating, rotating and revoking the plugin key need the dashboard"; no buttons, no calls | session-only | yes | |

| Poll `surface_url` | not used (optional in the spec) | 2d3aed7: `surface_url` + `placement.surface_url` on the poll, list and decide | yes | Accepted, not shown: the digest already names the post. |
| Listing in review | offer item with `status: submitted` + `review_reason` (claims/head/relevance/caps) + `message` → "In review", check in plain words, "Server says: …", next step; no time promised; optional. f249a77: an offer section with no item for its own id takes the held item from `GET /v1/digest` (same id, once) | f249a77: every digest offer item has the three fields. Live :18128: `GET /v1/digest` has them; `GET /v1/account/digest` leaves a held offer out of its project's `items` (`total_items` 0) | yes | In-review screen shown live from the real digests at 390 + 1440. Ask: list held offers in the account digest project items too. `/v1/digest` items are "since the last open", so after one open the held item only stays via the account digest. |

## Setup fields (`GET`/`PUT /v1/setup/settings`)

| field | mine | theirs 6042dcb | match | note |
|---|---|---|---|---|
| Install path | `install_path` (6 values) → "Connected via …" | same | yes | |
| Setup: YOLO picked, not on yet | `mode: manual` + `mode_preselected: yolo` -> YOLO checked + "Picked." + server `approval_message` (for `yolo_after_first_placement` the server sentence alone; for other reasons "It turns on after your first approved placement." + message); no reason -> our fallback. `yolo_after_first_placement` locks no other mode | f249a77: exactly that shape; live :18128 after a yolo save: manual / yolo / `yolo_after_first_placement` / "YOLO is picked and turns on after the first approved placement." | yes | Live shots live-18128/live-setup-yolo-picked-*. |
| Install path default | setup GET sends only `role`; never `?install_path=`; PUT sends back the server's `install_path` | 2d3aed7: GET without `install_path` returns the path for `owners.source`; PUT with empty stores the same default | yes | v9 dropped the v8 forwarding. |
| Role | `role` | same | yes | |
| Effective mode | `mode` → checked if it can be picked | same | yes | |
| Suggested mode | `mode_preselected` (Suggested tag only on an option that can be picked); checked = saved/effective `mode` or `mode_preselected`; none → nothing checked | same, from `owners.source` (cli/cursor/grok/mcp still yolo; effective manual under the stub) | yes | Never guessed from the install path. v9 test `mode_preselected_from_server` (cursor + no mode_preselected → nothing checked). |
| Mode shown | `mode_shown` sent back = the mode checked when the page opened | same | yes | |
| Modes on offer | `available_modes` | same | yes | |
| Auto/yolo disabled | when `approval_reason` is set; reason under each = `approval_message` | `approval_reason`, `approval_message` | yes | |
| Maker auto-accept under the stub | maker auto/yolo disabled whenever `approval_reason` is present | stub refuses publisher auto-accept server-side (must-fix 9); setup sends the reason when the saved mode is auto/yolo | yes | |
| Project name | `project_name` | same | yes | |
| Pitch | `promote` | same | yes | No more "Name: pitch" split. |
| Listing URL | `listing_url` | same | yes | |
| Commission | `{type: percent\|fixed, value, currency}`; currency required for fixed | same | yes | |
| Recurring commission | not offered any more | no type for it | yes | Dropped to match. |
| Listing price | number + currency, optional (`null` when empty) | `listing_price {value, currency}` or `null` | yes | |
| Topics to avoid / never promote | `avoid_topics`, `never_topics` | same | yes | |
| Publisher types to avoid | hidden in real mode | no field | yes | Kept in fixture mode only. |
| Sites / box position | `sites_channels`, `box_position` | same | yes | |
| Saving with a key | Continue disabled + "needs the dashboard" | PUT is owner session only (403 with a key) | yes | |
| Change / site links | `change_url`, `site_url` read | same | yes | |

## Rules kept (not changed to match)

- A box is shown placed or removed only after the plugin confirms it. Pending/offline digest items get neutral copy until the poll answers.
- `failed_action` is `removed`|`restored`; Retry repeats only that action; "Leave it removed" only when `failed_action=restored`.
- An item counts toward Live now only when confirmed in the post AND its go link redirects. fb493f0's `live_now` is the same rule.
- Every 409 is `{reason, message}` with a message that is never empty. fb493f0 now does this.
