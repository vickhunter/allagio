package handlers

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

Auth is the owner session or a project key. When nothing is saved, ` + "`mode`" + ` and ` + "`mode_shown`" + ` are the default for ` + "`install_path`" + `.

| install_path | default mode |
| --- | --- |
| cli, cursor, connector, grok | yolo |
| chatgpt_dir, claude_dir | manual |

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
  "commission": "30%"
}
` + "```" + `

Publisher body fields are ` + "`sites_channels`" + `, ` + "`never_topics`" + `, and ` + "`box_position`" + ` (` + "`after_intro`" + `, ` + "`matched_section`" + `, ` + "`end`" + `, ` + "`main_post`" + `, ` + "`first_reply`" + `, ` + "`early_mention`" + `, ` + "`description`" + `, ` + "`pinned_comment`" + `).

The row stores ` + "`mode_shown`" + `, the chosen ` + "`mode`" + `, the owner id, and the time. Saving also sets that mode on the owner's site, or on the maker's offers.
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

` + "`GET /v1/listing-cards/:id`" + ` returns the stored card for the same maker.
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

An owner session may loosen or tighten. An agent key may only tighten (` + "`yolo`" + ` to ` + "`auto`" + ` to ` + "`manual`" + `) and may not widen the rules. Offer copy, page content, and feedback are rejected and do not change the policy. Every mode change stores ` + "`owner_ui`" + ` or ` + "`agent_tool`" + `, the user id, and the time.

## Hard checks

These run in every mode:

- disclosure text plus rel=sponsored
- claim check (stub until D-028)
- copy at most 140 characters
- one box per post
- relevance at least 25
- offer is listed
- revoke still works

## Decide

` + "```" + `
POST /v1/placements/:id/decide
{"relevance":55,"position":"end","rel_sponsored":true,"copy_edited":false,"offer_data":"the offer summary"}
` + "```" + `

Manual, or anything outside an auto policy, returns ` + "`decision: needs_owner`" + ` and does not approve. Auto and yolo that pass record ` + "`approved_by`" + ` (` + "`policy:auto`" + ` or ` + "`policy:yolo`" + `), ` + "`policy_version`" + `, and ` + "`matched_rule`" + `.

` + "`POST /v1/offers/:id/accept`" + ` with ` + "`site_id`" + `, ` + "`site_quality`" + `, and ` + "`niche`" + ` is the maker mirror.

## Kill switch

` + "```" + `
POST /v1/sites/:id/kill-switch
` + "```" + `

The site returns to manual. Approved placements that are not published yet are paused, and commit refuses them.

## Digest

` + "`POST /v1/events`" + ` with ` + "`kind`" + ` ` + "`digest_opened`" + ` or ` + "`dashboard_opened`" + ` stores the user id and the time.

` + "`GET /v1/digest`" + ` lists placements and offers that went live since the last ` + "`digest_opened`" + `. Each item has a one-time ` + "`undo_token`" + `.

` + "```" + `
POST /v1/digest/undo
{"undo_token":"<token>"}
` + "```" + `

Undo revokes the placement or pauses the offer and stores ` + "`placement_undo`" + ` or ` + "`offer_undo`" + `.
`

const accountDoc = `# Account keys and digest

Two kinds of key exist. A project key comes from device-code approval and works for one site or one offer. An account key works for every site and offer on the account. Neither key can skip the hard checks or loosen a mode.

## Account keys

Create, list, and revoke require the owner session. ` + "`X-API-Key`" + `, ` + "`X-Maker-Key`" + `, and ` + "`Authorization`" + ` are refused with 403.

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

` + "`GET /v1/account/digest`" + ` uses the same auth. ` + "`totals`" + ` covers the whole account. ` + "`projects`" + ` repeats the numbers per site and per offer. Each item that went live has an ` + "`undo_token`" + `.

` + "```" + `
{
  "totals": {
    "went_live": 1,
    "clicks": 2,
    "earnings": {"mode": "test", "publisher": {"mode": "test", "amounts": [{"currency": "USD", "amount": 1.5}]}, "maker": null},
    "clicks_by_source": [{"source": "direct", "clicks": 2}]
  },
  "projects": [],
  "earnings_note": "..."
}
` + "```" + `

` + "`earnings`" + ` is null when no test-mode commission is recorded. Publisher amounts are ledger rows joined to conversions with ` + "`payment_mode=test`" + `. Maker amounts are ` + "`commission_amount`" + ` on those test conversions. Live and legacy rows are not included. Click source is the referrer host from the clicks table. A click with an empty referrer, or a referrer with no host, counts as ` + "`direct`" + `. Account click totals count each click once even when it belongs to both a site and an offer on the same account.

## Hard checks

` + "`GET /v1/policy/hard-checks`" + ` returns the fixed list the dashboard shows. The codes match the decision failures.
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
		return modesDoc, true
	case "account.md":
		return accountDoc, true
	default:
		return "", false
	}
}
