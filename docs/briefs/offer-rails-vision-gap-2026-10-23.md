# Offer Rails: vision gap check, 23 Oct 2026 (D-096 evidence)

Written Fri 23 Oct 2026, 07:00–07:20 UTC (09:00–09:20 Europe/Rome).
Author: AI Engineer cloud agent (Claude Opus 5.5), D-096 evidence pass. Oracle owns the pass and the verdict. This file is evidence and proposals only.
Reader: Product Lead, who turns the gaps into roadmap rows.
Scope: documentation only. Nothing was merged, deployed, published, submitted or paid for. No secret values were read or printed. Hosted `CLAIMS_CHECK` was not touched. `AUTO_MIGRATE` was not touched.

"unknown" means no evidence was found in the sources at the end of this file. Nothing is guessed.

## 0. Summary

- The protocol is the largest gap. Main serves DCP v0.1 for one deal type (affiliate placement). Agent-to-agent negotiation, link exchange, traffic swap, cross-sell or upsell, and reposts exist only in two unmerged drafts: PR #68 (Gemini, the fuller one) and PR #71 (Opus, schemas and SDK only). The Sonnet branch has no spec content. D-092 is not done.
- The in-flow "you can monetize this post, want to try?" suggestion (D-097) is not built. Victor called it "fundamental" on 21 Oct.
- The human guardrails are in good shape on main: per-step auto, ask or off (D-062), new sites ask before publishing (D-088), sites with no saved mode now ask too (D-093b), domain verification (D-063b), and a real claims check you opt into (D-066). D-093b is merged but not on hosted yet.
- Distribution is not started. There's no npm package (`offer-rails` returns 404 on npm), no listing in the official MCP registry, no public repo (`vickhunter/offer-rails` is 404), no `LICENSE`, and the MCP server still calls itself `distributoor`. Every submission needs Victor.
- Dogfooding is blocked. Italian matching (D-061) is not built. The Vibesales Stripe test-mode leg (D-054) is waiting. The live blog's host firewall blocks AI crawlers (as of 20 Oct).
- Correction to the task input: PR #73 (D-093b) is **merged**, not an open draft. Victor (`vickhunter`) merged it at 07:05:18 UTC today as `27e0a63`. Hosted still runs `a84646b`, so main is one merge ahead, and migration 047 is not applied on hosted.

## 1. Confirmed vision

### 1.1 Victor's 21 Oct restatement (verbatim, confirmed)
Source: `decisions.md`, 2026-10-21. Projects Manager wrote it and Victor answered "correct".

> "Offer Rails is the protocol any AI agent uses to get its human more traffic, engagement, clients or income, on any platform and with any agent. While it helps write a blog post, social post or newsletter, it suggests a matching offer ("this fits your post, want to monetize it?"). It deals with other agents directly for affiliate links, link exchanges, cross-sells, upsells and reposts. That means no cold emails and no signing up on websites, it runs at agent speed, the human's limits always come first, and no personal data is ever shared. It replaces both manual affiliate signups and email-spamming outreach bots."

Same day, as logged in `decisions.md`:
- "protocol must work on any platform like bitly/affiliate links; agents find opportunities autonomously, agent-to-agent, respecting human constraints, no personal sensitive info."
- "all platforms go live and tested in steps without waiting for the protocol; adapter issues feed back into the protocol." (D-095)
- 21 Oct 11:3x, in `roadmap.md` under D-095: the core of the protocol is a universal tracked link, and platform adapters are only add-ons.
- 21 Oct 18:47 and 18:56: open source it, Apache-2.0, public repo `vickhunter/offer-rails`, pushed only after D-099 passes and the D-100 full-history scan is clean.

### 1.2 Victor's new ask, 23 Oct 2026, Offer Rails room
This is the wording relayed in the D-096 task. I didn't see the raw room message, so treat it as a close paraphrase, not a quote.

- His Cursor usage has reset, so bots should use paid models again.
- He wants **one owner** who makes sure everything said so far was considered and nothing was missed, and who carries the adjustments, the implementation and the testing.
- The protocol must be built, and the marketplace too.
- It's agent-to-agent, with the human in the loop, guardrails, and PII/privacy by default.
- End goal: any AI agent works for its human's benefit (visibility, engagement, blog, partnerships), across any mix of digital growth deals that today need a manual process.
- The vision: open any agent or terminal, ask how to monetize your content, and Offer Rails (or "OfferAI") comes up as the answer. Integrations go everywhere (Grok Bot marketplace, MCP plugin marketplaces, Manufact if it's useful, ChatGPT, Anthropic), but the thin layer has to work without them.
- Zero budget. Offer Rails becomes the standard through distribution and dogfooding, because Victor has the same problem himself: monetizing and growing his own content.
- Agents help agents get discovered.

### 1.3 Requirements derived from 1.1 and 1.2
These IDs are used in the gap table.

| ID | Requirement | From |
|---|---|---|
| V1 | One open protocol for any combination of digital spaces and deal types (affiliate, link exchange, traffic swap, cross-sell or upsell, repost or retweet, partnership) on any platform | 21 Oct, 23 Oct |
| V2 | A universal tracked link is the core, and it works with no adapter (email, X, blog, video) | 21 Oct 11:3x |
| V3 | Agents deal with other agents directly: discover, offer, counter, agree, prove, at agent speed | 21 Oct, 23 Oct |
| V4 | The human's limits come first: approval per step, guardrails that can't be bypassed | 21 Oct, 23 Oct, 18 Oct 13:22 |
| V5 | PII and privacy by default; no personal data is ever shared with the other agent | 21 Oct, 23 Oct, 8 Oct North Star |
| V6 | No account, no email and no website signup where possible | 21 Oct, 23 Oct |
| V7 | A thin layer that works with no integration (link + `/mcp` + `llms.txt` + REST); adapters are optional extras | 21 Oct, 23 Oct |
| V8 | In-flow suggestion: a content agent says "you can monetize this post, want to try?" | 21 Oct 12:26, 23 Oct |
| V9 | Any agent or terminal asked "how do I monetize this content?" lands on Offer Rails (discovery / AEO) | 23 Oct, 14 Oct protocol doc §1 |
| V10 | Marketplace and distribution: Grok Bot MCP marketplace, official MCP registry, Anthropic and ChatGPT ecosystems, Manufact if it fits | 23 Oct, 10 Oct 01:38 |
| V11 | Agents help agents get discovered: a public directory of makers and publishers, filled by agents | 23 Oct, 18 Oct |
| V12 | Dogfood on victormotricala.it and the vibesales.tech Stripe test-mode run | 18 Oct, 23 Oct |
| V13 | Zero budget, open source (Apache-2.0), become the standard | 21 Oct 18:56, 23 Oct |
| V14 | One owner across considered, implemented and tested; use the paid models | 23 Oct |
| V15 | Visuals, examples, video and copy (Launch & Distribution / Motion) | 23 Oct ("later") |

## 2. State snapshot (read-only checks, 23 Oct 07:0x UTC)

| Item | Evidence |
|---|---|
| GitHub main | `27e0a63` = D-093b (PR #73), merged 07:05:18 UTC by `vickhunter`. Previous tip `a84646b` (D-093a). |
| Hosted https://offer-rails.onrender.com | `/health` → `{"commit":"a84646b…","status":"ok"}`. **One merge behind main.** Migration `047_d093b_placement_publish_ask.sql` is on main; whether it's applied on hosted is unknown, and it's presumably not, since the deploy hasn't happened. |
| Open PRs | #68 (Gemini ORP 0.3 draft), #71 (Opus ORP 0.3 draft, WIP), #72 (D-098 landing, draft, "Do not merge"). No open PR from the Sonnet branch. |
| Protocol branches | `cursor/protocol-v03-gemini` (#68, head `3577ba0`, 48 files: `spec/ schema/ sdk/ adapters/ conformance/ GAPS.md`). `cursor/protocol-v03-opus` (#71, head `1f2f112`, 121 files under `schema/ sdk/` only). `cursor/protocol-v03-sonnet` (tip `7eb8ee7`, one `llms.txt` commit on top of `bf334e9`, no spec). |
| Hosted protocol descriptor | `/.well-known/dcp.json`: `"dcp_version":"0.1"`, `"name":"distributoor"`, stdio command `distributoor-mcp`. |
| MCP server name in code | `"distributoor"` at `affilio-api/internal/dcp/mcp.go:53`, `internal/handlers/hosted_mcp.go:69` and `cmd/server/main.go:124`. |
| MCP tools on main | 26 tools, including `find_opportunities`, `prepare_placement`, `join_offer`, `get_step_modes`, `set_step_modes`, `undo_placement`, `preview_revenue`. None is a suggest, draft-content, negotiate or swap tool. |
| Agent join | `POST /v1/agent-tokens` with `{}` returns a 24-hour bearer for `/mcp`. No role, no email (`agent_entry.go:76`, `llms.txt`). `/mcp` without a bearer → 401. |
| CLAIMS_CHECK | In code: hosted (`GIN_MODE=release`) defaults to `stub`; `off-local` is refused on hosted; `real` is allowed (`internal/config/config.go:117-144`). Hosted value: `stub` per roadmap 21 Oct 13:26. I didn't re-check it today, because env values are not readable without the Render dashboard. |
| npm | `npm view offer-rails` → E404 (not published). Two local packages are both named `offer-rails` 0.1.0: `cli/package.json` and `packages/distributoor-setup/package.json`. |
| Official MCP registry | A `search=offer` query returned 25 servers and none is Offer Rails. Two names are protocol-like: `io.github.Chris85appding/unified-offer-protocol` and `com.textforagents/standing-offer-envelopes-offline-agent-deals`. I didn't look into what they are. |
| Public repo / license | `github.com/vickhunter/offer-rails` → 404. No `LICENSE` file on main. |
| Hosted pages | 200: `/`, `/agents`, `/makers`, `/publishers`, `/protocol`, `/privacy`, `/terms`, `/llms.txt`, `/docs/modes.md`. 404: `/openapi.json`, `/.well-known/agent-card.json`, `/robots.txt`, `/sitemap.xml`, `/for/cursor`, `/prompts`, `/you` (`/you` lives in draft PR #72). |
| Public copy check | On `/`, `/agents`, `/makers`, `/publishers` and `/protocol`: 0 hits for "ad"/"ads", "fully automatic" and "open source". `/protocol` is rendered in the browser, so its text wasn't inspected. |
| `llms.txt` client claims | Cursor, VS Code and Windsurf are marked "Untested". Claude Code: "sign-in to the hosted MCP server is not working yet". Generic: "Checked" (401 on initialize without a bearer). |

## 3. Gap table

Tags: **missing** = no build. **mismatch** = what's built or planned contradicts the vision or the copy. **improve** = it works, but short of the vision.
Severity: **blocker** = the vision statement is false without it. **high** = it blocks distribution or dogfooding. **med** / **low**.
"Built" = on GitHub main `27e0a63`. "Hosted" = `a84646b`.

| # | Vision | Plan (roadmap) | Built on main? | Hosted? | Tag | Sev | Next action |
|---|---|---|---|---|---|---|---|
| 1 | V1: one protocol, any space, any deal type | D-092 (protocol v0.1, MCP-style spec), D-022 DCP v0.2 (accepted 8 Oct, internal draft), protocol doc v0.0 (14 Oct) | No. Main is DCP v0.1, which covers maker offers and publisher placements for affiliate commission. `grep` finds 0 Go files with `traffic_swap`, `link_exchange` or `repost`. The spec exists only in drafts: #68 has 5 deal types, a spec, schemas, a conformance suite and adapters (26 tests and 11/11 reference checks per its PR body). #71 has schemas and an SDK only (24 tests per its PR body). | No. `dcp.json` says 0.1. | missing | blocker | Pick one base (#68 has the most surface) and fold #71's signing, JCS hashing and SDK in. Write the merged spec into one branch. D-092 acceptance stays as written: a fresh agent with only the spec URL completes discover → offer → accept → proof against hosted, on WordPress and on one non-WordPress surface. |
| 2 | V1: protocol name and version are consistent | protocol doc §12 Q1 (name); D-056 (server name) | Mismatch. Code says "distributoor" (3 places). Docs say DCP. Drafts say ORP 0.3. The task says "Offer Rails / OfferAI". | `dcp.json` name `distributoor`. | mismatch | high | Product Lead asks Victor for one public protocol name and version line. AI Engineer then renames `serverInfo`, the `dcp.json` name and the stdio command (D-056 item 2). That's a small, contained change. |
| 3 | V2: universal tracked link, no adapter | D-092 core (Victor 21 Oct 11:3x), GAP-01 | Yes for counting. Clicks count from the first `/v1/go/` redirect without plugin-ack (#70). Source comes from the referrer host, and an email is `direct` (`STATUS.md` 21 Oct). The raw IP is scrubbed, and the address is hashed truncated with a daily salt (`internal/handlers/go_privacy.go`). | Yes (`a84646b` includes #70). | improve | high | Run and record the D-092 link acceptance on hosted: one link clicked from an email, an X DM (private) and the staging blog, each shown with its source and no PII. The result of Site Keeper's 21 Oct 13:26 hosted re-run is unknown. |
| 4 | V3: agent-to-agent dealing (counter-offer, terms, signatures, round cap) | protocol doc §2 "negotiate / terms"; D-091 rooms study (due Fri 24 Oct); D-092 | No. `grep negotiat\|counter_offer` finds 0 Go files. #71 claims Ed25519 signing and a reference host in TypeScript. | No | missing | blocker | Define the smallest negotiation: propose → counter (max 3 rounds) → accept with signed terms, with each step under the owner's D-062 mode. D-091 says go/no-go on "rooms". Don't build rooms before D-092 v0.1. |
| 5 | V4: human limits first; per-step approval | D-031, D-062, D-088, D-093b, D-063b, D-066 | Yes. Per-step auto/ask/off (`aea81ca`). New sites ask (`bf334e9`). No-row sites ask (`27e0a63`). Domain verification (`70dc291`). Real claims check is opt-in (`98f152d`). A join bearer can't approve or set publish to auto (`STATUS.md` 21 Oct). | Partly. Hosted lacks D-093b, so an existing site with no saved row still reads every step as auto on hosted (`STATUS.md` 21 Oct, D-093a note). Hosted claims check is `stub`, so a stored auto still waits with `claim_check_unavailable`. | improve | high | Victor: deploy `27e0a63` and hand-apply migration 047 (Neon snapshot first). Then Site Keeper runs the D-093b acceptance on hosted. Only after that does the `CLAIMS_CHECK=real` decision come up (row 6). |
| 6 | V4 vs Victor 18 Oct 13:22 ("automated unless the user explicitly wants to be in the loop") | D-062 scope update said the default is auto; D-088 and D-093b changed publish to ask until real claims ship | Publish defaults to ask; every other step defaults to auto. | Same once deployed. | mismatch (deliberate, interim) | med | Product Lead logs the exit condition: when do new sites flip `placement_publish` to auto? Proposal: after `CLAIMS_CHECK=real` is live on hosted, D-099 passes, and Victor says so. Victor confirms or overrides. |
| 7 | V5: PII and privacy by default | D-022 (allowlist, deny-list, k=50, ranges), D-012b plugin allowlist, protocol doc §4.1 `pii.*` conformance tests | Partly. Click privacy is built (row 3). The plugin registration allowlist was accepted 8 Oct. A listing-upload denylist exists (`internal/entry/listing.go:45`). The Audience card is not built: `grep min_group_size` → 0. The `pii.*` conformance suite is not on main (#68 has a conformance folder; which checks it covers wasn't read). `POST /v1/agent-accounts` still requires `name` and `email` (`llms.txt` step 1). The `{}` agent-tokens path doesn't. | Same as main, minus D-093b. | improve | high | In D-092: make `pii.reject_unknown_fields`, `pii.reject_small_segment` and `pii.no_raw_ip_in_events` runnable against hosted. D-099's privacy reviewer checks them. Decide whether `agent-accounts` can drop `email` (Victor/Product Lead). |
| 8 | V6: no account where possible | D-093a (done), D-097, D-092 ("credential-free suggest and join" in #71) | Partly. An agent joins with `{}` (`a84646b`). A person still needs Google sign-in for the owner dashboard and for approval. In-flow suggest with no signup isn't built (row 9). | Join: yes. | improve | high | Keep the human sign-in only for approve, pay and owner settings. In D-097, let the suggest call work with an anonymous join bearer and return a claim link the human can open later (the "deferred claim" shape in #68). |
| 9 | V8: in-flow "you can monetize this post, want to try?" | D-097 (ready, after D-093a, which is now done) | No. 0 suggest or draft tools in the MCP list. #68 specifies `POST /v1/orp/inline-suggest` with an ephemeral party, and #71 has an example writing assistant. Both are drafts. | No | missing | blocker | Build D-097 right after the D-092 schema for `suggest` is fixed. First test: X Copywriter on the box drafts a post, gets 0–3 offers plus a tracked link, and the human sees the question. Test mode only, nothing posted. |
| 10 | V1/V9: deals beyond affiliate (visibility, engagement, partnerships) | D-092; D-078 non-WordPress surfaces (queued); D-077 `draft_content` (queued) | No. Affiliate placement only. `surface_url` appears in 7 Go files. Whether a non-WordPress placement plus receipt has been proven end to end is unknown (D-078 isn't in main's commit log). | No | missing | high | Ship deal types in protocol order: affiliate (done) → link exchange → traffic swap → repost → cross-sell/upsell. Each ships with a conformance test and a "Coming" label until Site Keeper passes it. |
| 11 | V7: thin layer works with no integration | D-092 (adapters optional), D-095 (optional extras), D-093/D-093a | Mostly. Universal link, one-line join, `/mcp`, `llms.txt` and REST all exist. The WordPress plugin is one adapter. No `openapi.json` (`STATUS.md` 20 Oct: "There is no OpenAPI file"). | Yes, except D-093b. | improve | med | Add `/openapi.json` and `/.well-known/agent-card.json` (A2A) to the D-092 reference server, so a cold agent has 3 machine entry points. Both are free files we own (agent-channels doc rows 2–3). |
| 12 | V9: any agent asked "how do I monetize?" finds Offer Rails | D-029, D-055 (done), D-064 Context7 patterns (ready), `/for/` pages and `/prompts` (agent-channels doc), AEO Monitor (pending Victor) | Partly. `/agents` and `llms.txt` exist. No `/for/*` pages and no `/prompts` (both 404). D-064 isn't in main's commit log. | `/agents` and `llms.txt` 200. | missing | high | After D-092: write `/for/cursor`, `/for/claude-code`, `/for/chatgpt`, `/for/grok` and 3 tested copy-paste prompts ("monetize my newsletter" and so on). Each one passes the paste test first. Measuring citation (AEO Monitor) needs Victor's yes. |
| 13 | V10: Grok Bot MCP marketplace | D-052 spec (accepted), D-080 marketplace plugin (queued after D-069 + D-067) | No submission. Known blockers: server name `distributoor` (row 2); legal pages still DRAFT (`LEGAL_PAGES_APPROVED` gate in `internal/handlers/hosted_auth.go:176`); reviewer test account (unknown); claims check stub; the D-052 §7 submit step is Victor's. | `/mcp` live with bearer join. | missing | high | D-080: build the bundle (hosted `mcp.json` + 3 skills: find offers, place with disclosure, show earnings) and its requirements file. Submission only with Victor's OK. |
| 14 | V10: official MCP registry | agent-channels doc row 4 (`server.json`, name `io.github.vickhunter/offer-rails`, `mcp-publisher` with GitHub login) | No `server.json` in the repo. Not in the registry (search 23 Oct). | `/mcp` is publicly reachable, which the registry requires. | missing | high | Stage `server.json` (remote-only, streamable-http) in the repo. Publishing uses Victor's GitHub identity: his action, his OK. Smithery, Glama and PulseMCP pull from the registry. |
| 15 | V10: Anthropic and ChatGPT ecosystems | D-030 MCP Apps (OAuth for human approve); protocol doc §5 (ChatGPT custom connectors get read/fetch only; "Dots" not in the EEA) | Not built. `llms.txt` marks Claude Code as not working for sign-in and Cursor, VS Code and Windsurf as untested. | — | missing | high | First, prove Claude Code and Cursor against hosted with the agent-tokens bearer and flip `llms.txt` from Untested to Checked with a date. Then do D-030. Directory submissions need Victor. |
| 16 | V10: Manufact (if it fits) | D-080 add ("hosted /mcp gets A or better (90+) on a free Manufact self-check"); D-081 competitors from the Manufact leaderboard (accepted) | Self-check: unknown, no result file found. | — | improve | med | Run the free Manufact self-check after the rename (row 2) and record the score. Listing or asking for a review needs Victor. If the score adds nothing beyond the registry, drop it (Victor said "if useful"). |
| 17 | V10: npm / one-line install | D-067 (one package; staging only, publish needs Victor) | Two packages share the name `offer-rails` 0.1.0 (`cli/`, `packages/distributoor-setup/`). The second also exposes an `affilio` bin. | `npm view offer-rails` → 404. | mismatch | high | D-067: keep one package. It runs `npx offer-rails mcp` (stdio) or prints the hosted URL config, and its version goes above 0.1.0. Stage only. Publishing needs Victor. |
| 18 | V11: agents help agents get discovered (directory) | D-059 back end (ready), D-060 pages (done, "empty until D-059"), D-072 unverified catalog, D-075 | Pages: yes (`017265f`). Back end D-059 isn't in main's commit log. Self-listing by one MCP call: not in the tool list. | `/makers` and `/publishers` 200. Whether real listings show is unknown (pages rendered in the browser weren't inspected). | missing | high | D-059 after D-092 v0.1: `list_me` / `unlist_me` MCP tools, profile auto-filled from the member's own site, opt-in only. Also expose the directory as a protocol `discover` read, so agents find each other and not only humans. |
| 19 | V12: dogfood on victormotricala.it | D-054 (waiting), D-061 Italian matching (ready), D-012 line (plugin) | No. Italian posts score 9–13 against a threshold of 25 (vision-gap 18 Oct, row 5). D-061 isn't in main's commit log. The live blog ran an old affilio-wp 0.1.6 against a Railway backend on 8 Oct, and its status today is unknown. The host firewall (Tophost) returned 406 to AI crawlers on 20 Oct, with the fix waiting on Tophost (roadmap D-091 evidence). | — | missing | high | Do D-061 before any live-blog test. Site Keeper checks whether Tophost still returns 406. Any change on the live blog (plugin swap, key rotation) is Victor's call. |
| 20 | V12: vibesales.tech Stripe test mode | D-054 (waiting), D-058 readiness for browser-rendered sites (ready), D-015 (before Stripe live) | No. vibesales.tech readiness scored 50/62 because the site renders in the browser (18 Oct). D-058 isn't in main's commit log. Whether a Stripe test account exists is unknown: the 8 Oct box signup failed, and Victor was asked to create one in his own browser. | — | missing | high | D-058, then a D-054 rerun on hosted in Stripe test mode: listing → match → tracked click → test checkout → pending commission → refund reversal. Never live keys. |
| 21 | V13: open source, Apache-2.0 | D-099 (devil's advocate) → D-100 (secret scan + split) | No `LICENSE`, no public repo. D-100's output file (`research/offer-rails-open-source-20261021.md`) is outside this repo, and its status is unknown. | Public copy makes no open-source claim (0 hits), which is correct. | missing (expected) | med | Follow the existing gate: D-099 → D-100 full-history scan → split plan → Victor's OK → first push to `vickhunter/offer-rails`. |
| 22 | V13: zero budget, hosting | D-045 Render credit (Victor), Neon free | Render free: cold starts (vision-gap 18 Oct row 20). Neon's 10-snapshot cap blocked deploys on 19, 20 and 21 Oct (roadmap). | — | improve | med | Before each deploy, Product Lead bundles the snapshot-delete ask with the deploy ask (one message to Victor). Paid Render only with Victor. |
| 23 | V14: one owner, nothing missed, paid models | D-096 (this), D-099 | This file is one model's evidence pass. D-096 acceptance asks for Opus 5.5, Sonnet 5.5 and Gemini 4 Argon restatements merged into `briefs/offer-rails-vision-v2-20261021.md`. | — | improve | med | Oracle decides whether this single pass plus the existing 18 Oct Opus/Gemini reviews count, or whether Sonnet 5.5 and Gemini 4 Argon rerun on this file now that the paid models are back. |
| 24 | V15: visuals, examples, video, copy | D-098 (PR #72 draft), D-094 docs site, D-087 /compare, launch drafts in `launch/channels.md` (Victor OK'd the directory copy v2 on 21 Oct, on hold until D-099) | Draft only. | — | improve | low | Launch & Distribution / Motion work. **Not a blocker for protocol v0.1.** It starts when D-092 v0.1 and D-099 are done. |

## 4. Coverage of the topics Oracle named

**Agent-to-agent protocol, any combination of digital spaces.** Rows 1–4 and 10. Today: one deal type (affiliate), one transport (REST + MCP), no negotiation. The two drafts (#68, #71) are the raw material. The open decision is which draft becomes the base. My proposal is #68 for its spec, conformance and adapters, with #71's signing and SDK folded in. Oracle or Victor confirms.

**Human-in-the-loop guardrails.** Rows 5–6. This is the strongest area on main. The open items are the hosted deploy of D-093b and a written exit condition for "publish asks by default", since Victor's 18 Oct preference is automated by default.

**PII and privacy by default.** Rows 3 and 7. Click privacy is in code. Card-level privacy (allowlists, k=50 segments, the `pii.*` conformance tests) is spec only. The `agent-accounts` email field is the one personal-data input left in the agent path.

**No account required where possible.** Rows 8–9. An agent can join with `{}`. A human needs Google only to approve and to reach the owner dashboard. In-flow suggest without signup is the missing piece.

**Thin layer that still works.** Rows 3 and 11. Link, join, `/mcp` and `llms.txt` work with no adapter. Missing machine entry points: `openapi.json` and the A2A agent card.

**Marketplace and distribution.** Rows 12–17.
- Grok Bot MCP marketplace: row 13.
- Official MCP registry: row 14.
- Anthropic and ChatGPT ecosystems: row 15.
- Manufact: row 16.
- npm: row 17.
Prerequisites shared by all of them: one public name (row 2), approved legal pages, a working tested client install, and for some directories a public repo (row 21). Every submission is Victor's.

**Dogfooding on victormotricala.it and vibesales.tech (Stripe test mode).** Rows 19–20. Both are blocked by product gaps (Italian matching, readiness for browser-rendered sites), not by policy.

**"Monetize this content" suggestion in the flow.** Row 9. Not built, and called "fundamental".

**Visuals, examples, video, copy.** Row 24. Listed under **Launch & Distribution / Motion**, after protocol v0.1. Not a blocker.

## 5. Proposed build queue (for Product Lead; a proposal, not a decision)

1. **D-093b deploy (Victor gate).** The merge is already done: `27e0a63`, merged by Victor 07:05 UTC 23 Oct. Left to do: a Neon snapshot, hand-applying migration 047 (`AUTO_MIGRATE` stays off), the deploy, and the Site Keeper D-093b acceptance on hosted.
2. **D-096 done.** Oracle accepts this evidence file, or asks for the Sonnet 5.5 and Gemini 4 Argon reruns (row 23). Product Lead turns rows 1–24 into roadmap rows.
3. **D-092 protocol v0.1.** Merge the drafts into one spec (row 1). Include the universal link proof (row 3), the negotiation minimum (row 4), privacy conformance (row 7), the suggest shape for D-097 (row 9), `openapi.json` and the agent card (row 11), and one public name (row 2, needs Victor).
4. **D-099 devil's advocate.** At least three strong models play hostile roles, and one of them joins hosted cold. It blocks calling D-092 done.
5. **D-100 open-source prep.** Apache-2.0, full-history secret scan, public/private split, README limited to "works today vs in testing". Public `vickhunter/offer-rails` only after a clean scan and Victor's OK.
6. **D-095 adapters, as optional extras.** X, then newsletter, YouTube, Instagram and TikTok, each against the D-092 conformance tests. They're never a prerequisite for the link.
7. **Marketplace submissions, only with Victor's OK.** Registry `server.json` (row 14) → Grok Bot (row 13) → Anthropic and ChatGPT (row 15) → Manufact if it's useful (row 16) → npm (row 17).

Not in the requested order, but needed for the vision. Product Lead decides where they go:
- D-097 in-flow suggest (row 9). Proposal: right after D-092 fixes the `suggest` schema, alongside D-099. It's the most visible part of Victor's 21 Oct and 23 Oct asks.
- D-061 Italian matching and D-058 readiness for browser-rendered sites (rows 19–20). Proposal: before any dogfood run, and they can run in parallel with D-092 because they don't touch the spec.
- D-059 directory back end (row 18), after D-092 v0.1.

## 6. Only Victor can approve

| Action | State today |
|---|---|
| Merge #73 (D-093b) | **Done by Victor**, 07:05:18 UTC 23 Oct (`27e0a63`). |
| Hosted deploy of `27e0a63` + migration 047 (and any Neon snapshot delete that needs) | Pending. |
| `CLAIMS_CHECK` flip to `real` on hosted | Pending. Gated on D-093b being live (roadmap 21 Oct 13:26). |
| Flipping new-site `placement_publish` back to auto (row 6) | Not asked yet. |
| npm publish of `offer-rails` | Not asked. Package staging is D-067. |
| Directory and marketplace submissions (official MCP registry on his GitHub login, Grok Bot, Anthropic, ChatGPT, Manufact, Smithery, Glama, mcp.so) | None submitted. |
| Public repo `vickhunter/offer-rails` first push | After D-099 + D-100. License already chosen: Apache-2.0. |
| `LEGAL_PAGES_APPROVED` (privacy and terms out of DRAFT) | Pending. A connector prerequisite. |
| Paid Render plan / D-045 credit | Pending. |
| Anything on the live blog victormotricala.it (plugin swap, key rotation, Tophost firewall ticket) | Pending. |
| Public protocol name ("Offer Rails Protocol", "DCP", "OfferAI"?) | Open since 14 Oct (protocol doc §12 Q1). |
| Launch posts (he posts; bots never post) | Directory copy v2 OK'd on 21 Oct, on hold until D-099. |

## 7. Disagreements and unknowns

Disagreements with the inputs:
- **PR #73 is merged, not an open draft.** The task said "draft, mergeable but not merged". GitHub shows it merged at 07:05:18 UTC on 23 Oct by `vickhunter`, with `isDraft: false`. That's why queue step 1 is now "deploy" only.
- **The D-096 roadmap acceptance doesn't match this run.** The roadmap asks for three model restatements merged into `briefs/offer-rails-vision-v2-20261021.md`. This task asks for one evidence file at `docs/briefs/offer-rails-vision-gap-2026-10-23.md`. I followed the task. Oracle decides whether a rerun is needed.
- **Default approval.** Victor said on 18 Oct "automated unless the user explicitly wants to be in the loop". Main now defaults publish to ask (D-088, D-093b). The roadmap records that as a deliberate safety step before the claims check is real. It still needs his explicit confirmation and an exit condition (row 6).
- **Three protocol drafts, not one.** #71 calls itself "one of three independent drafts", but the Sonnet branch has no spec content and no PR. So in practice there are two drafts.

Unknowns (no evidence found):
- The hosted `CLAIMS_CHECK` value today. The last record is `stub` (21 Oct).
- Whether Site Keeper's 21 Oct 13:26 hosted re-run (GAP-01 + join) passed.
- Whether a Stripe test-mode account exists for the vibesales.tech run.
- Which plugin version runs on victormotricala.it today, and whether it still calls the Railway backend.
- Whether Tophost still returns 406 to AI crawlers.
- Whether `/makers` and `/publishers` show real listings (browser-rendered; not inspected).
- What `/protocol` says (browser-rendered; not inspected).
- The Manufact self-check score.
- Whether "OfferAI" in the 23 Oct ask is a new name or just a slip of the tongue.
- What the two protocol-like servers in the official registry are (`unified-offer-protocol`, `standing-offer-envelopes-offline-agent-deals`), and whether they compete. That's a Competitor Scout check.
- The status of the D-100 output file and of `research/offer-rails-victor-asks-ledger.md`. Both live outside this repo and weren't provided.

## Sources

Uploaded to this run (copies of the shared business folder):
- `decisions.md` (Victor's decisions up to 21 Oct 18:56)
- `roadmap.md` (P-00, D-010 to D-100)
- `briefs/offer-rails-vision-gap.md` (18 Oct brief, UI walk, Opus 5.5 and Gemini 4 Argon reviews, merged short list)
- `design/offer-rails-protocol.md` (v0.0, 14 Oct)
- `design/dcp-v0.2.md` (D-022, 8 Oct)
- `design/offer-rails-agent-channels.md` (registry and directory requirements, 9–11 Oct)

Repo `vickhunter/affilio` at `27e0a63`: `STATUS.md`, `affilio-api/internal/config/config.go`, `internal/dcp/*.go` (tool names), `internal/dcp/mcp.go`, `internal/handlers/hosted_mcp.go`, `internal/handlers/hosted_auth.go`, `internal/handlers/go_privacy.go`, `internal/handlers/agent_entry.go`, `internal/handlers/agent_docs.go`, `internal/entry/listing.go`, `internal/migrations/`, `cli/package.json`, `packages/distributoor-setup/package.json`.

GitHub (read-only `gh`): PRs #68, #71, #72 and #73; branches `cursor/protocol-v03-gemini`, `cursor/protocol-v03-opus` and `cursor/protocol-v03-sonnet`.

Hosted (read-only GET, 23 Oct 07:0x UTC): `/health`, `/llms.txt`, `/.well-known/dcp.json`, public pages listed in §2. npm registry (`npm view offer-rails`). Official MCP registry (`/v0/servers?search=offer`). `github.com/vickhunter/offer-rails`.
