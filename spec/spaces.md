# Offer Rails Protocol — The `Space` Abstraction, Universal Links & Zero-Click Receipts

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. The `Space` Abstraction

A **`Space`** is any digital surface where attention, distribution, or aggregate insight can be shared under human-set constraints.

Crucially, **a `Space` does not require a platform integration or plugin to participate in Offer Rails Protocol**. Any surface where a human or agent can paste a tracked link (`/v1/go/:token`) or produce a verifiable receipt (`proof_url`, `public_check`, or `counterparty_attestation`) is a first-class `Space`.

### 1.1 Privacy-Preserving `Space` Descriptor

Note that `principal_id` is a pseudonymous identifier (`orp:party:...`) and `audience_card` contains only aggregate $k \ge 50$ statistics:

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

---

## 2. Canonical `Space` Kinds & How Each Works Without or With a Platform Adapter

| `kind` | Platforms (Examples) | Universal Adapter (No Plugin Needed) | Optional `SpaceAdapter` Helper |
|---|---|---|---|
| `content_inline` | WordPress, Ghost, Substack, Medium, any blog | Paste universal `tracking_url` (`/v1/go/:token`) inline + disclosure; verify via `public_check` on `proof_url` | Auto-inserts paragraph & `rel="sponsored"` via WP/Ghost API |
| `content_box` | Blogs, docs, resource directories | Paste callout snippet with `tracking_url`; verify via `public_check` | Renders native Gutenberg / widget block |
| `social_post` | X, LinkedIn, Bluesky, Facebook, Reddit, Instagram | Post copy with `tracking_url` or `tracking_token` + `#ad`; submit post `proof_url` | Publishes via X/LinkedIn API and polls status |
| `social_repost` | X (Repost/Quote), TikTok (Stitch/Duet), Reels | **Zero-click:** Execute repost/quote; submit `proof_url` verified by `public_check` (oEmbed) or `counterparty_attestation` | Triggers repost via platform API |
| `social_comment` | X reply plug, YouTube pinned comment, Reddit stickied comment | Post comment with `tracking_url` or mention; submit comment `proof_url` | Posts & pins comment via platform API |
| `newsletter_slot` | Substack, Beehiiv, Kit, Mailchimp, raw email | Include `tracking_url` in newsletter issue; verify via web archive `proof_url` or `counterparty_attestation` | Injects slot into draft via Beehiiv/Kit API |
| `video_segment` | YouTube, TikTok, Instagram, Podcast notes | Put `tracking_url` in description/show notes or verify host-read via `proof_url` transcript / `counterparty_attestation` | Checks YouTube Data API `paidProductPlacement` & description |
| `community_pin` | Discord, Slack, Telegram, Circle | Post `tracking_url` in channel; verify via `counterparty_attestation` or webhook receipt | Discord/Slack bot posts and pins message |
| `cross_promo_slot` | Partner widgets, post-checkout pages, newsletter referrals | Reciprocal `tracking_url` links tracking balanced visits (`traffic_swap`) | Embeddable widget script |
| `data_insight_room` | Clean-room benchmarks, aggregate research | Exchange aggregate $k \ge 50$ JSON payload; verify via `counterparty_attestation` + payload hash | Automated clean-room query connector |

---

## 3. Universal Link Tracking vs. Zero-Click Receipt Verification

### 3.1 Universal Tracked Link (`/v1/go/:token`)
When a deal enters `EXECUTING`, the Relay or Offeror mints:
- `tracking_url`: `https://<host>/v1/go/<token>`
- `tracking_token`: `orp_tok_<token>` (for surfaces where URLs are stripped or spoken, such as promo codes or podcast shoutouts).

**Crucial Invariant:** Unlike legacy `affilio-api` v0.2 (which gated `/v1/go/<id>` on a WordPress-only `plugin-ack` call and restricted `surface_url` to a single registered blog domain), **an ORP v0.3 tracked link activates as soon as a valid `receipt` of ANY verification mode (`proof_url`, `public_check`, `counterparty_attestation`, or `adapter_ack`) is recorded**, regardless of which platform or domain hosts the placement.

### 3.2 Three Receipt Verification Modes for Any Action (Including Zero-Click)

Every `receipt` message specifies `verification_mode`:

1. **`proof_url` (Universal Public Proof):**
   - The executing agent submits the public URL where the placement or action lives (e.g., `https://x.com/alice/status/184800001`, `https://youtube.com/watch?v=abc`, `https://newsletter.example.com/p/issue-12`).
2. **`public_check` (Automated Unauthenticated Verification):**
   - The Relay or counterparty fetches `proof_url` (via standard HTTP GET, OpenGraph, or public oEmbed endpoint) and confirms:
     - HTTP 200 status,
     - Presence of `tracking_url`, `tracking_token`, or agreed `target_post_uri` / brand mention, AND
     - Presence of the required disclosure marker (`#ad`, `Sponsored`, `rel="sponsored"`, etc.).
3. **`counterparty_attestation` (Bilateral Attestation for Non-Public Surfaces):**
   - When the action occurs in a private or non-scrapable space (e.g., an email-only newsletter with no public web archive, a private Discord server, a spoken podcast read, or an Instagram Story), the counterparty agent or owner inspects the proof (such as a forwarded seed email or screenshot/audio hash) and signs/submits a `counterparty_attestation` confirming fulfillment.
4. **`adapter_ack` (Optional Platform Plugin Attestation):**
   - When a native plugin (like the WordPress plugin) or platform OAuth adapter is installed, it may emit an automated `adapter_ack` receipt.

---

## 4. Settlement Modes

ORP v0.3 supports 8 consideration models across all 10 space kinds:
1. **`cpa_commission`** — Percentage (`commission_bps`) or flat bounty (`flat_amount_minor`) per verified conversion attributed via the universal `tracking_url` or `tracking_token`.
2. **`rev_share`** — Recurring revenue share (`share_bps`) over `duration_months`.
3. **`flat_fee`** — Fixed fee (`amount_minor`) unlocked upon verified `receipt` + survival past `min_live_seconds`.
4. **`space_swap`** — Bilateral space-for-space exchange ($S_A \leftrightarrow S_B$), settled when both parties record verified `receipt` messages within `sync_window_seconds`.
5. **`traffic_swap`** — Reciprocal human visit exchange measured via deduplicated universal `tracking_url` redirects.
6. **`repost_bounty`** — Fixed bounty for a zero-click `social_repost` or `social_comment` verified via `proof_url` + `public_check` after `min_live_seconds`.
7. **`co_marketing`** — Joint partnership deliverable with multi-party receipts.
8. **`data_exchange`** — Exchange of $k$-anonymous ($k \ge 50$) aggregate market/audience insights.
