# Offer Rails Protocol — Optional Platform Adapters (`adapters/`)

## Core Design Rule: The Tracked Link Is the Universal Adapter

In Offer Rails Protocol v0.3, **a deal never requires a platform adapter or plugin to work**.

Like `bit.ly` or an affiliate link, the deal's **Universal Tracked Link** (`/v1/go/:token`) and **Tracking Token** (`orp_tok_...`) work wherever a human or agent pastes them — email, X, Facebook, Instagram, YouTube, blog posts, podcast show notes, Discord, or Telegram. And for zero-click placements (reposts, comments, traffic swaps, space-for-space swaps), the protocol verifies execution through a 3-mode **Verifiable Receipt** (`proof_url`, `public_check`, or `counterparty_attestation`).

### When to Use a `SpaceAdapter`

Platform adapters in this directory are **optional helpers** (`SpaceAdapter` in `./interface.ts`) that agents or relays can load when they want convenience automation:

1. **Inventory discovery (`inspectSpace`)**: Auto-populate `Space` metadata from a profile or publication URL.
2. **Native formatting (`renderPlacement`)**: Format the copy, Universal Tracked Link, and FTC/platform disclosure (`#ad`, `rel="sponsored"`, etc.) for the target surface.
3. **Optional auto-posting (`executePlacement`)**: Publish directly if the user has provided platform API credentials, or gracefully fall back to `mode: "universal_link_ready"` (returning the formatted text + tracked link) when no API credentials exist.
4. **Automated receipt checks (`verifyReceipt`)**: Verify `proof_url`, `public_check` (HTTP / oEmbed), or `counterparty_attestation`.

### Included Worked Adapters

- [`./interface.ts`](./interface.ts) — `SpaceAdapter` TypeScript interface.
- [`./x/index.ts`](./x/index.ts) — X (Twitter) adapter supporting `social_post`, `social_repost`, and `social_comment`, with zero-API fallback (`universal_link_ready`) and oEmbed/`public_check` receipt verification.
- [`./newsletter/index.ts`](./newsletter/index.ts) — Newsletter adapter (Substack, Beehiiv, Kit, Ghost, generic HTML email) supporting `newsletter_slot` and `cross_promo_slot` with `rel="sponsored nofollow"` and relay-level link revocation.

### Running Adapter Tests

```bash
node --experimental-strip-types --test ./adapters/test/adapters.test.ts
```
