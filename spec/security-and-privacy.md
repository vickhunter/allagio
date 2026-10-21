# Offer Rails Protocol — Security, Trust, Reputation & Privacy

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Privacy Architecture: Zero Personal Data Between Agents

Privacy in Offer Rails Protocol is enforced structurally at the schema and transport layers:

1. **Pseudonymous Party Identifiers (`orp:party:<id>` or `orp:principal:<id>`):**
   - Agents never exchange owner email addresses, legal names, street addresses, phone numbers, or payment account numbers.
   - All messages address parties via pseudonymous URIs (`orp:party:pub_7f3c9a21`).
2. **Aggregate Statistics Only ($k \ge 50$):**
   - `AudienceCard` objects carry only coarse reach buckets (`10k_50k`), engagement basis points, and top countries/topics where the underlying cohort has at least **50 distinct individuals** (`k_anonymity_threshold >= 50`).
   - Any payload containing raw user emails, IP addresses, device IDs, or cookies is rejected by the JSON Schema and policy validators.
3. **Minimal Progressive Disclosure:**
   - During `discover`, a party MAY omit the exact placement sub-path and share only the category, reach bucket, and verified reputation summary, disclosing the specific `proof_url` only upon `offer`/`accept`/`receipt`.
4. **Click & Redirect Anonymization (`/v1/go/:token`):**
   - Visitor IPs on universal tracked links MUST be truncated (`/24` IPv4, `/48` IPv6) and hashed with a daily rotating salt (`00:00 UTC`). Raw visitor IPs and raw `User-Agent` strings are never exposed to either agent.

---

## 2. Security, Verification & Constraint Escalation

### 2.1 Public Surface Verification (Zero-Plugin Default)
A space owner can verify control of a domain or social account without installing any software:
- **DNS TXT or `.well-known` (`dns_txt` / `http_well_known`):** `/.well-known/orp-verify.txt` or `/.well-known/offer-rails.txt`.
- **Public Bio / Post Token (`signed_challenge`):** Placing `orp-verify:<token>` in an X bio, YouTube channel description, or newsletter header, checked via unauthenticated HTTP/oEmbed (`public_check`).
- **Optional OAuth / Plugin Key (`oauth2_handle` / `plugin_key`):** Used when an optional platform adapter is connected.

### 2.2 Canonical `terms_hash` & Consent Binding
- `terms_hash` is `"sha256:" + hex(SHA-256(RFC8785_JCS(terms)))`.
- Every `accept`, `consent`, and `execute` message MUST reference the exact `terms_hash`. Modifying any field in `terms` (price, target URL, copy, or disclosure) invalidates prior consent and triggers a fresh constraint evaluation.

### 2.3 Replay & Prompt-Injection Protection
- State-mutating requests require a unique `idempotency_key` and monotonic `seq` per `deal_id`.
- Free-text fields (`headline`, `body`, `reason`) MUST be sanitized to strip HTML `<script>`/`<iframe>`, `javascript:` URIs, and invisible bidirectional control characters.

---

## 3. Reputation System: "Proof-of-Outcome Karma"

ORP v0.3 includes a Reddit-inspired karma system that accumulates across deals while resisting agent wash-trading and Sybil farms:

1. **Receipt & Hold-Window Gate:** Karma is minted **only** after a deal reaches `RECEIPTED` (verified via `public_check`, `counterparty_attestation`, or `adapter_ack`) AND survives `min_live_seconds` (or settles a verified conversion) without revocation or upheld dispute.
2. **Pairwise Exponential Decay ($2^{-(n-1)}$):** The $n$-th deal between the same two pseudonymous parties $(A, B)$ within 180 days is weighted by $w(n) = 2^{-(n-1)}$. Two colluding agents washing 1,000 deals together can never earn more than $2\times$ the karma of a single deal.
3. **Counterparty Diversity Weighting:** Karma from a newly created party with zero independent counterparties is weighted at $0.1$, scaling linearly to $1.0$ once a party completes verified deals with $\ge 10$ distinct verified counterparties.
4. **Asymmetric Slashing on Disputes:** Fulfilling a deal grants $+5$ `fulfillment_karma`, $+3$ `conversion_karma`, and $+1$ `compliance_karma`. Losing a `dispute` (e.g., deleting a repost early or omitting disclosure) slashes **$-15$ `dispute_penalty`**.
