# D-032 UI interface (setup page + digest)

Static HTML/CSS + vanilla JS. **Data in** = JSON (fixtures now, server-rendered later). **Data out** = DOM `CustomEvent`s on `document` (they bubble; `event.detail` is plain JSON).
Field names follow `design/offer-rails.md` §D-031/§D-032 where the spec names them. Anything marked **(proposed)** is my suggestion for AI Engineer to accept or rename.

Internal mode values `manual | auto | yolo` and `approved_by` values `owner | policy:auto | policy:yolo` travel in data and events only. They never appear on screen (tests check this).

## Data in

| Page | How the Go template feeds it | Fixture fallback |
|---|---|---|
| `setup.html` | JSON inside `<script type="application/json" id="or-setup-data">` | `fixtures/setup-<name>.json` via `?fixture=maker\|publisher\|empty\|directory` |
| `digest.html` | JSON inside `<script type="application/json" id="or-digest-data">` | `fixtures/digest-<name>.json` via `?fixture=normal\|empty\|many\|no-earnings\|paused-signed-out` |
| `<digest-item>` | `el.item = {...}` or `data-item='{json}'` attribute | used by digest.html |

Preview switches (fixtures only): `?fail=save` (setup save fails), `?toast_ms=` (Redo toast, default 6000), `?from=email` (passed into digest-opened), `?mock=1` (in-page API mock answering from `fixtures/api/*.json`; pick outcomes with `&restore=200|409-window|409-paused|409-noreason|403|422-cap|422-relevance_min|422-offer_live|422-disclosure|422-unknown`, `&plugin=pending|placed|removed|offline|failed`, `&resume=200|403`). Tests mock the API with Playwright routes instead.
Saving the setup: `window.OfferRails.saveSetup(payload) -> Promise`. The preview version resolves after 250 ms; the host replaces it with the real call. Reject = error state, answers kept.

### Setup JSON (see `fixtures/setup-maker.json`)
`setup_id`(proposed), `session{user_id, signed_in, human_login, device_code}`, `agent{source, client_name(proposed), install_path(proposed)}`, `detected_roles[]`(proposed), `project{project_id, name, url}`, `mode_preselected` (v2: **set by the server**, including the default-by-install-path rule; the UI no longer derives it. `null` or missing = nothing preselected and Continue asks the person to pick), `maker{listing{name,pitch,price,url}, readiness{score, signals[{id,label,pass,reason}]}, avoid_topics[], avoid_publisher_types[], commission{type,value}}`, `publisher{sites[{site_id,domain,kind,found_by,selected}], readiness{...}, never_topics[], box_position}`.
`source` values: `cli | mcp | cursor | claude | chatgpt | grok` (spec source list + cli/mcp). `box_position`: `start | section | end` (proposed). `commission.type`: `percent_first_payment | percent_recurring_12m | fixed_per_sale` (proposed).

### Digest JSON (see `fixtures/digest-normal.json`, generator `fixtures/make_digest.py`)
`digest_id`, `user_id`, `period{since,until}`, `session{signed_in,user_id}`, `test_mode`, `paused`,
`totals{live_items, clicks, earnings, projects, sites, went_live}`,
`projects[]` and `sites[]` (the UI sets `item.side` = `maker` for project items, `publisher` for site items, proposed), each: `project_id|site_id, name, url, role, mode, mode_before_pause(only when paused), mode_change_url, went_live_count, live_count, clicks, earnings, top_sources[{source,clicks}], items[]`.
`earnings` = `{amount_minor, currency, test_mode}`, or (v5) a list `[{currency, amount_minor}]` with one line per currency, **or `null` = no earnings data yet** (shown as "No earnings data yet"). A real zero with data is `{amount_minor: 0}` and shows **€0.00** (decided).
Item: `item_id, kind(placement|listing), placement_id, owner_ref, site{name,domain}, article{title,url}, position, copy(<=140), link_text, disclosure_label, approved_by, policy_rule_id, policy_rule_text, policy_version, went_live_at, clicks, status(live|undone), plugin_state, preview_href`.
`policy_rule_text` is shown as is (plain words from the server); `policy_rule_id` is machine-only (exposed as `data-policy-rule-id`, tests assert on it). Both decided.
`plugin_state` (decided): **`pending | placed | removed | offline | failed`** (`failed` approved by Product Lead 03:35: the plugin acked a failed re-read check). `null` or any other value, or an impossible pair (`placed` on a removed item, `removed` on a live item), shows **"Status unknown"**; the UI never assumes a value. Live items carry `placed`.
With `failed` the server also sends **`failed_action: "removed" | "restored"`** (which action failed) and **`plugin_failed_reason`** (string or null, shown word for word in small text under the main sentence; nothing when null). If `failed_action` is missing the UI infers it from status (undone -> removed, live -> restored).
All names except `approved_by`, `policy_version`, `policy_rule_id`, `policy_rule_text`, `plugin_state`, `copy`, `mode`, `earnings` are proposed.

## Server calls (v2, decided in the Offer Rails room)

| Action | Call | Body | Responses handled |
|---|---|---|---|
| Undo | `POST /v1/placements/:id/revoke` **immediately on tap** (no client hold; `fetch keepalive` so it survives closing the tab) | `{"reason":"undo"}` | 2xx: clicks are off at once, the item shows "Leaving your post shortly" (pending; never "removed" until the server reports `plugin_state: removed`), `plugin_state` from body (missing = Status unknown), "Live now" and account "Live items" drop by one at once, Redo window opens (6 s). Toast: "Undone. Clicks are off; the box is leaving <site>." Anything else: "Couldn't undo right now. The box is still live. Try again." |
| Redo | `POST /v1/placements/:id/restore`, only while the toast is open | `{}` | 200: "Going back into your post" while pending; "Live · <date>" and "Live now"/"Live items" +1 **only when the server reports `plugin_state: placed`** (v4.3). A refused restore changes no count. 409 with `reason` (decided enum `window_expired | paused | not_failed`; `not_failed` only applies to retry): `window_expired` -> "Too late to redo. It stays undone."; `paused` -> "Can't redo: you paused after undoing. It stays undone."; unknown reason -> server `message` word for word (409 bodies are always `{reason, message}`, message never empty) ("Can't redo. <message> It stays undone."), else "Can't redo this one. It stays undone." 403 -> "Only the person who undid this box can bring it back. It stays undone." 422 -> "Can't redo. <check_text> It stays undone." **`check_text` from the server is always shown as is.** `check_id` (decided): `disclosure, rel_sponsored, claims_from_offer, copy_length, one_per_post, relevance_min, offer_live, revoke_path, cap`; a short fallback sentence per id is used only when `check_text` is missing, and an unknown id without text says "Can't redo: it no longer passes the checks. It stays undone." Other -> "Couldn't redo right now. It stays undone." Raw JSON is never shown. |
| Retry (failed only) | `POST /v1/placements/:id/retry` from the button inside the amber note: "Try removing again" (`failed_action: removed`) or "Try putting it back again" (`restored`). One request at a time: busy label "Trying again…", `disabled` + `aria-busy`, repeat taps ignored. 44 px tall, full width at 390. | `{}` (server repeats only the failed action) | 2xx: `plugin_state` from body (expected `pending`): removal shows "Leaving your post shortly", restore "Going back into your post"; polling restarts from 0 s. **status never changes on retry, so "Live now" never moves, and a retried removal can never show placed/Live** (shows "Status unknown" if the server ever says placed). 409 `reason: not_failed` -> "This box already changed. Showing the latest status." then `GET /v1/placements/:id` and show it. 422 -> "Couldn't try again. <check_text>" word for word, item stays failed, Retry offered again. Other 409 (`window_expired`, `paused`, unknown) / 403 / other: server `message` word for word after "Couldn't try again." (422 uses `check_text`), else a short generic sentence. |
| Plugin progress | `GET /v1/placements/:id`, only while `plugin_state` is `pending` or `offline` **and `document.visibilityState === 'visible'`**. Schedule: **every 4 s for the first 60 s, then every 30 s, stop at 10 min** (then a still-pending item says "Still waiting for your site."). A tick that lands while the tab is hidden is skipped; polling resumes immediately when the tab becomes visible again. Stops on `placed`, `removed`, `failed`, null/unknown; restarts after a successful Retry. | none | `plugin_state` updates the item. Hosts can also push it with `el.setPluginState(state)`. |
| Pause everything | `POST /v1/account/pause` | `{"scope":"all"}` (API also takes `"site"`/`"offer"` + `id`; the digest only uses `all` for now) | 2xx: paused, all modes shown as "Ask me every time". Else "Couldn't pause right now. Nothing changed. Try again." |
| Resume | `POST /v1/account/resume` (session only) | `{"scope":"all"}` | 2xx: previous modes back. 403: "Resume needs you signed in to the dashboard in this browser. Sign in again, then press Resume. Everything stays paused until then." Button disabled when `session.signed_in` is false. |

Item copy by state (`side` publisher says "your post", maker says "their post"):

| status | plugin_state | State line | Note |
|---|---|---|---|
| live | placed | Live · <date> | |
| live (after restore) | pending | Going back into your post | |
| undone | pending | Leaving your post shortly (after Undo / retry) · Taking it out of your post (after "Leave it removed") | after 10 min: "Still waiting for your site." |
| undone | removed | Box removed | |
| either | offline | Clicks off · site offline / Going back · site offline | removed: "Your site is offline. Clicks already stop at the link; the box will come out when it reconnects." live: "Your site is offline. The box will come back when it reconnects." |
| undone | failed, failed_action removed | Clicks off · box still in your post | "Your site couldn't update the post. Check the notice in your WordPress admin. Clicks already stop at the link." + reason + [Try removing again] (amber). **Only Retry**: no "Leave it removed", no Undo/Redo (v4.3). |
| live | failed, failed_action restored | Not back in your post yet | "Your site couldn't update the post. Check the notice in your WordPress admin. Clicks stay off until it's back in your post." + reason + [Try putting it back again] (amber). The item button reads **"Leave it removed"** (shown only for `failed_action: restored`): revoke `{reason:"undo"}`, no Redo toast, item shows "Taking it out of your post" (pending, clicks already off) until the server reports `removed` -> "Box removed"; never answered -> offline after 10 min (server) -> "Clicks off · site offline"; plugin fails -> `failed, failed_action: removed` -> only [Try removing again]. **Not counted in Live now / totals / earnings.** |
| either | (retry running) | Trying again… | button busy |
| either | null / unknown | Status unknown | |

Example bodies: `fixtures/api/*.json`.

### No optimistic state (v4.3, decided 03:50)
The UI never shows removed or placed before the server says so, after undo, redo/restore, retry (either way), "Leave it removed", pause or resume. Until confirmation it shows the pending copy above ("Leaving your post shortly", "Taking it out of your post", "Going back into your post"; pause/resume confirm buttons read "Pausing…" / "Resuming…" with `aria-busy` and modes/banner change only on a 2xx). "Live now" may drop at once on undo / leave it removed (clicks are off at the link), and rises only on a confirmed `placed`. Test `no_optimistic_audit` records every state/note/toast/mode text change and fails if any says "removed" or "Live" before the mocked server confirms.

## Events out

| Event | When | `detail` | session_only |
|---|---|---|---|
| `offerrails:setup-submit` | Continue pressed (fires before save) | `setup_id, user_id, source, install_path, roles[], roles_detected[], answers{maker?, publisher?}, mode_shown (= server's mode_preselected, may be null), mode_chosen, edited_fields[], actor:"owner_ui", session_only:true, submitted_at` | **true** |
| `offerrails:digest-opened` | digest rendered | `digest_id, user_id, opened_at, opened_from(proposed: web\|email\|agent), signed_in` | false |
| `offerrails:undo` | **v2: fires as the revoke request is sent, on tap, no hold** | `action:"undo", item_id, placement_id, kind, side, owner_ref, approved_by, policy_rule_id, policy_version, request{method,path,body:{reason:"undo"}}, actor:"owner_ui", session_only:false, at` | false (tightening) |
| `offerrails:restore` | **v2, replaces `offerrails:redo`**: Redo tapped, restore request sent | same shape, `action:"restore"`, `request.path=/v1/placements/:id/restore` | false |
| `offerrails:retry` | Retry pressed, request sent (once) | same shape as undo, `action:"retry"`, `failed_action`, `request.path=/v1/placements/:id/retry` | false |
| `offerrails:live-count` (proposed) | an item starts or stops counting as live | `item_id, placement_id, delta (+1\|-1)`. Counts = `status live` **and `plugin_state: placed`** (v4.3: rises only on confirmed placed; drops at once on undo / leave it removed because clicks stop at once). The digest applies the delta to the section "Live now" and the account "Live items". | n/a |
| `offerrails:placement-result` (proposed) | server answered an undo, restore or retry | `action (undo\|restore\|retry), failed_action (retry only), window_open (undo only), item_id, placement_id, http_status, outcome (revoked\|restored\|retried\|not_failed\|window_expired\|paused\|conflict\|forbidden\|check_failed\|error), reason, message (the plain copy shown), plugin_state` | n/a |
| `offerrails:plugin-state` (proposed) | plugin_state changed after polling or `setPluginState` | `item_id, placement_id, status, plugin_state` | n/a |
| `offerrails:pause-all` | kill switch confirmed, request sent | `digest_id, user_id, scope:"all", site_ids[], project_ids[], modes_before{ref:mode}, mode_after:"manual", pause_unpublished_auto_approvals:true, request{...}, actor:"owner_ui", session_only:false, at` | false (tightening) |
| `offerrails:resume-all` | Resume confirmed, request sent | `digest_id, user_id, scope:"all", restore_modes{ref:mode}, request{...}, actor:"owner_ui", session_only:true, at` | **true** |
| `offerrails:account-result` (proposed) | server answered pause/resume | `action (pause\|resume), scope, http_status, outcome (paused\|resumed\|forbidden\|error)` | n/a |

The UI changes state only after a 2xx (no optimistic pause/resume/restore). Undo is immediate on the server; the UI shows "Removing…" for the round trip.

`session_only: true` = the backend must accept it only from a signed-in dashboard browser session, never from an API or agent key. The four session-only actions (Product Lead 02:49): loosening the approval mode, raising caps or lowering minimum relevance, turning the kill switch back off (Resume), creating an account-wide key. On these screens: the mode question on setup and Resume on the digest carry a "Signed-in only" mark; per-section "Change" links say "Looser: signed-in only" (the change page itself is out of scope here). Caps/relevance and account-wide keys are not on these two screens.

## Every number and action -> proposed API / MCP tool
For AI Engineer to check against the D-032 mapping table. All names proposed unless they already exist (marked existing).

### Setup page
| UI element | Proposed HTTP (dashboard) | Proposed MCP tool (agent) |
|---|---|---|
| Agent/source name, project name, device code | `GET /v1/setup/:setup_id` (after device-code approve) | written by `start_device_login` / `scan_project` |
| Detected role(s) | same payload `detected_roles` | `scan_project` |
| Listing card (name, pitch, price, url) | same payload `maker.listing` | `scan_project` -> `upsert_listing` |
| Readiness score + each signal pass/fail + reason | same payload `maker.readiness` | `scan_project` (local) -> `submit_readiness` (card + signals + score only) |
| Publisher sites/channels found | `publisher.sites` | `scan_project` / `list_sites` |
| Topics/publisher types to avoid, never-topics | `answers.*` in submit | `get_settings` (agents read them) |
| Commission | `answers.maker.commission` | `get_settings`; becomes offer via existing `create_offer` / `update_offer` |
| Box position | `answers.publisher.box_position` | `get_settings` |
| Approval mode shown (server `mode_preselected`) + chosen | `POST /v1/setup/:setup_id` (session only) | `get_settings`; agent may only tighten via `set_approval_mode` |
| Hard checks list | static (fixed by D-031) | `get_hard_checks` (read only, optional) |
| Continue | `POST /v1/setup/:setup_id` body = setup-submit detail | n/a (human only) |
| Back to my agent | none (closes tab); agent polls `get_setup_status` | `get_setup_status` |

### Digest
| UI element | Proposed HTTP | Proposed MCP tool |
|---|---|---|
| Page data, period, test-mode flag | `GET /v1/digest` (latest) or `/v1/digest/:digest_id` | `get_digest` |
| Totals: live items, went live, clicks, earnings, projects, sites | `GET /v1/digest` `.totals` | `get_digest`; earnings also existing `get_earnings` |
| Per project/site: went live, live now, clicks, earnings | `.projects[]`, `.sites[]` | `get_digest` |
| Top click sources | `.top_sources` (from go-link click log `source`/referrer) | `get_digest` |
| Approval mode per section | `.mode` | `get_settings` |
| Change (mode) link | `GET /owner/settings/approval?site=|project=` (page; looser = session only) | `set_approval_mode` (tighten only) |
| Item: where, copy, label, approved_by + rule + version, clicks | `.items[]` | `list_placements` / `get_digest` |
| Undo | `POST /v1/placements/:id/revoke` `{reason:"undo"}` (decided) | `revoke_placement` |
| Redo | `POST /v1/placements/:id/restore` (decided; 6 s server window) | `restore_placement` |
| Plugin progress (pending/placed/removed/offline/failed + failed_action, plugin_failed_reason) | `GET /v1/placements/:id` | `get_placement` |
| Retry a failed plugin update | `POST /v1/placements/:id/retry` (decided; session or key) | `retry_placement` |
| Pause everything | `POST /v1/account/pause` `{scope:"all"}` (decided) | `pause` (allowed: tightening) |
| Resume | `POST /v1/account/resume` `{scope:"all"}` (decided, session only) | none: never from an agent key |
| Digest opened | `POST /v1/events` `{type:"digest_opened"}` or logged on `GET /v1/digest` server-side | n/a |

## Open questions for AI Engineer
Resolved: policy_rule_id/text, undo/restore/retry endpoints, 409 `{reason, message}` with enum window_expired|paused|not_failed, 422 `{check_id, check_text}`, plugin_state enum incl. failed, failed_action removed|restored, plugin_failed_reason, polling schedule, earnings null vs 0, mode_preselected, pause/resume, failed restore is not live (03:43).
1. Maker-side undo/retry: same endpoints for a maker's view of a publisher placement?
2. Session flag name: `session.signed_in` (+ `human_login` on setup).
3. Does the digest payload's `live_count` / `totals.live_items` count only `status live + plugin_state placed` (so excludes failed restores and restores still pending)? The UI assumes yes (fixture `digest-failed.json`).

## Wiring to the D-032a backend (v6, against PR #21 @ fb493f0)
Wired against the PR body at `fb493f0` (`PR21-body-fb493f0.md`) and the handler code at that ref. Product Lead decided that the PR's field names win. Every field and endpoint, with what matches and what doesn't: **`PARITY.md`**. PR #21 is a draft (not merged, not deployed); nothing has been run against a real backend.

### Modes
| URL | Data from | Use |
|---|---|---|
| `?fixture=<name>` (default) | `fixtures/*.json` (view model above) | design review |
| `?mock=1` | in-page mock answering from `fixtures/api/*.json` | clicking around by hand |
| `?api=real` | the real API through `assets/api.js` | the host page / a local backend |

Base URL for `?api=real`, first found wins: `window.OfferRails.apiBase`, `<meta name="or-api-base">`, `?api_base=`. The query value is accepted only as a same-origin path or `http(s)://localhost` / `127.0.0.1[:port]`. `api.js` never throws (resolves `{status, body}`) and refuses any `/v1/go/` path.

### Calls in real mode (exact shapes in `fixtures/api/`, checked against fb493f0 by `tests/contract_test.py`)
- Digest: `GET /v1/account/digest` + `GET /v1/digest` (only `since`), then `POST /v1/events {"kind":"digest_opened"}`. No mode reads (`paused.{account,site,offer}` comes with the digest) and no per-item status reads on load (`plugin_state`, `live_now` come with each item).
- Item view (`assets/contract.js`):
  - `live_now` or `plugin_state:placed` → live;
  - `removed` → undone;
  - `failed` + `failed_action` (`restored` → live, `removed` → undone);
  - `pending`/`offline` → "awaiting": neutral copy, no toggle, until the normal 4 s poll returns `status` (`revoked` → undone, `approved`/`committed` → live);
  - null/unknown `plugin_state` → "Status unknown".
  - Shown: `post_title`, `position` (8 values), `link_text`, `label`, `clicks`, `approved_by` / `policy_rule_text` / `rules_version`.
- Live now: an item counts while `live_now=true`. Once it changes on the page, the agreed rule applies (confirmed `placed` and not paused). Section and account numbers are the server's `went_live`, re-read after every confirmed change.
- Money: `earnings[] {currency, amount}` or `null`.
  - One line per currency, never added together.
  - `null` or `[]` → "No earnings data yet".
  - Site sections and the account total are income. Offer sections are "Commission to pay · Not income".
  - Maker endpoint for later: `GET /v1/maker/earnings` → `offers[].commission`, `offers[].currency`, `totals[] {commission, currency, mode}`. Not called by D-032.
- Modes: `mode`/`effective_mode` (effective), `mode_shown`, `mode_preselected`, `available_modes`.
  - While the claim check is a stub: `mode=manual` + `approval_reason=claim_check_unavailable` + `approval_message`.
  - The digest shows that message at the top and under each manual label. Setup disables auto/yolo with the message as the reason.
- Envelope: `went_live_since_last` (line under Live items), `change_url` (the Change link), `site_url` (shown only when there is exactly one site), `available_modes`.
- Redo 409s show the server `message` always. `not_undone` → "Can't redo. The placement was not undone. Showing the latest status.", then `GET /v1/placements/:id` to resync.
- Polling: 4 s for 60 s, then 30 s, stop at 10 min (the server allows one call every 2 s per placement).
- Retry is hidden for maker rows (a maker key may read status but gets 403 on retry).
- Undo of a listing (`POST /v1/digest/undo`) has no redo in v1, so it is not offered.
- Hard checks: `GET /v1/policy/hard-checks`, rendered as returned. Fixture mode reads `fixtures/api/hard-checks.json`.
- Setup save: one `PUT /v1/setup/settings` per role with the 13 keys `role, install_path, mode, mode_shown, promote, avoid_topics, project_name, listing_url, commission, listing_price, sites_channels, never_topics, box_position`.
  - `commission` is `{type: percent|fixed, value, currency}`; the currency is required for fixed, `''` for percent.
  - `listing_price` is `{value, currency}` or `null`.
  - Both are `null` for publishers.

### Expected after must-fix (Product Lead review of fb493f0). UI wired; fb493f0 doesn't do these yet
- (a) Retrying a failed restore re-runs the hard checks. A 422 `{check_id, check_text}` shows inline: "Can't put it back. <check_text> It stays removed." Retry and Undo/Redo disappear; the item is not counted.
- (b) While the claim check is a stub, maker auto/yolo are disabled whenever `approval_reason` is present, with `approval_message` as the reason.
- (c) Key/agent context (no signed-in session):
  - Digest: Undo, Pause and Stop all stay. Redo (incl. the toast), Retry of a failed restore and Resume are hidden/disabled. Plain line: "Putting a box back, Redo and approving need the dashboard".
  - Setup: Continue is disabled with "needs the dashboard".
- (d) `undone_by` (and any other `approved_by`) shown as plain text only when present ("Undone by you / your agent / …"); nothing when absent. Internal ids are never shown.
- (e) Expiry shown only for a prepared, never-committed item (`status:"prepared"` + `expires_at`). Never for committed or placed boxes.
