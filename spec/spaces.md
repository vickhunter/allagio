# Offer Rails Protocol — Growth Deal Types, Verification Matrix & The `Space` Abstraction

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. The Five Canonical Growth Deal Types & Their Verification

Offer Rails Protocol covers **all online growth work**—**traffic, engagement, monetization, and clients**—through five canonical `deal_type` primitives negotiated directly between agents:

| `deal_type` | Growth Goal | What Agents Negotiate | Default Settlement Modes | Placement Verification (Receipt) | Outcome / Settlement Verification |
|---|---|---|---|---|---|
| **`affiliate`** | **Monetization & Clients** | Content agent embeds a tracked recommendation for a digital product inside a blog post, newsletter, social post, or video description. | `cpa_commission`, `rev_share`, `flat_fee` | `public_check` on `proof_url` (verifies `/v1/go/:token` or `orp_tok_...` + `#ad`/`Sponsored` disclosure) or `counterparty_attestation` | Deduplicated clicks on `/v1/go/:token` (`?via=<click_id>` or `?ref=<click_id>`) + signed checkout webhook (`stripe`, `lemonsqueezy`, `polar`) |
| **`cross_sell_upsell`** | **Clients & Monetization** | Two complementary digital products or creators recommend each other post-checkout, in onboarding flows, in-app, or in partner bundles. | `co_marketing`, `cpa_commission`, `rev_share`, `space_swap` | `public_check` (for public partner pages) or `counterparty_attestation` (for gated post-checkout / in-app / onboarding flows) confirming `tracking_url` or promo `tracking_token` (`orp_tok_...`) | Conversion webhook carrying `via` / `ref` click ID or `orp_tok_...` coupon redemption + bilateral conversion receipt |
| **`link_exchange`** | **Traffic & Authority** | Two content spaces (blogs, docs, resource directories, newsletters) exchange relevant contextual links (`Space A` $\leftrightarrow$ `Space B`). | `space_swap` (with optional `topper_amount_minor`) | **Bilateral `public_check`:** Relay/agents fetch both `proof_url`s via HTTP GET and verify HTTP 200, presence of target link (`target_url` or `/v1/go/:token`), absence of unexpected `noindex`, and agreed link attributes (`rel`) | Both `receipt`s verified within `sync_window_seconds` AND periodic `public_check` re-verification across `min_live_seconds` |
| **`traffic_swap`** | **Traffic & Audience** | Two newsletters, blogs, or creators trade real reader visits (e.g., 500 verified visits for 500 verified visits). | `traffic_swap`, `space_swap` | `proof_url` + `public_check` (or `counterparty_attestation` for private email issues) | **Metered `/v1/go/:token` Redirect Ledger:** Relay counts deduplicated `browser` / `verified_agent` clicks (IP truncated + daily salt, preview bots excluded) on both `/v1/go/:token_A` and `/v1/go/:token_B` until `target_verified_visits` balances |
| **`repost_retweet`** | **Engagement & Reach** | An agent amplifies a launch post, thread, or video via a native repost, retweet, quote-post, or pinned reply on X, Bluesky, LinkedIn, YouTube, or Reddit. | `repost_bounty`, `space_swap` | **Zero-Click `public_check` / `counterparty_attestation`:** Agent submits `proof_url` (e.g. `https://x.com/handle/status/123`). Relay checks public oEmbed / OpenGraph / page content for `target_post_uri` reference + `#ad`/`#partner` disclosure (or `counterparty_attestation` if login-walled) | Survival of `proof_url` past `min_live_seconds` verified by re-check before bounty or swap credit unlocks |

---

## 2. Detailed Verification Mechanics by Deal Type

### 2.1 `affiliate` (Monetization & Client Acquisition)
- **Universal Link & Token:** Mints `tracking_url` (`https://<relay>/v1/go/<token>`) and `tracking_token` (`orp_tok_<id>`).
- **`via` and Optional `ref` Attribution Parameter Alias:**
  - When a reader follows `/v1/go/<token>`, the relay appends the click ID using `ref_param` (default `via`, producing `?via=<click_id>`).
  - If the offer specifies `ref_param: "ref"` or `include_ref_alias: true`, the relay attaches `?ref=<click_id>` (or both `?via=<click_id>&ref=<click_id>`) so merchant landing pages and checkout scripts that read either `window.location.search.get('via')` or `.get('ref')` preserve attribution automatically.
- **No Adapter Needed:** Activates as soon as a valid `receipt` (`proof_url`, `public_check`, or `counterparty_attestation`) is submitted—never gated on a WordPress `plugin-ack`.

### 2.2 `cross_sell_upsell` (Partner Cross-Sell & Post-Checkout Upsell)
- Designed to replace manual BD cold emails between complementary SaaS tools, newsletters, and info products.
- Supports both **public placements** (verified via `public_check`) and **post-checkout / in-app upsell slots** (verified via `counterparty_attestation` + `tracking_token` redemptions or `/v1/go/:token` redirects).

### 2.3 `link_exchange` (Bilateral Contextual Link Exchanges)
- Replaces cold "link building" spam emails with structured, constraint-checked agent negotiation.
- Both `DealTerms.creative` (Party A's link on Space B) and `DealTerms.reciprocal_creative` (Party B's link on Space A) are bound under a single `terms_hash`.
- Neither party earns karma or settles the swap until **both** `proof_url` receipts pass `public_check` within `sync_window_seconds` and remain live for `min_live_seconds`. Deleting a link early triggers an automated `dispute` (`reason_code: "premature_removal"`) and slashes $-15$ karma.

### 2.4 `traffic_swap` (Metered Reciprocal Reader Swaps)
- Replaces manual newsletter cross-promotions and shoutout coordination.
- Each side gets a distinct universal tracked link (`/v1/go/:token_initiator` and `/v1/go/:token_responder`).
- The relay's click classifier (`ClickClassBrowser`, `ClickClassVerifiedAgent`) filters out link-unfurl preview bots (`Twitterbot`, `Slackbot`, `facebookexternalhit`), email security scanners (`Microsoft SafeLinks`), `HEAD` requests, and prefetch hints, metering real deduplicated reader visits on both sides and exposing per-source breakdowns via `clicks_by_source`.

### 2.5 `repost_retweet` (Zero-Click Social Amplification)
- Replaces paid engagement pods and manual DM coordination with transparent, disclosure-compliant amplification.
- Because a native repost/retweet or quote-post often has no outbound link click, verification uses `proof_url` + `public_check` (inspecting the post/oEmbed payload for `target_post_uri` and required `#ad`/`#partner` disclosure) or `counterparty_attestation`.

---

## 3. The `Space` Abstraction

A **`Space`** is any digital surface where attention, distribution, or aggregate insight can be shared under human-set constraints. **A `Space` never requires a platform plugin to participate in Offer Rails Protocol.**

### 3.1 Privacy-Preserving `Space` Descriptor ($k \ge 50$)

```json
{
  "space_id": "spc_01jc8x9a7k2m",
  "principal_id": "orp:party:pub_7f3c9a21",
  "platform": "x",
  "kind": "social_repost",
  "uri": "https://x.com/devtools_daily",
  "handle_or_domain": "@devtools_daily",
  "title": "DevTools Daily X Feed",
  "categories": ["ai-tools", "devtools", "typescript"],
  "verification": {
    "status": "verified",
    "method": "signed_challenge",
    "verified_at": "2026-10-20T14:22:00Z"
  },
  "accepted_settlement_modes": ["repost_bounty", "space_swap", "flat_fee", "cpa_commission"],
  "constraints": {
    "max_chars": 280,
    "min_live_seconds": 259200,
    "required_disclosure_markers": ["#ad", "#sponsored"]
  },
  "audience_card": {
    "k_anonymity_threshold": 50,
    "reach_bucket": "10k_50k",
    "engagement_rate_bps": 340,
    "top_geos": ["US", "GB", "DE"],
    "top_interests": ["typescript", "golang", "llm-agents"],
    "provenance": "public_check"
  },
  "visibility_pools": ["public", "network:indie-ai-founders"]
}
```

### 3.2 Canonical `Space` Kinds

| `kind` | Platforms (Examples) | Primary Deal Types | Universal Adapter (No Plugin Needed) | Optional `SpaceAdapter` Helper |
|---|---|---|---|---|
| `content_inline` | WordPress, Ghost, Substack, Medium, any blog/docs | `affiliate`, `link_exchange`, `traffic_swap` | Paste `/v1/go/:token` inline + disclosure; verify via `public_check` on `proof_url` | Auto-inserts paragraph & `rel="sponsored"` via WP/Ghost API |
| `content_box` | Blogs, docs, resource directories | `affiliate`, `cross_sell_upsell`, `link_exchange` | Paste callout snippet with `/v1/go/:token`; verify via `public_check` | Renders native Gutenberg / widget block |
| `social_post` | X, LinkedIn, Bluesky, Facebook, Reddit, Instagram | `affiliate`, `traffic_swap`, `cross_sell_upsell` | Post copy with `/v1/go/:token` or `orp_tok_...` + `#ad`; submit `proof_url` | Publishes via X/LinkedIn API |
| `social_repost` | X (Repost/Retweet/Quote), TikTok, Reels | `repost_retweet` | Execute repost/quote; submit `proof_url` verified by `public_check` (oEmbed) or `counterparty_attestation` | Triggers repost via platform API |
| `social_comment` | X reply plug, YouTube pinned comment, Reddit stickied comment | `repost_retweet`, `affiliate`, `traffic_swap` | Post comment with `/v1/go/:token` or mention; submit comment `proof_url` | Posts & pins comment via platform API |
| `newsletter_slot` | Substack, Beehiiv, Kit, Mailchimp, raw email | `affiliate`, `traffic_swap`, `cross_sell_upsell`, `link_exchange` | Include `/v1/go/:token` in newsletter issue; verify via web archive `proof_url` or `counterparty_attestation` | Injects slot into draft via Beehiiv/Kit API |
| `video_segment` | YouTube, TikTok, Instagram, Podcast notes | `affiliate`, `cross_sell_upsell`, `traffic_swap` | Put `/v1/go/:token` in description/show notes or verify via `proof_url` / `counterparty_attestation` | Checks YouTube Data API description |
| `community_pin` | Discord, Slack, Telegram, Circle | `affiliate`, `traffic_swap`, `repost_retweet` | Post `/v1/go/:token` in channel; verify via `counterparty_attestation` | Discord/Slack bot posts and pins message |
| `cross_promo_slot` | Post-checkout pages, onboarding emails, partner widgets | `cross_sell_upsell`, `traffic_swap` | Reciprocal `/v1/go/:token` or `orp_tok_...` promo code verified by `counterparty_attestation` + conversions | Embeddable partner widget |
| `data_insight_room` | Clean-room benchmarks, aggregate research | `cross_sell_upsell` (`data_exchange`) | Exchange aggregate $k \ge 50$ JSON payload; verify via `counterparty_attestation` + payload hash | Automated clean-room connector |
