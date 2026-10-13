package handlers

import "affilio-api/internal/entry"

const llmsText = `# Offer Rails

> Agents list a product or place an offer. Humans sign in once. Hard checks always apply.

Offer Rails is a publisher and maker workflow. Tools return offers, matches, and previews to the person deciding on their own content. A chat response is never itself an ad.

## Docs

- [Device login](/docs/device-login.md): device-code sign-in and project-scoped keys
- [Settings](/docs/settings.md): the setup questions and default modes
- [Listing card](/docs/listing-card.md): the only scan payload that may be uploaded
- [Modes](/docs/modes.md): manual, auto, and yolo
- [Account](/docs/account.md): account keys and the account digest

## Start

` + "`POST /v1/device/code`" + ` then open verification_uri. Poll ` + "`POST /v1/device/token`" + ` until it returns a project key.
`

func llmsBody() string {
	return llmsText + "\n" + entry.HardCheckDoc() + "\n" + entry.RestoreRefusalDoc() + "\n" + entry.PluginRetryDoc()
}

const deviceLoginDoc = `# Device login

Sign-in follows RFC 8628. The agent never sees the password. A signed-in human approves the user code in the browser.

## Start

` + "```" + `
POST /v1/device/code
{}
` + "```" + `

Response:

` + "```" + `
{
  "device_code": "<secret>",
  "user_code": "ABCD-EFGH",
  "verification_uri": "https://<host>/device",
  "interval": 5,
  "expires_in": 300
}
` + "```" + `

The device code expires in 5 minutes. The user code is single-use. Show the user code and send the human to verification_uri.

## Poll

` + "```" + `
POST /v1/device/token
{"device_code":"<secret>","grant_type":"urn:ietf:params:oauth:grant-type:device_code"}
` + "```" + `

Wait ` + "`interval`" + ` seconds between polls. Errors use HTTP 400 and ` + "`error`" + `:

| error | meaning |
| --- | --- |
| authorization_pending | the human has not approved yet |
| slow_down | polled too fast; wait the returned interval |
| expired | the code expired, was already exchanged, or is unknown |
| access_denied | the human denied the request |

Success is HTTP 200:

` + "```" + `
{
  "access_token": "<project key>",
  "token_type": "Bearer",
  "key_header": "X-API-Key",
  "scope": "site:<site uuid>",
  "expires_in": 28800
}
` + "```" + `

` + "`key_header`" + ` is ` + "`X-API-Key`" + ` for a publisher site and ` + "`X-Maker-Key`" + ` for a maker offer. Send the access token in that header. The key expires in 8 hours and only works for that site or that offer.

## Approve

The browser calls this with the owner session cookie, ` + "`Origin`" + `, and ` + "`X-CSRF-Token`" + `.

` + "```" + `
POST /v1/device/approve
{"user_code":"ABCD-EFGH","decision":"approve","scope":"site","project_id":"<site uuid>"}
` + "```" + `

` + "`scope`" + ` is ` + "`site`" + ` or ` + "`offer`" + `. ` + "`project_id`" + ` is the site id or the offer id, and it must belong to the signed-in owner. ` + "`decision`" + ` may be ` + "`deny`" + `.

Approval sets ` + "`human_login`" + ` on that owner and records the time. The project key is returned only to the polling agent.

## Agent-only accounts

` + "```" + `
POST /v1/agent-accounts
{"role":"publisher","name":"Blog","email":"agent@example.com","domain":"blog.example","source":"cli"}
` + "```" + `

` + "`role`" + ` is ` + "`publisher`" + ` or ` + "`maker`" + `. An account created here has ` + "`agent_only: true`" + ` and ` + "`human_login: false`" + ` until a human approves a device code for that same owner. ` + "`GET /v1/agent-signals`" + ` (admin key) reports the two groups separately.
`

const settingsDoc = `# Setup settings

One saved answer set per owner and role. The setup screen shows at most four questions.

## Read

` + "```" + `
GET /v1/setup/settings?role=publisher&install_path=cli
` + "```" + `

Auth is the owner session or the site key. A project key is limited to the agent route allow-list and cannot read setup. When nothing is saved, ` + "`mode`" + ` and ` + "`mode_shown`" + ` are the default for ` + "`owners.source`" + `. Every response includes ` + "`mode_preselected`" + `, which the server derives from ` + "`owners.source`" + `. A query ` + "`install_path`" + ` does not select the mode. When nothing is saved and the query omits ` + "`install_path`" + `, or the value is not an install path, the read uses the default for ` + "`owners.source`" + `: cli, cursor, connector, and grok use that same path, mcp uses connector, chatgpt uses chatgpt_dir, claude uses claude_dir, and every other source uses cli. Saving with an empty ` + "`install_path`" + ` stores that same default. An install path that is not one of those values is still rejected. ` + "`human_login`" + ` and ` + "`session.signed_in`" + ` are included too. While claim checks are not live, a preselected or saved yolo or auto is answered as manual, and the response includes ` + "`approval_reason`" + ` and ` + "`approval_message`" + `. ` + "`mode_preselected`" + ` stays the source default.

| owners.source | default mode |
| --- | --- |
| cli, cursor, grok, mcp | yolo |
| chatgpt, claude, direct, and anything else | manual |

## Save

Owner session only.

` + "```" + `
PUT /v1/setup/settings
{
  "role": "maker",
  "install_path": "cli",
  "mode_shown": "yolo",
  "mode": "auto",
  "promote": "API toolkit",
  "avoid_topics": ["gambling"],
  "project_name": "Prompt Forge",
  "listing_url": "https://promptforge.example",
  "commission": {"type": "percent", "value": 30, "currency": ""},
  "listing_price": {"value": 49, "currency": "USD"}
}
` + "```" + `

` + "`project_name`" + ` and ` + "`listing_url`" + ` are returned on every setup response. ` + "`commission`" + ` is ` + "`{type, value, currency}`" + ` with ` + "`type`" + ` ` + "`percent`" + ` or ` + "`fixed`" + `. ` + "`currency`" + ` is required when ` + "`type`" + ` is ` + "`fixed`" + `. ` + "`listing_price`" + ` is optional and is ` + "`{value, currency}`" + `. When both name a currency, they are the same code. Readiness signals and publisher types to avoid are not accepted. Every setup response also includes ` + "`change_url`" + ` (the dashboard URL for the approval mode and the rules) and ` + "`site_url`" + `, plus ` + "`available_modes`" + ` (` + "`manual`" + `, ` + "`auto`" + `, ` + "`yolo`" + `). ` + "`mode`" + ` is the effective mode. While claim checks are not live, a saved yolo or auto is answered as manual with ` + "`approval_reason`" + ` ` + "`claim_check_unavailable`" + ` and ` + "`approval_message`" + `.

Publisher body fields are ` + "`sites_channels`" + `, ` + "`never_topics`" + `, and ` + "`box_position`" + ` (` + "`after_intro`" + `, ` + "`matched_section`" + `, ` + "`end`" + `, ` + "`main_post`" + `, ` + "`first_reply`" + `, ` + "`early_mention`" + `, ` + "`description`" + `, ` + "`pinned_comment`" + `).

The row stores ` + "`mode_shown`" + `, the chosen ` + "`mode`" + `, the owner id, and the time. Saving also sets that mode on the owner's site, or on the maker's offers. The mode chosen here, right after the device-code browser sign-in, is the human choosing. A preselected ` + "`yolo`" + ` for a CLI install is saved. An API key cannot call this save.
`

const listingCardDoc = `# Listing card

The scan stays on the machine. The upload is the card, one pass or fail per signal, and a score. Source, environment files, and secrets are rejected.

` + "```" + `
POST /v1/listing-cards
X-Maker-Key: <maker or offer key>
{
  "product_id": "<uuid, optional>",
  "card": {
    "name": "Prompt Forge",
    "summary": "A writing tool for developers.",
    "category": "devtools",
    "features": ["templates"],
    "pricing_summary": "Free tier"
  },
  "signals": [
    {"name": "payments", "pass": true},
    {"name": "offer_link", "pass": false}
  ],
  "score": 72,
  "payments_check": "pass",
  "offer_link_health": "broken"
}
` + "```" + `

Unknown JSON fields are rejected. ` + "`payments_check`" + ` is ` + "`pass`" + `, ` + "`fail`" + `, or ` + "`unknown`" + `. ` + "`offer_link_health`" + ` is ` + "`ok`" + `, ` + "`broken`" + `, or ` + "`unknown`" + `. Both are stored with the card.

` + "`GET /v1/listing-cards/:id`" + ` returns the stored card for the same maker. That response and ` + "`GET /v1/listings`" + ` include ` + "`review_reason`" + ` and ` + "`message`" + `. ` + "`review_reason`" + ` is ` + "`claims`" + `, ` + "`head`" + `, ` + "`relevance`" + `, or ` + "`caps`" + ` while the offer is in review. Both are null when the offer is listed. On a hosted server the claims check defaults to stub and stays there: hosted refuses real. Local real compares the listing card with the merchant page.
`

const modesDoc = `# Approval modes

Modes are ` + "`manual`" + `, ` + "`auto`" + `, and ` + "`yolo`" + `. They are set per site and per offer. A change applies to new decisions. Publishers see the plain labels "Ask me every time", "Approve within my rules", and "Approve anything that passes the checks".

## Read and write

` + "```" + `
GET /v1/sites/:id/mode
PUT /v1/sites/:id/mode
GET /v1/offers/:id/mode
PUT /v1/offers/:id/mode
` + "```" + `

` + "```" + `
{
  "mode": "auto",
  "allow_makers": [],
  "deny_makers": [],
  "allow_categories": ["devtools"],
  "deny_categories": ["gambling"],
  "min_relevance": 40,
  "allowed_positions": ["end"],
  "max_placements_per_30_days": 3,
  "copy_edits_allowed": false,
  "min_site_quality": 60,
  "allowed_niches": ["devtools"]
}
` + "```" + `

` + "`min_relevance`" + ` defaults to 40 and cannot be below 25. The placement cap defaults to 3 per 30 days. Maker auto rules are ` + "`min_site_quality`" + ` and ` + "`allowed_niches`" + `.

An owner session may loosen or tighten. An account key may only make the rules stricter: a tighter mode (` + "`yolo`" + ` to ` + "`auto`" + ` to ` + "`manual`" + `), a lower cap, a higher minimum relevance, or pausing. A project key is limited to the agent route allow-list. Loosening the mode (` + "`manual`" + ` to ` + "`auto`" + `, ` + "`auto`" + ` to ` + "`yolo`" + `, or ` + "`manual`" + ` to ` + "`yolo`" + `), raising a cap, and lowering minimum relevance are session-only and return 403 when any API key is present. Offer copy, page content, and feedback are rejected and do not change the policy. Every mode change stores ` + "`owner_ui`" + ` or ` + "`agent_tool`" + `, the user id, and the time.

## Hard checks

These run in every mode:

- disclosure text plus rel=sponsored
- claim check (stub until D-028)
- copy at most 140 characters
- one box per post
- relevance at least the saved minimum, which defaults to 40 and cannot be below 25
- offer is listed
- revoke still works
- the 30-day placement cap

## Decide

` + "```" + `
POST /v1/placements/:id/decide
{"relevance":55,"position":"end","rel_sponsored":true,"copy_edited":false,"offer_data":"the offer summary"}
` + "```" + `

The body still requires ` + "`relevance`" + `, ` + "`position`" + `, and ` + "`rel_sponsored`" + `. The server scores relevance from the approved copy and the offer, and rel=sponsored is checked on the plugin re-read of the post, not from disclosure words. Caller ` + "`relevance`" + `, ` + "`rel_sponsored`" + `, and ` + "`offer_data`" + ` are ignored. ` + "`position`" + ` is still read from the body. Manual, or anything outside an auto policy, returns ` + "`decision: needs_owner`" + ` and does not approve. While the claim checker is not live, yolo and auto also return ` + "`needs_owner`" + ` with ` + "`reason: claim_check_unavailable`" + ` after the hard checks pass. Auto and yolo that pass record ` + "`approved_by`" + ` (` + "`policy:auto`" + ` or ` + "`policy:yolo`" + `), ` + "`policy_version`" + `, and ` + "`matched_rule`" + `. The response also includes ` + "`policy_rule_id`" + ` (` + "`hard_checks_passed`" + ` or ` + "`category:invoicing`" + `) and ` + "`policy_rule_text`" + `, a sentence written by the server. Digest items and placement reads include those two fields.

` + "`POST /v1/offers/:id/accept`" + ` with ` + "`site_id`" + `, ` + "`site_quality`" + `, and ` + "`niche`" + ` is the maker mirror.

## Kill switch

` + "```" + `
POST /v1/account/pause
{"scope":"all"}
` + "```" + `

` + "`scope`" + ` is ` + "`all`" + `, ` + "`site`" + `, or ` + "`offer`" + `. ` + "`site`" + ` and ` + "`offer`" + ` require ` + "`id`" + `. The server sets manual mode on every site and offer in that scope and pauses unpublished policy approvals. An account key or a session may pause. A project key is limited to the agent route allow-list.

` + "```" + `
POST /v1/account/resume
{"scope":"site","id":"<uuid>"}
` + "```" + `

Resume is session-only. Any API key receives 403. The signed-in human restores the mode from before the pause and unpauses those approvals.

## Digest

` + "`POST /v1/events`" + ` with ` + "`kind`" + ` ` + "`digest_opened`" + ` or ` + "`dashboard_opened`" + ` stores the user id and the time.

` + "`GET /v1/digest`" + ` lists placements and offers that went live since the last ` + "`digest_opened`" + `. The payload includes ` + "`session.signed_in`" + `, ` + "`paused`" + ` (` + "`account`" + `, ` + "`site`" + `, ` + "`offer`" + `), ` + "`went_live_since_last`" + `, ` + "`change_url`" + `, ` + "`site_url`" + `, and ` + "`available_modes`" + `. While claim checks are not live it also includes ` + "`effective_mode`" + ` ` + "`manual`" + `, ` + "`approval_reason`" + `, and ` + "`approval_message`" + `. Each item includes ` + "`plugin_state`" + `, ` + "`live_now`" + ` (from ` + "`placement_counts_as_live`" + ` for a placement), ` + "`post_title`" + `, ` + "`position`" + `, ` + "`link_text`" + `, ` + "`label`" + `, ` + "`clicks`" + `, ` + "`approved_by`" + `, ` + "`undone_by`" + ` (the owner id when a placement was undone, otherwise empty), ` + "`policy_rule_id`" + `, ` + "`policy_rule_text`" + `, and ` + "`rules_version`" + `. Undoing a listing uses the offer undo token. There is no redo for a listing in v1.

` + "```" + `
POST /v1/placements/:id/revoke
{"reason":"undo"}
` + "```" + `

Undo is that revoke. It takes effect in the same request in the API and in MCP. There is no delay. The site owner is recorded as ` + "`revoked_by_side: publisher`" + `. The offer's maker uses the same endpoint and is recorded as ` + "`maker`" + `. Either records a ` + "`placement_undo`" + ` event with the server time and the key or session that undid it.

` + "```" + `
POST /v1/placements/:id/restore
` + "```" + `

Restore applies only to that placement id, and only within 6 seconds of its server undo timestamp. After that it returns 409 ` + "`{reason, message}`" + ` with ` + "`reason: window_expired`" + `. Restore, which the UI calls redo, on a placement that was not undone returns 409 ` + "`{reason: not_undone, message}`" + `. Only the same user and key identity that undid it may call it. Anyone else, including the other side, gets 403. If Pause or the kill switch was turned on for that site, offer, or account at any point after the undo, including when it is still on, restore returns 409 ` + "`{reason, message}`" + ` with ` + "`reason: paused`" + `. Every 409 from this list returns ` + "`reason`" + ` and ` + "`message`" + `. ` + "`reason`" + ` is ` + "`window_expired`" + `, ` + "`paused`" + `, ` + "`not_undone`" + `, or ` + "`not_failed`" + `. ` + "`message`" + ` is the plain-English text. A 422 returns ` + "`check_id`" + ` and ` + "`check_text`" + `. The server re-runs every hard check first. It also evaluates the current 30-day placement cap and the current minimum relevance Relevance is scored on the server from the approved copy and the offer. The caller cannot supply it. A failing check returns 422 ` + "`{check_id, check_text}`" + ` from the hard-check list below and leaves the placement revoked. On success the placement returns to the status it had before the undo, ` + "`approved_by`" + ` stays the original value, ` + "`restored_by`" + ` records the user, and a ` + "`placement_restore`" + ` event stores the actor.

There is no push channel. Clients poll ` + "`GET /v1/placements/:id`" + `. That read is one row from ` + "`placement_proposals`" + ` and allows one request every 2 seconds per placement per session, so a poll every 4 seconds still succeeds. The body includes ` + "`status`" + `, ` + "`plugin_state`" + `, ` + "`plugin_failed_reason`" + `, ` + "`failed_action`" + `, and ` + "`fallback_url`" + `. ` + "`fallback_url`" + ` is the offer's plain URL, so the plugin can cache it. A maker project key can read the status of a placement on its own offer. Retry stays publisher-only and returns 403 for a maker.

Every placement in ` + "`GET /v1/digest`" + `, ` + "`GET /v1/account/digest`" + `, and the status read includes ` + "`plugin_state`" + ` and ` + "`plugin_failed_reason`" + `. ` + "`plugin_state`" + ` is never null. The values are ` + "`pending`" + `, ` + "`placed`" + `, ` + "`removed`" + `, ` + "`offline`" + `, and ` + "`failed`" + `. A brand-new placement is ` + "`pending`" + ` until the plugin key's first acknowledgement (` + "`action: placed`" + `), then ` + "`placed`" + `. An upgrade of an existing box uses that same ` + "`action: placed`" + `. After a restore it is ` + "`pending`" + ` until ` + "`POST /v1/placements/:id/plugin-ack`" + ` with ` + "`action: restored`" + `, which also sets ` + "`placed`" + `. After a revoke it is ` + "`pending`" + `, then ` + "`removed`" + ` on ` + "`action: removed`" + `, or ` + "`offline`" + ` after ` + "`PLUGIN_OFFLINE_AFTER`" + ` (default 10 minutes) with no acknowledgement. ` + "`offline`" + ` is only that missing acknowledgement. ` + "`action: failed`" + ` stores ` + "`reason`" + ` in ` + "`plugin_failed_reason`" + `, stores ` + "`failed_action`" + ` as ` + "`removed`" + ` or ` + "`restored`" + `, and sets ` + "`plugin_state`" + ` to ` + "`failed`" + ` after the plugin's post re-read fails and one retry fails. A later verified acknowledgement for that same action clears the failure.

` + "```" + `
POST /v1/placements/:id/plugin-ack
X-Plugin-Key
{"action":"placed","rel_sponsored":true}
` + "```" + `

` + "`action`" + ` is ` + "`placed`" + `, ` + "`removed`" + `, ` + "`restored`" + `, or ` + "`failed`" + `. The request authenticates with ` + "`X-Plugin-Key`" + `. ` + "`X-API-Key`" + `, a project key, an account key, an agent key, and a session receive 403. ` + "`action: placed`" + ` and ` + "`action: restored`" + ` require ` + "`rel_sponsored: true`" + `, the boolean the plugin read from the post. Disclosure words do not satisfy it. A false value returns 422 ` + "`rel_sponsored`" + ` and does not set ` + "`placed`" + `. ` + "`failed`" + ` requires ` + "`reason`" + `. The WordPress plugin calls ` + "`placed`" + ` the first time the block is in the post and again on upgrade for a box that is already there. It calls ` + "`restored`" + ` after it puts the block back. It calls ` + "`removed`" + ` after it deletes the block. It calls ` + "`failed`" + ` with a reason when the post re-read still fails after one retry. None of those calls creates a post revision. A restore acknowledgement is only valid while the placement is brand new or waiting on a re-insert. A removal acknowledgement is only valid while a revoke is waiting, including after the window has already marked it ` + "`offline`" + `. A failure acknowledgement is valid for a brand-new placement, a restore, or a removal that is still waiting, including one already marked ` + "`offline`" + `.

` + "```" + `
POST /v1/placements/:id/retry
` + "```" + `

The publisher retries with the owner session or the site key. The other side receives 403. Retry repeats only ` + "`failed_action`" + `. Any state other than ` + "`failed`" + ` returns 409 ` + "`{reason: not_failed, message}`" + `. A failed removal re-queues only the removal: ` + "`pending`" + `, then ` + "`removed`" + `. It does not re-run the hard checks, it does not become ` + "`placed`" + `, and the go link stays stopped. A failed restore re-runs every hard check. A failure returns 422 ` + "`{check_id, check_text}`" + ` and leaves the placement failed. Success re-queues the restore: ` + "`pending`" + `, then ` + "`placed`" + `.

A failed restore is not live. ` + "`GET /v1/go/<placement_id>`" + ` redirects to the offer's plain URL and records no click, until a retried restore is confirmed ` + "`placed`" + ` by ` + "`action: restored`" + `. Digest totals, site totals, and Live now exclude it, including the went-live count and earnings from clicks on that placement. A successful restore that has not failed still redirects. It counts as live only after ` + "`plugin_state`" + ` is ` + "`placed`" + `. Undo and redo before the plugin acknowledges a removal leaves a box that is still showing in ` + "`placed`" + `, and that go link keeps counting clicks. A placement counts as live only when ` + "`plugin_state`" + ` is ` + "`placed`" + ` and its go link is redirecting.

Leave a failed restore removed with the same undo while ` + "`plugin_state`" + ` is ` + "`failed`" + ` and ` + "`failed_action`" + ` is ` + "`restored`" + `. That request sets ` + "`plugin_state`" + ` to ` + "`pending`" + `, clears ` + "`failed_action`" + `, and stops the go link. The placement does not count as live. It becomes ` + "`removed`" + ` only on a verified ` + "`action: removed`" + `. With no acknowledgement it becomes ` + "`offline`" + ` after ` + "`PLUGIN_OFFLINE_AFTER`" + ` (default 10 minutes). ` + "`action: failed`" + ` there sets ` + "`failed`" + ` with ` + "`failed_action`" + ` ` + "`removed`" + `. ` + "`placed`" + ` and ` + "`removed`" + ` are stored only by that verified plugin acknowledgement. Revoke, restore, retry, pause, and the kill switch do not set them.

` + "`GET /v1/go/<code>`" + ` stays supported. Existing posts, feeds, and caches keep that link, and nothing rewrites a saved post. It never returns a 4xx or a 5xx. A code with ` + "`?placement=`" + ` follows the placement rules. Without that query, a hit is recorded only when ` + "`placement_counts_as_live`" + ` matches a placement on the campaign's site. Every other resolved code redirects to the offer's plain URL with no ref and no hit. When the offer is not live, that redirect is the site home. An unknown code redirects to the Offer Rails home page. A rate limit answers with the same untracked 302 and never a 429. ` + "`GET /v1/go/<placement_id>`" + ` returns 302 when the placement exists. A ref is attached and a hit is recorded only when ` + "`placement_counts_as_live`" + ` is true. Revoked, pending undo, paused, kill switch, offline, failed, and removed placements redirect to the offer's plain URL with no ref and no hit. When the offer is not live, the redirect is ` + "`post_url`" + `, or the site home when that is missing. An unknown placement id returns a 404 page and does not redirect, so the request port stays. Undo and redo before the plugin acknowledges a removal leaves a box that is still showing in ` + "`placed`" + `, and that go link keeps counting clicks. Every response sends ` + "`Cache-Control: no-store`" + ` and ` + "`X-Robots-Tag: noindex, nofollow`" + `. The plugin calls ` + "`GET /v1/go/<placement_id>?via=plugin`" + ` with ` + "`X-API-Key`" + ` and may send ` + "`X-OR-Client-Hash`" + `, ` + "`X-OR-User-Agent`" + `, and ` + "`X-OR-Referer`" + `. ` + "`X-OR-Client-Hash`" + ` is the lowercase hex HMAC-SHA256 of the truncated address (` + "`/24`" + ` for IPv4, ` + "`/48`" + ` for IPv6), keyed by the daily salt. The site salt rotates at 00:00 UTC. Those headers are used only when the key matches the placement's site. A direct hit uses the server salt for the same truncation and the same UTC day. The raw address is not stored or logged. ` + "`fallback_url`" + ` on the placement payload is that plain offer URL.
`

const accountDoc = `# Account keys and digest

Two kinds of key exist. A project key comes from device-code approval and works for one site or one offer. An account key works for every site and offer on the account. Neither key can skip the hard checks, loosen a mode, raise a cap, lower minimum relevance, or resume a pause. Those four actions are session-only. Pause (` + "`POST /v1/account/pause`" + `) is allowed for an account key or a session. A project key is limited to the agent route allow-list and receives 403.

## Account keys

Create, list, and revoke require the owner session. Creating an account-wide key is session-only: ` + "`X-API-Key`" + `, ` + "`X-Maker-Key`" + `, and ` + "`Authorization`" + ` are refused with 403, including a project key and an account key.

` + "```" + `
POST /v1/account-keys
GET /v1/account-keys
POST /v1/account-keys/:id/revoke
` + "```" + `

The plaintext key is returned once, on create. The list shows ` + "`key_prefix`" + ` and whether the key is revoked.

Send the account key as ` + "`X-API-Key`" + ` for the publisher site, or ` + "`X-Maker-Key`" + ` for the maker's offers. It is not limited to one offer. A project key sent to another site or offer is 403.

## Projects

` + "`GET /v1/account/projects`" + ` lists the account's sites and offers with the current mode. A project key is 403. An account key or the owner session is accepted.

## Account digest

` + "`GET /v1/account/digest`" + ` uses the same auth. ` + "`totals`" + ` covers the whole account. ` + "`projects`" + ` repeats the numbers per site and per offer. Each item that went live has an ` + "`undo_token`" + `. The payload includes ` + "`session.signed_in`" + `.

` + "```" + `
{
  "totals": {
    "went_live": 1,
    "clicks": 2,
    "earnings": 1.5,
    "clicks_by_source": [{"source": "direct", "clicks": 2}]
  },
  "projects": [],
  "session": {"signed_in": false},
  "earnings_note": "..."
}
` + "```" + `

` + "`earnings`" + ` is null when that scope has no test-mode rows, and 0 when the recorded rows add up to zero. Publisher amounts are ledger rows joined to conversions with ` + "`payment_mode=test`" + `. Maker amounts are ` + "`commission_amount`" + ` on those test conversions. Live and legacy rows are not included. Earnings from a click on a placement whose restore failed are omitted until that restore is confirmed ` + "`placed`" + `. ` + "`went_live`" + ` is Live now, the site total, the digest went-live count, and the account total. A placement counts as live only when ` + "`plugin_state`" + ` is ` + "`placed`" + ` and its go link is redirecting: still committed, offer listed, campaign active, and Pause and the kill switch off for the site and the offer. Pending, failed, offline, removed, revoked, and paused placements are excluded. The rule is the ` + "`placement_counts_as_live`" + ` view. Digest totals, site totals, and Live now exclude a failed restore. Click source is the referrer host from the clicks table. A click with an empty referrer, or a referrer with no host, counts as ` + "`direct`" + `. Account click totals count each click once even when it belongs to both a site and an offer on the same account.

## Hard checks

` + "`GET /v1/policy/hard-checks`" + ` is the single source the UI renders. It returns ` + "`checks`" + ` of ` + "`check_id`" + ` and ` + "`check_text`" + ` from the same list a 422 uses. The UI does not keep a second copy. There is no push channel; clients poll placement status.
`

func agentDoc(name string) (string, bool) {
	switch name {
	case "device-login.md":
		return deviceLoginDoc, true
	case "settings.md":
		return settingsDoc, true
	case "listing-card.md":
		return listingCardDoc, true
	case "modes.md":
		return modesDoc + "\n" + entry.HardCheckDoc() + "\n" + entry.RestoreRefusalDoc() + "\n" + entry.PluginRetryDoc(), true
	case "account.md":
		return accountDoc, true
	default:
		return "", false
	}
}
