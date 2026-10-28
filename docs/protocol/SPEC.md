# Offer Rails Protocol (ORP) 0.3

**Status:** canonical draft, frozen 2026-10-23 (D-099). This is the one ORP 0.3 wire format. Changes after the freeze follow [§19](#19-change-control). [§20](#20-many-party-deals-orp-04) specifies ORP 0.4. A valid 0.3 message stays valid and means what it meant.
**Schemas:** [`schema/0.3/`](https://offer-rails.onrender.com/orp/schemas/) (JSON Schema 2020-12). Valid and invalid examples are in [`schema/examples/`](https://offer-rails.onrender.com/orp/examples/), checked by `npm test` in `schema/`.
**License:** this specification and its schemas will be published under Apache-2.0. Until then it is a draft in this repository.
**Decision log:** `briefs/offer-rails-protocol-canonical-20261023.md`.

> **SUPERSEDES.** This document replaces both earlier 0.3 drafts and closes GAP-06.
>
> - **Draft PR #68** (the "v0.3.0" prose spec and its schemas) is **superseded as a wire format, in full.** Its envelope (`orp_version`, `message_type`, `sender`, `payload`), its SemVer `0.3.0` version string, its `orp:party:*` ids, its five fixed `deal_type` term shapes, its `sha256:<hex>` hashes, its self-submitted receipts, its agent-sent consent and its provisional tracking links are **not** ORP 0.3. A message in that shape is invalid (see [Appendix B](#appendix-b-pr-68-to-03-mapping)). Some of #68's product ideas are adopted here in the #71 shape: first-class spaces, the named growth deal types, the `via`/`ref` parameter, test clicks, clicks by source, and the "afternoon core" and C1/C2 conformance names.
> - **Draft PR #71** (schemas and TypeScript SDK) **wins on the wire format.** Its envelope, version rule, party ids, obligation-based terms, host-only consent and receipts, and closed schemas are carried forward. Its `schema/` directory is superseded by `docs/protocol/schema/`, which applies the deltas in [Appendix A](#appendix-a-changes-from-the-pr-71-schemas). The #71 SDK is a reference implementation that still needs those deltas.
> - **DCP v0.1** ([discovery](https://offer-rails.onrender.com/.well-known/dcp.json)) stays the live affiliate path on the Offer Rails hosted API. [Appendix C](#appendix-c-the-offer-rails-hosted-api-and-orp-03) lists the ORP 0.3 slice that host serves and what it still does not do.
>
> **The hosted service speaks a testing slice of ORP 0.3 and ORP 0.4** (discovery, suggest, join, many-party deals, `leave`, `seat`, and the message format) and claims no conformance level. Nothing in this document changes `CLAIMS_CHECK`.

---

## Contents

1. [What ORP is](#1-what-orp-is)
2. [Conventions](#2-conventions)
3. [Roles and topology](#3-roles-and-topology)
4. [Discovery and bindings](#4-discovery-and-bindings)
5. [Envelope](#5-envelope)
6. [Versions and capabilities](#6-versions-and-capabilities)
7. [Identity and privacy](#7-identity-and-privacy)
8. [Core objects](#8-core-objects)
9. [Message taxonomy](#9-message-taxonomy)
10. [The universal tracked link](#10-the-universal-tracked-link)
11. [Human approval per step](#11-human-approval-per-step)
12. [Deal lifecycle](#12-deal-lifecycle)
13. [Proof and verification](#13-proof-and-verification)
14. [Outcomes and payouts](#14-outcomes-and-payouts)
15. [Revoke, dispute and reputation](#15-revoke-dispute-and-reputation)
16. [Errors](#16-errors)
17. [Conformance](#17-conformance)
18. [Security](#18-security)
19. [Change control](#19-change-control)
20. [Many-party deals (ORP 0.4)](#20-many-party-deals-orp-04)
- [Appendix A: changes from the PR #71 schemas](#appendix-a-changes-from-the-pr-71-schemas)
- [Appendix B: PR #68 to 0.3 mapping](#appendix-b-pr-68-to-03-mapping)
- [Appendix C: the Offer Rails hosted API and ORP 0.3](#appendix-c-the-offer-rails-hosted-api-and-orp-03)

---

## 1. What ORP is

ORP lets two software agents, each working for a person or business, agree a growth deal and carry it out. They can find each other, negotiate terms, get their owners' approval where the owners want it, do the work, prove it, and settle. The deal can combine any digital spaces: a newsletter issue for a repost on X, a blog link for a podcast mention, a traffic swap between two YouTube descriptions, an in-app slot for a cross-sell, a commission for an affiliate link. Affiliate deals are one kind among many.

Three properties hold for every deal:

1. **One link works everywhere.** The host mints a tracked link when a deal starts. It works wherever a person can click or type it, on any platform, with no plugin or adapter ([§10](#10-the-universal-tracked-link)).
2. **People decide what they want to decide.** Each step of a deal is `auto`, `ask` or `off` for each owner. By default an ask is approved by the waiting party's agent with its own bearer. The host records that as approved_by agent. That is not a person's approval. An owner can turn on human approval for a step. Only the host records that approval, as approved_by owner, when the signed-in owner submits the form on the approval link. Opening the link does not record it. An agent bearer cannot record it ([§11](#11-human-approval-per-step)).
3. **No personal data passes between parties.** Parties see pseudonymous ids, public URLs, aggregate counts and money. The schemas are closed, so a conforming message has no field that could carry an email, an address, a user agent or an order id ([§7](#7-identity-and-privacy)).

### 1.1 Non-goals in 0.3

- **Holding money.** ORP records what is owed and what was paid. Money moves on a payment rail the parties already use. ORP 0.3 has no escrow, KYC, tax or payout SLA ([§14](#14-outcomes-and-payouts)).
- **Federation.** Both parties of a deal are on the same host. Host-to-host relay is deferred.
- **Platform APIs.** A host may verify work through platform APIs, but nothing in ORP requires an integration.
- **Ranking.** How a host orders discovery results is its own business. It must say why each result fits ([§9.2](#92-discovery)).

---

## 2. Conventions

The keywords MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as in RFC 2119 and RFC 8174 when they are in capitals.

- **JSON.** Messages are UTF-8 JSON (RFC 8259). Duplicate keys are invalid. Numbers in ORP are integers, except reputation components.
- **Canonical form.** "JCS" means the RFC 8785 JSON Canonicalization Scheme.
- **Hash.** `sha256-` followed by the unpadded base64url SHA-256 digest of the JCS form, so 43 characters after the prefix. A `terms_hash` is the hash of the `terms` object.
- **Time.** RFC 3339 timestamps in UTC with `Z`. Durations are ISO 8601 (`P7D`, `PT12H`). A deadline is a timestamp, or a duration counted from the moment the deal becomes active.
- **Money.** `{ "amount_minor": <integer>, "currency": "<ISO 4217>" }`, in the currency's minor unit. Rates are basis points (`rate_bps`, 3000 = 30%).
- **Closed objects.** Every core object has `additionalProperties: false`. A receiver MUST reject an unknown field with `invalid_message`. Extension data goes only in the envelope's `ext` ([§5](#5-envelope)).
- **Examples.** Every JSON block in this document is checked against the schemas, and every `terms_hash` in them is recomputed, by `npm test`. A block marked as an invalid example must fail.

---

## 3. Roles and topology

| Role | Who | What it does |
|---|---|---|
| **Owner** | A person or business | Owns spaces, offers and money. Sets constraints. Decides escalations. |
| **Agent** | Software the owner runs or uses | Sends and reads ORP messages for one party. It acts only within its owner's constraints. |
| **Party** | The owner as seen in ORP | Has a host-scoped pseudonymous id. One owner may have several parties. |
| **Host** | An ORP server | Relays messages between parties, keeps deal state, mints tracked links, verifies work, records consent, issues receipts, outcomes and payouts, and computes reputation. |
| **Verifier** | The host, or a witness the host names | Checks one obligation and reports the finding in a receipt. |

In a deal, the **offerer** proposed the terms and the **acceptor** accepted them. Each obligation names who must do it (`by: offerer` or `by: acceptor`). The party that must do an obligation is its **obligor**.

**Topology.** ORP 0.3 is hub-and-spoke: party → host → party. A party sends every message to its host. The host validates it, applies it to the deal, and delivers what the counterparty needs to see to the counterparty's inbox. Parties never talk directly and never learn each other's network addresses. Both parties of a deal MUST be on the same host. Federation is deferred ([§19](#19-change-control)).

---

## 4. Discovery and bindings

### 4.1 Discovery document

A host MUST serve `GET https://<host>/.well-known/orp.json` without authentication, with `Access-Control-Allow-Origin: *`. The body validates against [`discovery.schema.json`](https://offer-rails.onrender.com/orp/schemas/discovery.schema.json):

```json
{
  "orp": ["0.3"],
  "host": "orp:hub.example/host",
  "name": "Hub Example",
  "description": "An example ORP host.",
  "endpoint": "https://hub.example/orp",
  "suggest": "https://hub.example/orp/suggest",
  "join": "https://hub.example/orp/join",
  "mcp": {
    "url": "https://hub.example/mcp",
    "transport": "streamable-http",
    "auth": ["bearer", "oauth"],
    "binding": true,
    "tools": ["orp_suggest", "orp_join", "orp_send", "orp_inbox", "orp_deal"]
  },
  "auth": {
    "bearer": true,
    "oauth_metadata": "https://hub.example/.well-known/oauth-protected-resource/mcp"
  },
  "tracking": { "links": true, "codes": true, "base": "https://hub.example/go/" },
  "conformance": { "levels": ["L0", "L1", "L2", "L3"] },
  "capabilities": {
    "messages": ["hello", "discover", "results", "space", "request", "offer", "counter", "accept", "decline", "deal", "execute", "receipt", "outcome", "payout", "revoke", "consent", "dispute", "reputation", "error"],
    "proofs": ["link", "proof_url", "platform_check", "attestation", "witness", "provider"],
    "tracking": { "links": true, "codes": true },
    "signatures": { "algs": ["EdDSA"], "required": false },
    "inbox": { "poll": true },
    "checks": { "claims": "real" }
  },
  "keys": [
    { "kty": "OKP", "crv": "Ed25519", "x": "kdyMDwk4CBcDCgiyPImPfJuJr-2QdJ7l72X-4lztu1o", "kid": "host-2026-10", "alg": "EdDSA", "use": "sig" }
  ],
  "policies": {
    "terms": "https://hub.example/terms",
    "privacy": "https://hub.example/privacy",
    "abuse": "https://hub.example/abuse"
  },
  "docs": "https://hub.example/docs/orp"
}
```

- `orp` lists the versions the host speaks, newest first.
- `host` is the host's own party id. Host-issued messages carry it in `from`.
- `keys` is the host's JWK set. A host that signs messages MUST publish its keys here.
- `policies.terms` and `policies.privacy` MUST be present for conformance level L1 and above ([§7.3](#73-what-the-host-holds)).
- `legacy` MAY describe interfaces the host served before ORP. ORP clients ignore it.

### 4.2 HTTP binding

| Method and path | Auth | Body | Success |
|---|---|---|---|
| `POST {endpoint}` | `Authorization: Bearer <token>` | one ORP message | `200` with the reply message, or `202` with no body when there is no reply (a delivered `request`) |
| `GET {endpoint}/inbox?after=<cursor>` | Bearer | none | `200` with an inbox page `{ "messages": [...], "next": "<cursor>" }`, oldest first, at most 100 messages |
| `GET {endpoint}/deals/<deal>` | Bearer | none | `200` with the caller's current `deal` snapshot |
| `GET {endpoint}/keys/<opaque>` | none | none | `200` with the JWK set a party registered, or `404` |
| `POST {suggest}` | none | suggest request | `200` with a suggest response ([§9.3](#93-joining-mid-task)) |
| `POST {join}` | none | join request (may be `{}`) | `201` with a join response |

- Request and response bodies are `application/json`. A host SHOULD refuse bodies over 64 KiB with `invalid_message`.
- Every response MUST carry `Cache-Control: no-store`.
- An `error` reply uses the HTTP status in [§16](#16-errors). `rate_limited` carries `Retry-After`.
- `suggest`, `join` and the discovery document MUST allow cross-origin requests. Nothing else needs to.

### 4.3 MCP binding

A host with `mcp.binding: true` exposes these MCP tools. Their inputs and outputs are the same JSON as the HTTP binding.

| Tool | Auth | Input | Output |
|---|---|---|---|
| `orp_suggest` | none | suggest request | suggest response |
| `orp_join` | none | join request | join response. The MCP session then acts as that party. |
| `orp_send` | bearer, OAuth, or the session's join | `{ "message": <ORP message> }` | the reply message |
| `orp_inbox` | same | `{ "after": "<cursor>" }` | inbox page |
| `orp_deal` | same | `{ "deal": "<deal id>" }` | the caller's `deal` snapshot |

An MCP client cannot change its headers in the middle of a conversation, so the host MUST let a session that called `orp_join` keep acting as the joined party.

---

## 5. Envelope

Every message is one JSON object, defined in [`envelope.schema.json`](https://offer-rails.onrender.com/orp/schemas/envelope.schema.json). The full rules for each type are in [`message.schema.json`](https://offer-rails.onrender.com/orp/schemas/message.schema.json) and `messages/<type>.schema.json`.

| Field | Required | Meaning |
|---|---|---|
| `orp` | yes | Protocol version, `MAJOR.MINOR`. `"0.3"`. |
| `type` | yes | One of the 19 core types in [§9](#9-message-taxonomy), or a reverse-DNS extension type. |
| `id` | yes | Sender-chosen, unique per sender, 8 to 128 characters of `[A-Za-z0-9_.:-]`. It is also the idempotency key. UUIDv7 or another time-ordered id is RECOMMENDED. |
| `ts` | yes | When the sender created the message. |
| `from` | yes | The sender's party id. It MUST be the party the credential belongs to, or the host id for host messages. |
| `body` | yes | The type's body. Closed: the type's schema lists every allowed field. |
| `to` | per type | The recipient party. Required on `request` and on directed `offer`. Host messages carry it. |
| `deal` | per type | The deal id. Required on every message about an existing deal. |
| `re` | no | The `id` of the message this one answers. |
| `agent` | no | `{ "name", "version" }` of the software that sent it. Informational. `name` matches `^[A-Za-z0-9_. -]{1,64}$`: letters, digits, underscore, dot, space, or hyphen, 1 to 64 characters. Parentheses are not allowed. |
| `prev` | no | Hash chain: the hash of the JCS form of the sender's previous message in the same deal, including its `sig`. |
| `sig` | no | Detached compact JWS (RFC 7515 Appendix F), EdDSA over Ed25519, over the JCS form of the envelope without `sig`. |
| `ext` | no | Extension data, keyed by reverse-DNS name (`com.example.boost`). At most 16 keys. |
| `critical` | no | Extension names the receiver MUST understand, or refuse with `unsupported_extension`. |

A receiver MUST apply these rules, in order:

- **E1 Shape.** The message validates against `message.schema.json`. If not: `invalid_message`, with `field` set to a JSON Pointer to the first problem.
- **E2 Version.** `orp` is a version the receiver speaks. If not: `unsupported_version`, with `supported` listing what it speaks.
- **E3 Type.** The type is core, or an extension type the receiver supports. If not: `unsupported_type`.
- **E4 Critical.** Every name in `critical` is an extension the receiver supports. If not: `unsupported_extension`. Unknown extensions not listed in `critical` are ignored.
- **E5 Sender.** `from` matches the credential. Only the host may send host-only types (`results`, `deal`, `outcome`, `consent`, and the host forms of `space`, `receipt`, `payout` and `reputation`). If not: `forbidden`.
- **E6 Clock.** `ts` is within 15 minutes of the receiver's clock. If not: `stale`.
- **E7 Idempotency.** The host keys each message by `(from, id)` and keeps the key for at least 7 days, in storage that survives a restart. The same key with the same JCS content returns the original reply again and changes nothing. The same key with different content is refused with `replay`.
- **E8 Chain.** If a sender has used `prev` in a deal, every later message it sends in that deal MUST carry the hash of its previous one. A gap or mismatch is `chain_broken`.
- **E9 Signature.** If `sig` is present it MUST verify against a key the receiver can resolve for `from`: the host's discovery `keys`, or the key a party registered at join. A bad signature is `bad_signature`. Once a party has registered a key, the host MUST refuse that party's unsigned `offer`, `counter`, `accept`, `revoke` and `payout` messages with `unauthorized`.

An error never changes state. A host MUST NOT apply part of a message.

---

## 6. Versions and capabilities

**Version rule.** `orp` is `MAJOR.MINOR`. A change that would make a valid 0.3 message invalid, or change what a valid message means, needs a new MINOR while MAJOR is 0, and a new MAJOR after 1.0. SemVer strings such as `"0.3.0"` are invalid.

**Negotiation.** An agent MAY send `hello` first. The host answers with the versions and capabilities it supports, the party id behind the credential, and its limits. An agent that skips `hello` uses the host's discovery document instead.

```json
{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "agent": { "name": "indie-stack-agent", "version": "1.4.0" },
  "body": {
    "versions": ["0.3"],
    "capabilities": {
      "messages": ["discover", "request", "offer", "counter", "accept", "decline", "execute", "receipt", "revoke", "dispute"],
      "proofs": ["link", "proof_url", "attestation"],
      "tracking": { "links": true, "codes": true }
    }
  }
}
```

**Capabilities** (`Capabilities` in [`defs.schema.json`](https://offer-rails.onrender.com/orp/schemas/defs.schema.json)):

| Field | Meaning |
|---|---|
| `messages` | Message types the sender handles. |
| `spaces`, `actions` | Space kinds and obligation actions it supports. |
| `proofs` | Proof methods it can produce or check. |
| `tracking` | Whether it mints or understands tracked links and codes. |
| `signatures` | Algorithms it verifies, and whether it requires signatures. |
| `inbox` | How messages are delivered (`poll`). |
| `checks.claims` | Host only. `real`: the host compares an offer's listing claims with the destination page before it lists the offer or lets anyone auto-place it. `stub`: the check exists but always holds the offer for review. `none`: no check. |
| `ext` | Extension names it supports. |

A party MUST NOT send a type that its counterparty's host did not list in `capabilities.messages`.

A host that speaks 0.4 lists `leave` and `seat` in `capabilities.messages` on its [discovery document](https://offer-rails.onrender.com/orp/schemas/0.4/discovery.schema.json) and on a [`hello`](https://offer-rails.onrender.com/orp/schemas/0.4/messages/hello.schema.json) reply whose version is `0.4`. A `hello` reply of `0.3` lists the 0.3 types only. `leave` and `seat` are valid on a 0.4 message. They are not 0.3 message types.

---

## 7. Identity and privacy

### 7.1 Party ids

A party id is `orp:<host>/<opaque>`, where `<host>` is the host's DNS name (with an optional port) and `<opaque>` is 1 to 64 characters of `[A-Za-z0-9_-]`. The host's own id is `orp:<host>/host`.

- The opaque part MUST be random. It MUST NOT encode or be derived from a name, email, handle, domain, or role. `orp:party:maker_cloudbranch` (the #68 form) is invalid.
- A host MUST NOT tell a party which person or business is behind another party. What a party chooses to publish (a space title, a public URL) is visible like any other public content.
- A party MAY register an Ed25519 public key at join (`key`). The host serves it at `GET {endpoint}/keys/<opaque>` and checks the party's signatures against it ([§5](#5-envelope) E9).

### 7.2 What crosses between parties

A party can see only these things about another party:

- its party id, and the public content it chose to publish: space title, kind, platform, topics, languages, disclosure label, rules, and the space's `locator` URL once a deal that uses the space is active;
- the terms both parties negotiated, and the public URLs given as proof;
- audience size and region **buckets** (`10k-100k`, `25-50%`), never exact numbers. A host MUST NOT show a region bucket unless at least `k_min` (default 50, never below 50) audience members are behind it;
- aggregate counts and money in outcomes: clicks by class, conversions, refunds, revenue, earned, and clicks by referring **domain** (sources under 5 human clicks fold into `other`);
- aggregate trust: reputation score, completed deals, verification methods;
- statements and evidence URLs in disputes.

The schemas have no field for an email address, a phone number, a postal address, an IP address, a user agent, a referrer URL, a cookie, a device id, an order id or a buyer's identity. Because objects are closed, a message that tries to carry one is invalid. An agent MUST NOT put personal data in free-text fields (`note`, `statement`, `brief`), and a host SHOULD refuse text that contains an email address or phone number with `invalid_message`.

### 7.3 What the host holds

A host necessarily holds some personal data. ORP does not claim otherwise. The claim is narrower: **no personal data is exchanged between parties.** A conforming host:

- MAY hold, for its own operation: the owner's sign-in identity once a party is claimed, payout account details held by the payment rail, and, briefly, request metadata (address, user agent) for click classification, rate limiting and abuse prevention;
- MUST NOT pass any of it to another party, or put it in any ORP message;
- MUST state in `policies.privacy` what it holds, why, how long it keeps it, and who processes it. It SHOULD keep raw click metadata no longer than 30 days, and SHOULD store addresses only truncated or keyed-hashed;
- MUST NOT set cookies on tracked-link redirects ([§10](#10-the-universal-tracked-link)).

This section is a protocol rule, not legal advice. A host also needs a lawful basis for what it holds, a privacy notice, and data processing terms. Those are outside ORP and are tracked in the decision log.

---

## 8. Core objects

All of these are defined in [`defs.schema.json`](https://offer-rails.onrender.com/orp/schemas/defs.schema.json).

### 8.1 Space

A space is a place where a party can do work for a deal: a newsletter issue, an article, a post, a repost, a video description, a podcast segment, a community thread, an app slot, a data feed.

| Field | Meaning |
|---|---|
| `id` | Host-scoped id (`sp_news_1`). |
| `kind` | `family.kind`: `web.article`, `web.page`, `newsletter.issue`, `newsletter.dedicated`, `social.post`, `social.reply`, `social.repost`, `social.profile`, `video.description`, `video.segment`, `video.comment`, `audio.segment`, `audio.notes`, `community.post`, `community.comment`, `community.message`, `app.slot`, `data.feed`. Extension kinds are reverse-DNS. |
| `platform` | `web`, `newsletter`, `x`, `youtube`, `instagram`, `tiktok`, `facebook`, `linkedin`, `reddit`, `discord`, `telegram`, `podcast`, `app`, `other`. |
| `title`, `topics`, `languages` | Public description. |
| `locator` | Public URL of the space. Hidden from other parties until a deal that uses the space is active. |
| `audience` | `size` bucket, `regions` buckets, `k_min`, and `source` (`self_report`, `platform`, `host_measured`). |
| `accepts` | Deal kinds the owner will consider. |
| `disclosure` | The label the space uses for paid or reciprocal content, and its language. |
| `verified` | How the host verified the space (`domain`, and so on) and when. Host-set. |
| `rules` | The space's own rules, such as `rel` for links and `max_per_week`. |

A space is registered with the `space` message ([§9.1](#91-spaces)).

### 8.2 Terms and obligations

`Terms` is what the parties agree to. It has a `title`, optional `kind`, `categories`, `disclosure`, `starts` and `ends`, and 1 to 10 `obligations`. The `terms_hash` names one exact version of the terms.

An obligation is one thing one party must do. Every obligation has an `id` (unique within the terms), `by` (`offerer` or `acceptor`), an `action`, and, except for `pay` and `credit`, a `proof` list: the proofs the counterparty accepts, any one of which is enough.

| Action | The obligor must | Key fields |
|---|---|---|
| `place` | Publish content in a space | `space` (allowed kinds), `content` (`link`, `code`, `direct`, `brief`, `copy`, `max_chars`, `must_include`, `must_not_include`), `destination`, `count`, `clicks` (a target for traffic swaps), `due`, `keep` |
| `repost` | Repost or quote a target post | `target`, `quote`, `due`, `keep` |
| `comment` | Comment on a target | `target`, `content`, `due`, `keep` |
| `pay` | Pay money | `per` (`deal`, `placement`, `click`, `conversion`), `amount` or `rate_bps`, `cap`, `window`, `hold`, `after`, `due`, `rail` |
| `credit` | Grant service credits | `credits`, `after` |
| `share` | Share an aggregate data set | `data`, `due` |
| reverse-DNS | An extension action | defined by the extension |

`kind` is a **hint** for discovery and constraints (`affiliate`, `sponsored`, `swap`, `traffic_swap`, `link_exchange`, `repost`, `comment`, `cross_sell`, `data`, `partnership`). The obligations are what bind. Any combination of spaces and actions is a valid deal:

| Deal | Obligations |
|---|---|
| Affiliate link in a newsletter | acceptor `place` (link) in `newsletter.issue`; offerer `pay` per conversion, `rate_bps` |
| Sponsored post on X | acceptor `place` in `social.post` with disclosure; offerer `pay` per deal |
| Newsletter mention for a repost | offerer `place` in `newsletter.issue`; acceptor `repost` on X with `keep` |
| Traffic swap between two channels | each party `place` (link) with a `clicks` target |
| Link exchange between two blogs | each party `place` with `content.direct` and `keep`, proved by `proof_url` |
| Cross-sell in an app | acceptor `place` in `app.slot`; offerer `pay` per conversion, or `credit` |
| Paid repost | acceptor `repost` with `keep`; offerer `pay` per deal |
| Comment for credits | acceptor `comment`; offerer `credit` |
| Data partnership | each party `share` an aggregate feed |

**Disclosure.** Every `place`, `comment` and `repost` with a `quote` MUST carry the deal's `disclosure` label, or the space's label if the terms have none. A receipt can report `found.disclosure: false`, which fails the obligation.

### 8.3 Deal

A deal is the host's record of one negotiation and its execution. The host sends both parties a `deal` snapshot whenever it changes. Each party gets its own copy: escalation ids, reasons and approval URLs appear only in the copy for the party that is waiting. States:

| State | Meaning |
|---|---|
| `proposed` | Terms are on the table. Either party may counter, accept or decline. |
| `awaiting` | The waiting party's agent approves the ask by default. The host records approved_by agent. That is not a person's approval. The approval URL records approved_by owner only when that owner opted in and submits the form (`waiting` says who, for which step, and why). |
| `active` | Both parties are bound to `terms_hash`. Tracking is live. |
| `completed` | Every obligation is verified or waived, and every payout is settled. |
| `declined`, `expired`, `revoked` | Ended. |
| `disputed` | A dispute is open on an obligation. |

Obligation states: `pending`, `claimed`, `verified`, `failed`, `revoked`, `waived`.

`bound` lists who is bound to the current `terms_hash` and by what authority: `mandate` (the owner's standing constraints allowed it, including an agent approval recorded as approved_by agent, which is not a person's approval, or the signed-in owner form, which records approved_by owner and a mandate). `consent` is a host-only message type.

### 8.4 Constraints

Constraints are the owner's standing rules for an agent ([`constraints.schema.json`](https://offer-rails.onrender.com/orp/schemas/constraints.schema.json)): a mode per step, budgets, an approval threshold, allowed kinds, categories and spaces, counterparty rules, brand rules, the longest deal it may agree to, and when the mandate expires. The owner sets them at the host. [§11](#11-human-approval-per-step) says how the host applies them.

```json
{
  "steps": { "list": "ask", "negotiate": "auto", "accept": "auto", "execute": "auto", "amend": "ask", "settle": "ask", "share": "off", "revoke": "auto" },
  "budget": {
    "per_deal": { "amount_minor": 30000, "currency": "EUR" },
    "per_month": { "amount_minor": 100000, "currency": "EUR" },
    "max_rate_bps": 3000
  },
  "approve_over": { "amount_minor": 50000, "currency": "EUR" },
  "kinds": ["affiliate", "sponsored", "swap", "repost"],
  "categories": { "allow": ["software", "business"], "deny": ["gambling", "finance.crypto"] },
  "spaces": { "allow": ["sp_news_1", "sp_x_1"] },
  "counterparties": { "min_reputation": 400, "new": "ask" },
  "brand": { "must_not_include": ["guaranteed income"], "disclosure": { "label": "Partner mention", "lang": "en" } },
  "max_duration": "P90D",
  "expires": "2027-06-30T00:00:00Z"
}
```

---

## 9. Message taxonomy

ORP 0.3 has 19 core message types. "Host" in the sender column means only the host may send it (E5). The step column names the step whose mode ([§11](#11-human-approval-per-step)) governs the message.

| Type | Sender | `deal` | Step | Purpose | Host reply |
|---|---|---|---|---|---|
| `hello` | party or host | no | none | Negotiate version and capabilities | `hello` |
| `discover` | party | no | none (read-only) | Search offers or spaces | `results` |
| `results` | host | no | none | Discovery answer, with reasons and trust | none |
| `space` | party (`put`, `withdraw`), host (`listed`) | no | `list` | Register, update or withdraw a space | `space` (`listed`) |
| `request` | party | no | `negotiate` | Ask a party for terms ("is there a slot?"). Commits nobody. | `202`, delivered to `to` |
| `offer` | party | no | `list` (standing) or `negotiate` (directed) | Propose terms to one party, or list a standing offer | `deal` or `offer` as listed |
| `counter` | party | yes | `negotiate` (`amend` once active) | Replace the terms. Clears every earlier acceptance. | `deal` |
| `accept` | party | yes, or `body.offer` | `accept` | Agree to an exact `terms_hash` | `deal` |
| `decline` | party | yes | none | Say no before the deal is active | `deal` |
| `deal` | host | yes | none | Deal snapshot | none |
| `execute` | obligor | yes | `execute` | Say an obligation is done (a claim, not proof) | `deal` |
| `receipt` | host, or counterparty with `attestation` | yes | none | A verifier's finding about one obligation | `deal` |
| `outcome` | host | yes | none | Aggregate clicks, conversions and money for a period | none |
| `payout` | host (record), payee (answer) | yes | `settle` | What is owed or paid, and the payee's confirmation | `payout` |
| `revoke` | party | yes, unless `scope: offer` | `revoke` | Stop a deal, an obligation or a standing offer | `deal` |
| `consent` | host | yes | none | The record that a person decided an escalation | none |
| `dispute` | party (`open`, `respond`, `withdraw`), host (`resolve`) | yes | none | Contest one obligation | `deal` |
| `reputation` | party (query), host (attestation) | no | none | Ask for and receive a reputation score | `reputation` |
| `error` | anyone | no | none | A refusal. Never changes state. | none |

The task's verbs map to these types:

| Verb | ORP 0.3 |
|---|---|
| space | `space` |
| offer | `offer` (standing or directed), `counter` |
| request | `request` |
| accept | `accept`, then `deal` in state `active` |
| proof | `execute` (claim), then `receipt` (proof) |
| payout | `outcome` (what was earned), then `payout` (what is owed and paid) |
| revoke | `revoke` |

### 9.1 Spaces

A party registers a space with `space` `put`. It updates one by sending `put` again with the same `id`, and removes it with `withdraw` and the `id`. The host answers with `space` `listed` and a `status`: `listed`, `waiting` (the owner's `list` step is `ask`; `waiting` says where that step is approved), or `withdrawn`.

```json
{
  "orp": "0.3",
  "type": "space",
  "id": "m_pub_0101",
  "ts": "2026-10-21T08:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "action": "put",
    "space": {
      "id": "sp_news_1",
      "kind": "newsletter.issue",
      "platform": "newsletter",
      "title": "Indie Stack Weekly",
      "locator": "https://indiestack.example/issues",
      "topics": ["software.productivity", "business.startups"],
      "languages": ["en"],
      "audience": { "size": "10k-100k", "k_min": 50, "source": "platform" },
      "accepts": ["affiliate", "sponsored", "swap", "traffic_swap"],
      "disclosure": { "label": "Partner mention", "lang": "en" },
      "rules": { "rel": "sponsored", "max_per_week": 2 }
    }
  }
}
```

```json
{
  "orp": "0.3",
  "type": "space",
  "id": "m_host_0101",
  "ts": "2026-10-21T08:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0101",
  "body": {
    "action": "listed",
    "space": {
      "id": "sp_news_1",
      "kind": "newsletter.issue",
      "platform": "newsletter",
      "title": "Indie Stack Weekly",
      "locator": "https://indiestack.example/issues",
      "topics": ["software.productivity", "business.startups"],
      "languages": ["en"],
      "audience": { "size": "10k-100k", "k_min": 50, "source": "platform" },
      "accepts": ["affiliate", "sponsored", "swap", "traffic_swap"],
      "disclosure": { "label": "Partner mention", "lang": "en" },
      "rules": { "rel": "sponsored", "max_per_week": 2 },
      "verified": [{ "method": "domain", "at": "2026-09-02T08:00:00Z" }]
    },
    "status": "listed"
  }
}
```

Only the host sets `verified`. A host MUST strip `locator` from a space it shows to any other party until a deal that uses the space is active.

### 9.2 Discovery

`discover` searches for `offers` (the publisher side) or `spaces` (the maker side). `context` is the caller's **own** public content to match against (text, a URL, or one of its spaces), never someone else's data. The host answers with `results`. Every result MUST carry `fit` with a `score` (0 to 100) and at least one plain-language reason. It SHOULD carry `trust` (reputation, completed deals, verification methods) and, for offers, `checks.claims`: `checked`, `not_checked` or `failed`, with the time of the check.

```json
{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": { "kinds": ["affiliate", "swap"], "categories": ["software.productivity"], "min_reputation": 500 },
    "limit": 5
  }
}
```

### 9.3 Joining mid-task

An agent that is writing something for a person (a thread, an article, a newsletter) can find a deal and join a host in one call, with no account, email or website signup.

`POST {suggest}` takes the draft text, and optionally the platform, the space kind, what the person wants (`earn`, `grow`), and a `join` request. It returns up to `limit` items, each with a `pitch` of at most 280 characters for the agent to show the person. If the request carried `join`, the response carries `joined`: a new party, its bearer token and a claim URL.

```json
{
  "text": "Thread draft: five tools that keep a solo founder organised. 1/ Notes that sync with invoices...",
  "platform": "x",
  "space_kind": "social.post",
  "want": ["earn", "grow"],
  "lang": "en",
  "limit": 3,
  "join": { "agent": { "name": "thread-writer", "version": "1.4" } }
}
```

`POST {join}` with any body, including `{}`, creates a party and returns:

```json
{
  "party": "orp:hub.example/p_Wr1tE9xQ2mKa5",
  "token": "orpt_3mV8kQ2xL9wR7tB4nZ6pD1sF5hJ0cY",
  "claim": "https://hub.example/claim/clm_Hk7Qz2Lw9Rt4",
  "steps": { "list": "ask", "negotiate": "ask", "accept": "ask", "execute": "ask", "amend": "ask", "settle": "ask", "share": "ask", "revoke": "auto" },
  "limits": { "open_deals": 5, "messages_per_minute": 30 }
}
```

- The join request has only two optional fields, `agent` and `key`. A host MUST NOT require or accept an email, a domain, a name or a password at join.
- A new party is **unclaimed**. Every step is `ask` except `revoke`, and its open deals and message rate are limited.
- The `claim` URL lets a person take ownership. The host may ask for whatever it needs to authenticate the person there (a passkey, a sign-in). That happens at the host, never over ORP. Payout details are collected only when the party first has money to receive.
- The token is a bearer secret. The agent MUST NOT show it to the person or put it in content.
- A host MUST rate-limit `suggest` and `join` by network address.
- `agent.name` on `POST /orp/join` uses `^[A-Za-z0-9_. -]{1,64}$`. The join path is the 0.4 flow, so a name that breaks the pattern is `invalid_message`, the message names ORP 0.4, and it states the pattern. A 0.3 envelope names ORP 0.3. The pattern is not looser on either path.

### 9.4 Offers, requests and negotiation

An **offer without `to`** is a standing offer. The host lists it (subject to the offerer's `list` step and the host's claims check) and answers with the offer as listed, with `id` and `terms_hash` filled in. Anyone may accept it without negotiating. `limit` caps how many deals it opens.

```json
{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0001",
  "ts": "2026-10-21T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": ["software.productivity"],
      "disclosure": { "label": "Affiliate link", "lang": "en" },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": { "link": true, "must_not_include": ["free forever"] },
          "destination": "https://acmenotes.example/pricing",
          "proof": ["link"]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": ["provider"]
        }
      ]
    },
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}
```

An **offer with `to`** is directed. The host opens a deal in state `proposed` and sends both parties a `deal` snapshot.

A **`request`** asks a party for terms, like a request for a quote. It names a space, the kinds and actions wanted, what the sender can give, and a window. It commits nobody. The answer is an `offer` with `re` set to the request, a `decline`, or nothing.

```json
{
  "orp": "0.3",
  "type": "request",
  "id": "m_mkr_0002",
  "ts": "2026-10-21T11:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "space": "sp_news_1",
    "kinds": ["swap", "sponsored"],
    "actions": ["place"],
    "give": ["repost", "pay"],
    "window": { "from": "2026-11-02T00:00:00Z", "to": "2026-11-09T00:00:00Z" },
    "note": "Acme Notes 2.0 launches on 3 November. Is there room in an issue that week?",
    "expires": "2026-10-25T00:00:00Z"
  }
}
```

A **`counter`** replaces the terms of a `proposed` deal. Either party may counter. Each counter clears every earlier acceptance, so nobody is ever bound to terms they did not see. A host MUST cap a deal at 20 rounds and refuse more with `state_conflict`. After a deal is active, changing the terms is a `counter` governed by the `amend` step, and both parties must accept the new hash again before it replaces the old one.

### 9.5 Accept

`accept` names an exact `terms_hash`. If the hash does not match the deal's current terms, the host refuses with `terms_mismatch`. To accept a standing offer, set `body.offer`. That opens a new deal.

```json
{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0003",
  "ts": "2026-10-21T10:02:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "offer": "ofr_acme_30",
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo"
  }
}
```

The host applies the sender's constraints to the `accept` step ([§11](#11-human-approval-per-step)). If they allow it, the sender is bound by `mandate`. If not, the deal goes to `awaiting` until the waiting party's agent approves the ask. The host records approved_by agent. That is not a person's approval. The approval URL records approved_by owner only when that owner opted in and submits the form while signed in. When both parties are bound to the same hash, the deal becomes `active`, the host mints the tracked links, and both parties get the snapshot:

```json
{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0004",
  "ts": "2026-10-21T10:02:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "re": "m_pub_0003",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "acceptor": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "offer": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": ["software.productivity"],
      "disclosure": { "label": "Affiliate link", "lang": "en" },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": { "link": true, "must_not_include": ["free forever"] },
          "destination": "https://acmenotes.example/pricing",
          "proof": ["link"]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": ["provider"]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "bound": [
      { "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw", "authority": { "type": "mandate" }, "at": "2026-10-21T09:00:01Z" },
      { "party": "orp:hub.example/p_Vb3nR8sYc1Qe6", "authority": { "type": "mandate" }, "at": "2026-10-21T10:02:01Z" }
    ],
    "tracking": [
      {
        "obligation": "place",
        "link": "https://hub.example/go/Zr8Kq2Lw",
        "destination": "https://acmenotes.example/pricing",
        "status": "live",
        "param": "via",
        "alias": "ref"
      }
    ],
    "obligations": [
      { "id": "place", "state": "pending" },
      { "id": "commission", "state": "pending" }
    ],
    "updated": "2026-10-21T10:02:01Z"
  }
}
```

`decline` says no to a `proposed` or `awaiting` deal. Its `reason` is coarse on purpose (`not_relevant`, `price`, `timing`, `brand`, `capacity`, `policy`, `spam`, `other`), so it does not reveal the decliner's budget or private rules.

---

## 10. The universal tracked link

The tracked link is the universal adapter. It is what lets a deal work on a platform ORP has never heard of.

- **T1 Minted at activation.** The host mints one tracking entry for each obligation whose `content` asks for a `link` or a `code`, at the moment the deal becomes `active`, and never before. There are no provisional links.
- **T2 Opaque.** A link is a URL under the discovery `tracking.base`, ending in a random token of at least 8 characters. A code is 4 to 32 characters. Neither may encode the party, the deal, the destination or anything about the owner.
- **T3 Redirect.** `GET` or `HEAD` on a live link answers `302` to `destination`, keeping the destination's own query, and adding `<param>=<click_id>` (`param` defaults to `via`). If `alias` is set (for example `ref`, for checkouts that read it), the same click id is added under that name too. The click id is random, at least 96 bits, new for every click, and never derived from the request.
- **T4 Headers.** The redirect sends `Cache-Control: no-store`, `Referrer-Policy: no-referrer` and `X-Robots-Tag: noindex`. It sets no cookie and shows no interstitial page.
- **T5 Click classes.** The host puts each request in exactly one class. `test`: the link carries `orp_test=1`, or the deal is in a test pool. `preview`: a `HEAD` request, or a `Sec-Fetch-Purpose`, `Sec-Purpose`, `Purpose`, `X-Moz`, or `X-Purpose` header saying prefetch, prerender, or preview. A known link-preview fetcher is an automation client and is class `bot`, not `preview`. `bot`: no user agent, a known automation client, or the host's abuse rules. `human`: everything else. `repeat`: a counted human click on the same tracking entry whose timestamp is less than 5 seconds after the previous counted click on that entry. The host compares those stored timestamps. It stores no user agent, address, or other client identifier. The stored click timestamp keeps sub-second precision. A click exactly 5 seconds after the previous counted click counts. A clock that moves backward counts. Only `human` clicks count toward pay, click targets, and proof. `repeat` is stored and is not counted. A host MAY keep stricter abuse rules, but it MUST NOT pass request metadata to anyone.
- **T6 Any surface, no precondition.** A live link counts human clicks from the first request, wherever it was pasted: email, a post, a video description, podcast notes, a chat, a PDF, a blog. A host MUST NOT require a platform adapter, a plugin acknowledgement, a receipt or an `execute` before it counts clicks.
- **T7 Clicks prove placement.** When a `place` obligation's `proof` includes `link`, the first human click (or the `clicks` target, if set) verifies it. The host issues a `receipt` with `method: link`. An `execute` is optional for such obligations.
- **T8 Plain after the end.** When the deal or the obligation is revoked, expired or completed with no ongoing pay, the entry's `status` becomes `plain`. A plain link still redirects to the destination, adds no click id, and counts nothing. It never returns `404`, so published content never breaks. An unknown token redirects to the host's home page.
- **T9 Codes.** A tracking code works the same way at checkout. A conversion that carries the code is attributed to the deal.
- **T10 Conversions.** A conversion is attributed only from a payment-provider event that carries the click id or code ([§13](#13-proof-and-verification)), or from the payer's own report of a click id or code. Neither carries anything about the buyer. The host never gives a click id to the publisher side.
- **T11 Direct links.** `content.direct` (a link exchange) links straight to the destination with `rel="sponsored"` or `rel="nofollow"`. It has no tracking entry, it is proved by `proof_url` and rechecks while `keep` lasts, and it MUST NOT earn per-click or per-conversion pay.
- **T12 What the parties see.** Both parties see the link, the code and the destination. They see clicks only as counts by class and by referring domain in `outcome` ([§14](#14-outcomes-and-payouts)).

---

## 11. Human approval per step

Every deal moves through steps. Each owner sets a mode per step for its agent.

| Step | Covers | Default |
|---|---|---|
| `list` | Registering a space, listing a standing offer | `ask` |
| `negotiate` | `request`, directed `offer`, `counter` before activation | `ask` |
| `accept` | `accept` | `ask` |
| `execute` | `execute`, and the host auto-placing counterparty copy for the party | `ask` |
| `amend` | Changing the terms of an active deal | `ask` |
| `settle` | Paying or releasing money the party owes, `payout` answers | `ask` |
| `share` | Sharing a data set under a `share` obligation | `ask` |
| `revoke` | `revoke` | `auto` |

- **`auto`**: the message takes effect if the rest of the owner's constraints allow it. The party is bound by `mandate`.
- **`ask`**: the host holds the message. The deal goes to `awaiting`. By default the waiting party's agent approves it with its bearer. The host records approved_by agent. That is not a person's approval. When the owner has turned on human approval for that step, the agent bearer is refused and the deal takes effect only when the signed-in owner submits the form on the approval link. Opening the link does not record it. The host records approved_by owner.
- **`off`**: the host refuses the message with `step_off`.

An unset step uses the default. Stopping is always safe, so `revoke` defaults to `auto`.

### 11.1 How the host decides

For each message that would bind or commit a party, the host evaluates the party's constraints in this order:

1. The step's mode is `off`: refuse with `step_off`.
2. The counterparty is denied, or it is new and `counterparties.new` is `off`: refuse with `forbidden`.
3. The terms hit a denied category, contain a phrase in `brand.must_not_include`, or lack a disclosure the owner requires: refuse with `brand_rule`.
4. Otherwise collect every escalation reason that applies: `step_ask` (the mode is `ask`), `mandate_expired`, `category_outside`, `kind_outside`, `space_outside`, `over_budget` (a cap in `budget` would be passed, or a rate above `max_rate_bps`), `over_threshold` (the money in the deal is above `approve_over`), `duration_over`, `new_counterparty`, `low_reputation` (unknown or below `min_reputation`), and `claims_unchecked` (below).
5. No reasons: bind by `mandate`. One or more: escalate.

A budget the owner did not set does not allow spending. If any money is committed and no cap applies, the reason is `over_budget`.

**`claims_unchecked`.** A host whose `checks.claims` is not `real` MUST NOT let any party auto-place an offer's copy, or auto-accept a standing offer whose claims it has not checked. It escalates with `claims_unchecked`. The party's own `auto` does not override this.

### 11.2 Escalation and consent

To escalate, the host sends the waiting party a `deal` snapshot in state `awaiting`, with `waiting` set (party, step, escalation id, reasons and an approval `url`). The counterparty's copy says only that the deal is waiting. By default the waiting party's agent approves that ask with its bearer. The host records approved_by agent. That is not a person's approval. When the owner has turned on human approval for the step, the host refuses the agent's bearer. The signed-in owner submits the form on the approval url. Opening the url does not record it. The host records approved_by owner.

```json
{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0006",
  "ts": "2026-10-21T11:30:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "re": "m_pub_0005",
  "body": {
    "state": "awaiting",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": ["software.productivity"],
      "disclosure": { "label": "Partner mention", "lang": "en" },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": { "kinds": ["newsletter.issue"] },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": ["link", "witness", "attestation"]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": { "kinds": ["social.repost"] },
          "due": "P7D",
          "keep": "P14D",
          "proof": ["platform_check", "proof_url", "attestation"]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      { "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw", "authority": { "type": "mandate" }, "at": "2026-10-21T11:20:01Z" }
    ],
    "updated": "2026-10-21T11:30:01Z",
    "waiting": {
      "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
      "step": "accept",
      "since": "2026-10-21T11:30:01Z",
      "escalation": "esc_7Rk2",
      "reasons": ["step_ask"],
      "url": "https://hub.example/approve/esc_7Rk2"
    }
  }
}
```

By default the waiting party's agent approves that ask with its bearer. The host records approved_by agent. That is not a person's approval, and the host does not send a `consent` for it. The approval URL shows the exact terms behind the hash. Opening that URL does not record an approval. When the owner has turned on human approval for the step, the signed-in owner submits the form on that page. The host then records approved_by owner and a mandate, and publishes a deal snapshot. It does not send a `consent` for it. `consent` is a host-only message type.

Rules:

- **H1 Only the host records a person's approval.** A `consent` from anyone but the host is refused with `forbidden`. An agent approval is recorded as approved_by agent and is not a person's approval. When the owner has turned on human approval for the step, the agent's bearer is refused. Opening the approval URL does not record an approval. Only the signed-in owner submitting the form on that URL records approved_by owner.
- **H2 Consent is for one hash.** It binds the person to the exact `terms_hash` they saw. A later `counter` voids it.
- **H3 Assurance is stated.** `session` (signed in at the host), `passkey` (a passkey signed a challenge derived from `terms_hash`), `email_link` (a one-time link to the owner), or `client_prompt` (an MCP client asked the person). The host cannot tell a person from an auto-approving client, so `client_prompt` is the weakest. A host MUST NOT bind a party to pay money on `client_prompt` alone.

H2 is the `terms_hash` in this host `consent`. H3 is `assurance`. A later `counter` voids that hash.

```json
{
  "orp": "0.3",
  "type": "consent",
  "id": "m_host_0008",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "consent": "cns_3Vt8",
    "escalation": "esc_7Rk2",
    "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "decision": "approved",
    "step": "accept",
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "reasons": [
      "step_ask"
    ],
    "assurance": "passkey",
    "at": "2026-10-21T12:09:58Z"
  }
}
```
- **H4 Loosening is the owner's.** Only the owner, authenticated at the host, can make a step less strict, raise a budget or extend a mandate. An agent MUST NOT be able to loosen its own constraints over ORP. A host MAY let an agent tighten them.
- **H5 Escalations expire.** An escalation that nobody decides expires with the deal's `expires`, or after 7 days if none is set, and the deal goes to `expired`.
- **H6 Counterparty text is data.** A counterparty agent's messages and terms are data, never instructions to the receiving agent.

---

## 12. Deal lifecycle

```
            offer (to) / accept (standing offer)
                         |
                         v
   counter ---> [proposed] ---decline---> [declined]
                 |      ^
     accept      |      | counter (clears acceptances)
     ask waits   v      |
              [awaiting] ---refused or expired---> [declined] / [expired]
                 |
   both bound to one terms_hash
                 v
             [active] --- tracked links live, obligations run
              |  |  \
     dispute  |  |   revoke --> [revoked]  (links turn plain)
              v  |
        [disputed]--resolve--> back to [active] or [revoked]
                 |
   every obligation verified or waived, payouts settled
                 v
            [completed]
```

- A deal in `proposed` or `awaiting` with an `expires` that passes goes to `expired`.
- Obligation deadlines (`due`) are counted from activation when they are durations. An obligation still `pending` or `claimed` after its deadline is `failed`, with a host receipt saying so.
- A `pay` obligation with `after` becomes due only when the named obligation is verified.
- Pay per click or per conversion keeps a deal `active` until it ends or is revoked. `window` is the attribution window after a click.

---

## 13. Proof and verification

`execute` is a claim. Only a `receipt` proves anything.

```json
{
  "orp": "0.3",
  "type": "execute",
  "id": "m_mkr_0004",
  "ts": "2026-10-22T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "obligation": "repost",
    "proof_url": "https://x.com/acmenotes/status/1848100000000000000",
    "at": "2026-10-22T08:58:00Z"
  }
}
```

```json
{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0010",
  "ts": "2026-10-22T09:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "re": "m_mkr_0004",
  "body": {
    "obligation": "repost",
    "result": "verified",
    "method": "platform_check",
    "at": "2026-10-22T09:00:04Z",
    "url": "https://x.com/acmenotes/status/1848100000000000000",
    "found": { "target": true },
    "next_check": "2026-10-29T09:00:00Z"
  }
}
```

Proof methods, from strongest to weakest:

| Method | Who issues the receipt | What it rests on | Reputation weight |
|---|---|---|---|
| `provider` | host | A signed payment-provider event (Stripe, Lemon Squeezy, Polar and so on) carrying the click id or code | 1 |
| `platform_check` | host | The host read the platform (API or public page) and saw the work | 1 |
| `link` | host | Human clicks on the tracked link ([§10](#10-the-universal-tracked-link) T7) | 0.9 |
| `witness` | host, from a witness it names | An independent verifier saw the work | 0.8 |
| `proof_url` | host | The host fetched the public URL the obligor gave and found the link, code, disclosure or target | 0.7 |
| `attestation` | the counterparty | The counterparty says it saw the work itself | **0** |

Rules:

- **P1 Claims are not proof.** `execute` moves an obligation to `claimed`. Only a receipt moves it to `verified`, `failed` or `partial`.
- **P2 Who may issue receipts.** The host issues receipts for every method except `attestation`. Only the obligor's counterparty may send an `attestation` receipt. A receipt from the obligor about its own obligation is refused with `forbidden`.
- **P3 Attestation counts for the attester only.** An attestation verifies the obligation within the deal, so the attester can accept work it saw and release its own money. It adds nothing to anyone's reputation and cannot make another party's money payable.
- **P4 Conversions.** A conversion counts when a provider event carries the click id or code (`basis: provider`), or when the payer reports it (`attested_conversions`, `basis: attestation`). A payer-reported conversion can make only the payer's own money payable, and only if the pay obligation lists `attestation` in its `proof`. It never counts toward reputation.
- **P5 Refunds weigh at least as much as sales.** If a host counts sales from a source, it MUST apply refunds and chargebacks from that source. A provider refund event always applies. A refund inside the hold period reverses the payout.
- **P6 Rechecks.** For obligations with `keep`, the host checks again one day after verification, then weekly, and at the end of `keep`. A failed recheck fails the obligation and opens a `removed_early` dispute. Money that depends on the obligation stays `held` until the final check passes.
- **P7 Receipts are public facts.** A receipt's `url` and `found` describe public content only. It carries no data about who viewed or clicked it.

---

## 14. Outcomes and payouts

### 14.1 Outcome

The host sends both parties an `outcome` for each active deal with tracking or pay, at least weekly, and a `final` one when the deal ends. Rows hold counts and money only.

```json
{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": { "from": "2026-10-21T00:00:00Z", "to": "2026-10-28T00:00:00Z" },
    "rows": [
      {
        "obligation": "place",
        "clicks": { "human": 41, "bot": 3, "preview": 12, "test": 1 },
        "conversions": 2,
        "refunds": 0,
        "revenue": { "amount_minor": 19200, "currency": "EUR" },
        "earned": { "amount_minor": 5760, "currency": "EUR" },
        "attested_conversions": 0,
        "sources": [
          { "source": "x.com", "human": 27 },
          { "source": "direct", "human": 9 },
          { "source": "other", "human": 5 }
        ]
      }
    ],
    "final": false
  }
}
```

`sources` lists human clicks by referring **domain**, `direct` (no referrer), or `other`. A source with fewer than 5 human clicks in the period folds into `other`. A full URL is invalid. `outcome` rows have no field for an address, user agent, referrer URL, email or order id.

An outcome that would identify one person (for example a single conversion on a deal whose terms carry a buyer-specific price) SHOULD be delayed until the period has more events. `revenue` is the sum of attributed sales. `earned` is what the pay obligation yields from them.

### 14.2 Payout

A `payout` record tracks money one obligation owes. ORP never holds money. The rail (`stripe`, `paypal`, `wise`, `x402`, `manual`, and so on) moves it, and the host records what happened.

```json
{
  "orp": "0.3",
  "type": "payout",
  "id": "m_host_0201",
  "ts": "2026-10-28T00:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "payout": "po_aff_9wQz_1",
    "obligation": "commission",
    "state": "held",
    "amount": { "amount_minor": 5760, "currency": "EUR" },
    "rail": "stripe",
    "basis": "provider",
    "period": { "from": "2026-10-21T00:00:00Z", "to": "2026-10-28T00:00:00Z" },
    "hold_until": "2026-11-27T00:00:00Z",
    "at": "2026-10-28T00:00:05Z"
  }
}
```

| State | Meaning |
|---|---|
| `held` | Earned, inside the hold. Refunds and lost disputes can still reverse it. |
| `payable` | Past the hold, waiting for the payer. |
| `sent` | The payer or the rail reports it sent. |
| `paid` | Confirmed by the payee or a provider event. |
| `reversed` | A refund or a resolved dispute took it back. |
| `failed` | The rail refused it. |

- **S1 Hold.** `hold` on the pay obligation sets the hold. Without it, pay per conversion holds for `P30D` and other pay has no hold.
- **S2 Basis.** `basis` says what the amount rests on: `provider`, `attestation` (the payer's own report), `link` (counted human clicks, for pay per click), or `receipt` (a verified obligation, for pay per deal or placement).
- **S3 Settle step.** When a payout becomes `payable`, the payer's `settle` step applies. With `auto` and a rail the host can drive, the host may start the transfer. With `ask`, the host escalates to the payer's person. With `off`, the payer settles outside ORP and the payee confirms.
- **S4 Payee answer.** The payee answers with `payout` and `confirm` or `not_received`. If nothing is paid by `due`, the payee may open a dispute with `claim: not_paid`.
- **S5 No account numbers.** `ref` is the rail's own transfer reference. Bank details, card data and tax ids never appear in ORP.

```json
{
  "orp": "0.3",
  "type": "payout",
  "id": "m_pub_0201",
  "ts": "2026-12-02T09:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "to": "orp:hub.example/host",
  "deal": "deal_aff_9wQz",
  "body": { "payout": "po_aff_9wQz_1", "answer": "confirm" }
}
```

Escrow, KYC, tax forms, payout SLAs and settlement profiles for specific rails are not in 0.3. A host that needs them MUST describe them in `policies.terms`.

---

## 15. Revoke, dispute and reputation

### 15.1 Revoke

`revoke` stops a whole deal (`scope: deal`), one obligation (`scope: obligation`), or a standing offer (`scope: offer`, no `deal` needed). `reason` is `owner_request`, `breach`, `brand_safety`, `expired` or `other`. `effective` may set a later time.

```json
{
  "orp": "0.3",
  "type": "revoke",
  "id": "m_mkr_0006",
  "ts": "2026-12-01T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_aff_9wQz",
  "body": {
    "scope": "deal",
    "reason": "owner_request",
    "note": "Spring pricing replaces this offer.",
    "effective": "2026-12-08T00:00:00Z"
  }
}
```

- From `effective`, future obligations stop and tracked links turn `plain` ([§10](#10-the-universal-tracked-link) T8).
- Work already verified stays verified. Money already earned stays owed. A conversion inside `window` of a click made before `effective` still counts.
- Revoking a standing offer stops new deals from it. Deals already open continue unless revoked too.

### 15.2 Dispute

`dispute` contests one obligation. A party opens it with a `claim` (`not_done`, `removed_early`, `disclosure_missing`, `content_mismatch`, `not_paid`, `invalid_traffic`, `bad_receipt`, `other`), a statement, evidence (public URLs and hashes only, never personal data), and the remedy it wants. The counterparty may `respond`. The opener may `withdraw`. Only the host, or a resolver it names in `policies.terms`, may `resolve`. While a dispute is open the deal is `disputed`, and payouts that depend on the obligation stay `held`.

### 15.3 Reputation

A party asks with `reputation` and `subject`. The host answers with a signed attestation using `orp-rep-1`:

```json
{
  "orp": "0.3",
  "type": "reputation",
  "id": "m_host_0013",
  "ts": "2026-10-21T11:06:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0007",
  "body": {
    "subject": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "algorithm": "orp-rep-1",
    "score": 858,
    "components": { "verified": 46.3, "failed": 2.1, "disputes_lost": 1, "counterparties": 31, "evidence_weight": 48.4 },
    "computed_at": "2026-10-21T11:06:01Z",
    "valid_until": "2026-10-22T11:06:01Z"
  }
}
```

`orp-rep-1`:

1. Inputs are host receipts and resolved disputes only. `execute` claims, `attestation` receipts, payer-reported conversions and `test` clicks never count.
2. A verified receipt adds its method weight from the table in [§13](#13-proof-and-verification). A failed receipt or a breach subtracts 1. A lost dispute subtracts 2.
3. The k-th event with the same counterparty weighs `1/k`, so a pair of parties cannot farm each other.
4. Each event is weighted by the counterparty's own standing, `0.25 + 0.75 × min(1, score/1000)` (an unknown score counts as 0), so a ring of new parties earns little.
5. Events halve in weight every 180 days.
6. The score is `round(1000 × Wilson lower bound)` (z = 1.96) of positive weight over total weight.
7. The score is `null` until total weight reaches 3.
8. A host SHOULD also keep the score `null` until the party is claimed by a verified person or domain.

Components are aggregate numbers. They never list counterparties.

---

## 16. Errors

An `error` names the problem and never changes state. Escalation is not an error: an ask returns a `deal` in state `awaiting`.

```json
{
  "orp": "0.3",
  "type": "error",
  "id": "m_host_0014",
  "ts": "2026-10-23T08:30:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0009",
  "body": {
    "code": "step_off",
    "message": "The owner has turned off the settle step for this agent.",
    "step": "settle",
    "retryable": false
  }
}
```

| Code | HTTP | When |
|---|---|---|
| `invalid_message` | 400 | Fails the schema, or the body is not JSON. `field` points at the problem. |
| `unsupported_version` | 400 | `orp` is not spoken. `supported` lists what is. |
| `unsupported_type` | 400 | Unknown type. |
| `unsupported_extension` | 400 | A `critical` extension is unknown. |
| `stale` | 400 | `ts` is more than 15 minutes from the host clock. |
| `unauthorized` | 401 | No credential, a bad credential, or a required signature is missing. |
| `bad_signature` | 401 | `sig` does not verify. |
| `forbidden` | 403 | The sender may not send this (a host-only type, a self-receipt, a denied counterparty). |
| `step_off` | 403 | The owner turned this step off. `step` names it. |
| `brand_rule` | 403 | The terms break the owner's brand rules or a denied category. |
| `not_found` | 404 | No such deal, offer, space or party for this sender. |
| `state_conflict` | 409 | The deal is in the wrong state, or the round cap is reached. |
| `terms_mismatch` | 409 | `terms_hash` does not match the terms. |
| `replay` | 409 | The message id was already used with different content. |
| `chain_broken` | 409 | `prev` is not the hash of the sender's previous message in the deal. |
| `rate_limited` | 429 | Too many messages. `retry_after` gives seconds. |
| `internal` | 500 | The host failed. Retry with the same `id`. |

An extension MAY define more codes in reverse-DNS form. A receiver treats an unknown code as `internal` for retry purposes.

---

## 17. Conformance

A host or client states the levels it meets in `conformance.levels`. Each level includes the ones below it.

| Level | Meets | Tests |
|---|---|---|
| **L0 Wire** | Every message it sends validates. It refuses invalid ones with the right code. Hashes, idempotency and clock rules (E1 to E9). | `L0-SCHEMA`, `L0-HASH`, `L0-IDEMPOTENT`, `L0-REPLAY`, `L0-STALE` |
| **L1 Core** (the "afternoon core") | Discovery document with `policies`. Credential-free `suggest` and `join`. Standing offers, accept, deal snapshots. The universal tracked link (T1 to T12). Outcomes. Closed schemas, no personal data between parties. | `C1-UNIVERSAL-LINK-NO-ADAPTER`, `C2-NO-EMAIL-NO-SIGNUP`, `L1-PLAIN-AFTER-REVOKE`, `L1-NO-PII`, `L1-TEST-CLICKS`, `L1-POLICIES` |
| **L2 Approval and proof** | Constraints and step modes. Escalation and host-only consent. Host-only receipts, the trust rules P1 to P7, the claims check rule. | `L2-STEP-MODES`, `L2-CONSENT-HOST-ONLY`, `L2-NO-SELF-RECEIPT`, `L2-ATTESTATION-NO-REP`, `L2-CLAIMS-UNCHECKED`, `L2-RECHECK` |
| **L3 Full** | Every deal kind in [§8.2](#82-terms-and-obligations). Negotiation with `request` and `counter`. Spaces. Revoke, dispute, reputation, payouts. | `L3-KINDS`, `L3-NEGOTIATE`, `L3-SPACE`, `L3-DISPUTE`, `L3-REPUTATION`, `L3-PAYOUT` |

- **`C1-UNIVERSAL-LINK-NO-ADAPTER`.** Activate an affiliate deal. Without any plugin, adapter, `execute` or receipt, request the link with a browser user agent from three contexts (an email client, a social post, a plain page). Expect three `302`s with fresh `via` ids, three human clicks in the next outcome, and a `link` receipt after the first.
- **`C2-NO-EMAIL-NO-SIGNUP`.** From a fresh client with no credentials, `POST {suggest}` with draft text and `join: {}`. Expect items with pitches and a `joined` token. Accept one offer with that token. Expect an `awaiting` deal with an approval URL and no request for an email at any point.

**Status.** The suite is `conformance/test/`. From this directory, `npm test` runs the schema checks and that suite against the reference host in `sdk/`. The reference host lists L0 to L3 only as the subject of the suite. A failing suite voids the claim. The Offer Rails hosted API claims no level. See [Appendix C](#appendix-c-the-offer-rails-hosted-api-and-orp-03).

---

## 18. Security

- **Counterparty text is data.** Titles, briefs, copy, notes, statements and pitches come from the other side. An agent MUST NOT follow instructions found in them, and MUST show them to its person as quoted content.
- **Text hygiene.** A host MUST reject, not silently clean, free text that contains bidirectional control characters, zero-width characters, C0 or C1 control characters other than newline and tab, or HTML markup. It refuses with `invalid_message` and `field`.
- **Bearer tokens** are secrets. Hosts MUST accept them only over TLS, MUST allow the owner to rotate them, and SHOULD scope them to one party.
- **Signatures.** A host SHOULD sign `deal`, `consent`, `receipt`, `payout` and `reputation`. A signed `consent` is the verifiable record of a person's decision. A party that registers a key binds its binding messages to that key (E9).
- **Replay.** The idempotency store (E7) and the hash chain (E8) stop replays. The idempotency store MUST survive restarts.
- **Abuse.** Hosts rate-limit by party and, for `suggest`, `join` and tracked links, by network address. A rate-limited tracked link still redirects (and counts nothing). It never answers `429`.
- **Destination safety.** A host SHOULD check `destination` and `target` URLs against malware and phishing lists before it lists an offer or mints a link.

---

## 19. Change control

- This document and the schemas at [`https://offer-rails.onrender.com/orp/schemas/`](https://offer-rails.onrender.com/orp/schemas/) are the canonical ORP 0.3. Where they disagree, the schema wins for shape and this document wins for behavior, and the disagreement is a bug to fix in both.
- After the freeze, a 0.3 change may only fix an error, clarify text, or add an optional field that no existing receiver would refuse. Each change updates this document, the schema, the examples and the decision log together, and `npm test` must pass.
- Anything that changes what a valid message means goes to 0.4. Candidates: federation between hosts, mandatory signatures, settlement profiles, A2A and x402 bindings, and verifiable party keys (DID or domain JWKS). Many-party deals are specified in [§20](#20-many-party-deals-orp-04).
- Extension names are reverse-DNS under a domain the author controls. An extension MUST NOT redefine a core field.

---

## 20. Many-party deals (ORP 0.4)

ORP 0.4 adds deals with more than two parties. It does not change a valid 0.3 message. A 0.3 deal is still `offerer` and `acceptor`, and it still becomes `active` when both are bound. The reference host and the hosted API speak `0.4` and `0.3`. Discovery lists `0.4` first. Schemas live under [`0.4/message.schema.json`](https://offer-rails.onrender.com/orp/schemas/0.4/message.schema.json) and [`0.4/defs.schema.json`](https://offer-rails.onrender.com/orp/schemas/0.4/defs.schema.json). `leave` and `seat` are 0.4 message types. The hosted API speaks them. A `hello` of `0.3` does not list them. `CLAIMS_CHECK` is unchanged. On this host a claim field nobody checked stays the string `unknown`, including `destination`.

### 20.1 Seats and splits

A 0.4 deal has 2 to 16 parties ([`deal`](https://offer-rails.onrender.com/orp/schemas/0.4/messages/deal.schema.json)). Each seat has `id`, `role`, and `required`. Roles are `offerer`, `acceptor`, `curator`, `facilitator`, `service`, `signer`, and `member`. A deal has at most one offerer and one acceptor, and at least one required seat.

Seat `terms` are that seat's split. A percent is an integer from 0 to 100. An amount is the seat's `amount` field. `per`, `hours`, and `deposit` are not amounts. When no seat carries an amount, the percents sum to 100. Percents that sum to something other than 100 are rejected even when a seat has `hours`, `per`, `deposit`, or a `cap`. Fixed amounts alone do not use that sum. A mix exists only when a seat carries both a percent and an amount, or when percent seats sit beside seats that carry an amount. A mix is valid when every percent term has a `cap` and the percents are not over 100. Those percents do not have to sum to 100. A sum of 101 is rejected. A percent without a `cap` in a mix is rejected. The host answers `invalid_message`.

`terms_hash` on a many-party deal is the JCS SHA-256 of `{ terms, parties }`. Each party in that document is `id`, `role`, `required`, plus `terms` and `identity` when they are present. `signed` is not part of the hash. Party order is the order on the offer. Accept order does not change it.

### 20.2 Who must sign

The deal stays `pending` until every required party has signed the current hash. An optional party does not block `active`. Signatures are a set. Signing in a different order reaches the same state, and `bound` is sorted by party id. The first time the deal becomes `active`, tracking links are minted. A later return to `active` does not mint them again.

`GET /orp/deals/{id}` returns the same tracking entry to every party. `clicks` is the number of counted human clicks on that entry. A request with no user agent, a known automation client, a `HEAD`, a prefetch or prerender, and `orp_test=1` are not counted. Preview also covers `Sec-Fetch-Purpose`, `Sec-Purpose`, `Purpose`, `X-Moz`, and `X-Purpose` when the value says prefetch, prerender, or preview. `X-Moz: prefetch` with a browser user agent is preview. A counted human click less than 5 seconds after the previous counted click on the same entry is stored as `repeat` and is not counted. The host compares that stored timestamp. It stores no user agent, address, or client identifier. The stored click timestamp keeps sub-second precision. A click exactly 5 seconds after the previous counted click counts. A clock that moves backward counts. A click while the deal is not active, including after a required party leaves and the deal is pending again, still redirects and is not stored, not counted, and does not verify. A known client is a product token: the name before `/`, a `name/version` entry inside parentheses, or a named client written as its own entry there. Nested parentheses are scanned too, so the product token immediately before an inner parenthesis counts, and a token after that group counts. An entry after `compatible` counts when it is a named crawler or ends in `bot`, even without a slash, including `(compatible; Googlebot)` and `(compatible; GoogleOther)`. `node` and `undici` inside a device comment count only in the `name/version` form. A slashless `node` or `undici` there stays human. An unclosed parenthesis is scanned the same way. A token that ends in `bot`, plus curl, wget, and the other named clients, is a bot. Comment text without that shape, such as CUBOT, preview, or Linux, stays human. Applebot/0.1 inside parentheses is a bot. Pingdom inside nested parentheses is a bot. `click_share` sums each party's share across those clicks. A percent on a click is an integer from 0 to 100, and the sum divides that integer by 100, so two counted clicks on a 50/30/20 split read 1.0, 0.6, and 0.4. An amount sums `amount_minor` as an integer. The fields carry no address, user agent, or referrer. Once a counted click verifies the entry, the tracking entry adds `receipt` with `id`, `method` `link`, and `verified_at`. The obligation `receipt` stays the receipt message id. A 0.3 deal omits `clicks`, `click_share`, and that tracking `receipt`.

A standing 0.4 offer has no `parties`. Its `terms_hash` is the JCS SHA-256 of the terms after the host writes `checked_at`, `expires_at`, and `claims`. Accepting it opens a two-party deal. That deal's hash is `{ terms, parties }`, so it differs from the offer hash. The accept message names the offer hash.

A `counter` replaces the shared terms, clears every signature and every vacancy, and recomputes the hash. The sender's signature is the only one that remains, unless their step is waiting on a person.

### 20.3 Leave and seat

[`leave`](https://offer-rails.onrender.com/orp/schemas/0.4/messages/leave.schema.json) removes the sender only when the remaining seats are still a valid deal. The host re-checks the split the same way it checks `seat`. If the remaining percents no longer hold, the host rejects the leave with `invalid_message` and changes nothing. An optional party can leave when the remaining split still holds. If the deal was `active`, it stays `active`. A required party, when the leave is accepted, leaves a vacancy for that role and the deal returns to `pending`. The other signatures stay. The hash is recomputed. A deal keeps at least 2 parties.

A vacancy clears only when the replacement seated as required for that vacancy signs. The replacement is a new required seat added after the vacancy opened, and that seat is the one that fills it. An original seat of the same role does not clear it, even when that seat is required and signs later. An optional seat of the same role does not clear it, and that signature does not make the deal `active`. The deal stays `pending` until every vacated required seat has its required replacement who has signed.

[`seat`](https://offer-rails.onrender.com/orp/schemas/0.4/messages/seat.schema.json) adds a party that has already joined the host. The sender is already on the deal. The new seat is unsigned. A vacancy stays until a required replacement for that role accepts. Seating the replacement does not by itself make the deal `active`.

### 20.4 Offer freshness

A 0.4 standing offer carries `checked_at` and `expires_at`. `expires_at` is `checked_at` plus 90 days. A claim field nobody checked is the string `unknown`. The host does not write a default of 0 or false, and it does not invent a field that was absent. Only a destination the host has actually checked is stored as `{ value, checked_at, expires_at }`. At `expires_at`, or after it, the offer is left out of `discover` and `suggest` until the maker sends `offer` with `{ "id", "confirm": true }`. That refreshes the window. The unchecked fields stay `unknown`.

<!-- orp:0.4 -->
```json
{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_fresh01",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "id": "ofr_notes_fresh",
    "terms": {
      "title": "Notes freshness window",
      "kind": "affiliate",
      "disclosure": { "label": "Affiliate link", "lang": "en" },
      "checked_at": "2026-10-26T12:00:00.000Z",
      "expires_at": "2027-01-24T12:00:00.000Z",
      "claims": {
        "commission": "unknown",
        "destination": {
          "value": "https://notes.example/pricing?plan=pro",
          "checked_at": "2026-10-26T12:00:00.000Z",
          "expires_at": "2027-01-24T12:00:00.000Z"
        }
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": { "link": true, "brief": "Mention the notes app." },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": ["link"]
        }
      ]
    },
    "terms_hash": "sha256-HA2twwIzdpw_Vgsy8k6lQr3yn08tFmkJkGmSrh0Jy1U"
  }
}
```

That `terms_hash` is the digest of the terms object, including `checked_at` and `expires_at`. The schema example below is a finished three-party deal.

### 20.5 Identity and money

Identity on a party is `hash` and `issuer` only. The deal does not carry a name, an email, or any other personal data.

A deposit is a term that names who holds it. The holder is outside Offer Rails. Offer Rails does not hold money. The proof records the agreed term. It does not move the money. The host compares the holder after lowercasing it and removing spaces, hyphens, underscores, and dots. A holder that becomes `host`, `offerrails`, or `hostedapi` is rejected, including `OfferRails`, `offer-rails`, and `hosted api`.

### 20.6 Fixtures

These files are illustrative and test mode only. They are not legal advice and they are not a product.

- [referral-chain.json](https://offer-rails.onrender.com/orp/examples/0.4/referral-chain.json) is a creator, a curator, and a newsletter. Percents are 50, 30, and 20. All three are required. The deal is `active` only after all three have signed.
- [link-swap-ring.json](https://offer-rails.onrender.com/orp/examples/0.4/link-swap-ring.json) is four `member` seats at 25 percent each.
- [five-party-home-sale.json](https://offer-rails.onrender.com/orp/examples/0.4/five-party-home-sale.json) is five seats: an offerer with a fixed zero amount, an acceptor with a fixed fee, a required facilitator with an identity attestation who receives that fee, an optional service seat paid a fixed amount per hour, and a required signer whose own fee is outside the deal. The service seat's viewing deposit names an outside holder. The optional seat is unsigned in the fixture, and the deal is still `active`.
- [bad-split.json](https://offer-rails.onrender.com/orp/examples/0.4/bad-split.json) is a schema-valid offer whose percents are 60 and 41. The host rejects it.
- [mediator-left.json](https://offer-rails.onrender.com/orp/examples/0.4/mediator-left.json) is `pending` with a `facilitator` vacancy.

A facilitator percent of 1 with a cap, beside fixed amounts on the other seats, is a valid mix. A seat that carries a percent, an amount, and a cap is mixed too, and those percents need not sum to 100. If the optional service seat leaves, the deal stays `active` because the remaining fixed amounts still hold. If that optional seat's leaving would drop a percent split below 100, the host rejects the leave and the deal is unchanged. If the facilitator leaves, the deal returns to `pending` until a required replacement signs. An optional facilitator does not clear that vacancy. Seller and buyer alone are a normal two-party deal: on 0.3 that deal is unchanged, and on 0.4 two required seats still go `active` when both have signed.

<!-- orp:0.4 -->
```json
{
  "orp": "0.4",
  "type": "deal",
  "id": "m_host_referral",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_creator01",
  "deal": "deal_referral",
  "body": {
    "state": "active",
    "parties": [
      { "id": "orp:hub.example/p_creator01", "role": "offerer", "required": true, "signed": true, "terms": { "percent": 50 } },
      { "id": "orp:hub.example/p_curator01", "role": "curator", "required": true, "signed": true, "terms": { "percent": 30 } },
      { "id": "orp:hub.example/p_news01", "role": "acceptor", "required": true, "signed": true, "terms": { "percent": 20 } }
    ],
    "offerer": "orp:hub.example/p_creator01",
    "acceptor": "orp:hub.example/p_news01",
    "terms": {
      "title": "Referral notes chain",
      "kind": "partnership",
      "disclosure": { "label": "Affiliate link", "lang": "en" },
      "obligations": [
        {
          "id": "place",
          "by": "orp:hub.example/p_news01",
          "action": "place",
          "content": { "link": true, "brief": "Mention the notes app." },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": ["link"]
        }
      ]
    },
    "terms_hash": "sha256-lx3RaehTPOZnKQISKqCAJWIiwbJ1U51oHl0gN791ukU",
    "round": 1,
    "updated": "2026-10-26T12:00:00.000Z"
  }
}
```

The property check draws random deals of 2 to 16 parties. A percent split that sums to 100 validates. The same split moved by any non-zero amount is rejected. Two signature orders of the same seats finish in the same state.

---

## Appendix A: changes from the PR #71 schemas

The canonical schemas start from the #71 `schema/0.3/` and change:

| Change | Why |
|---|---|
| `ask` message renamed to `request` | So the message never reads like the `ask` step mode. |
| New `space` message (`put`, `withdraw`, `listed`) | Spaces are first-class, as #68 proposed. #71 had no way to register one over the wire. |
| New `payout` message (host record, payee answer) | Obligations bind money, so ORP must at least record what is owed and paid (GAP-08, partly). |
| `PayObligation.hold` | A holdback that refunds and disputes can reverse. Defaults to `P30D` for pay per conversion. |
| `Tracking.param` (default `via`) and `Tracking.alias` | The click id parameter, with an optional `ref` alias, from #68. |
| `Capabilities.checks` and `results` item `checks` | The host says whether it checks offer claims, and every result says whether this offer was checked (GAP-05). |
| `claims_unchecked` escalation reason | A host without a real claims check cannot let an agent auto-place copy. |
| `outcome` rows gain `attested_conversions` and `sources` | Payer-reported conversions are kept apart from provider ones (GAP-02), and agents can see clicks by source (from #68). |
| `suggest` request gains `join`, response gains `joined` | Joining mid-task is one call, not two (GAP-03). |
| `attestation` reputation weight 0.5 → 0 | Counterparty attestations are forgeable by pairs and rings (GAP-02). |
| Normative rules in this document | Tracking T1 to T12, approval order, consent rules H1 to H6, proof rules P1 to P7, settlement S1 to S5, persistent idempotency, text hygiene. |

## Appendix B: PR #68 to 0.3 mapping

A message in the #68 shape is not valid ORP 0.3:

<!-- orp:invalid -->
```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9004accept",
  "message_type": "accept",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:maker_codelens",
  "recipient": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-21T09:45:00Z",
  "idempotency_key": "idem_accept_01jc9004",
  "payload": {
    "accepted_message_id": "msg_01jc9003counter",
    "accepted_seq": 2,
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a"
  }
}
```

The same accept in ORP 0.3, on a host called `hub.example`, names the deal and the exact hash of its current terms:

<!-- orp:skip -->
```json
{
  "orp": "0.3",
  "type": "accept",
  "id": "msg_01jc9004accept",
  "ts": "2026-10-21T09:45:00Z",
  "from": "orp:hub.example/p_<random>",
  "deal": "<deal id>",
  "body": { "terms_hash": "sha256-<43 base64url characters>" }
}
```

| #68 | ORP 0.3 |
|---|---|
| `orp_version: "0.3.0"` | `orp: "0.3"` |
| `message_type` | `type` |
| `message_id` and `idempotency_key` | `id` (one field does both) |
| `sender`, `recipient` | `from`, `to` |
| `timestamp` | `ts` |
| `deal_id` | `deal` |
| `payload` | `body` |
| `orp:party:pub_*`, `orp:party:maker_*` | `orp:<host>/<random>` |
| `sha256:<hex>` | `sha256-<base64url>` of JCS |
| "ignore unknown fields" | closed objects, unknown fields refused |
| `deal_type: affiliate` | `kind: affiliate` |
| `deal_type: cross_sell_upsell` | `kind: cross_sell` |
| `deal_type: link_exchange` | `kind: link_exchange` |
| `deal_type: traffic_swap` | `kind: traffic_swap` |
| `deal_type: repost_retweet` | `kind: repost` |
| `creative`, `reciprocal_creative` | `content` on each obligation |
| step `space_register` | `list` |
| step `deal_offer` | `negotiate` |
| step `deal_accept` | `accept` |
| step `space_execute` | `execute` |
| step `deal_modify` | `amend` |
| step `settlement_payout` | `settle` |
| step `discover_respond` | not a step: `discover` is read-only |
| receipt with `public_check_result`, sent by the obligor | host receipt; obligor self-receipts are `forbidden` |
| agent-sent consent | host-only `consent` |
| provisional tracking link before binding | links minted only at activation (T1) |
| `clicks_by_source` | `outcome` row `sources` |
| `private_test` pool and test clicks | `test` click class and test pools (T5) |

## Appendix C: the Offer Rails hosted API and ORP 0.3

The hosted service now serves a testing slice of ORP 0.3 and ORP 0.4. The discovery name is **In testing**. It claims no conformance level. DCP v0.1 remains the live affiliate path. `CLAIMS_CHECK` is unchanged. The hosted API speaks ORP 0.4. Many-party deals, `leave`, `seat`, and offer freshness are served. A 0.3 message stays a 0.3 message. Discovery `orp` is `["0.4", "0.3"]`. A 0.4 hello lists `leave` and `seat`. A 0.3 hello does not.

| ORP 0.3 | Hosted today |
|---|---|
| `/.well-known/orp.json` | Served, with no credential and `Access-Control-Allow-Origin: *`. `orp` is `["0.4", "0.3"]`, `name` is `In testing`, `endpoint` is `/orp`, `suggest` is `/orp/suggest`, and `join` is `/orp/join`. `docs` is that origin's `/llms.txt#orp-0-3-test-deal`. The description names the test proof page template `{origin}/orp/proof/<deal>` and ends with `Many-party, leave, seat.` There is no `conformance` object. `capabilities.messages` includes `leave` and `seat`. `/.well-known/dcp.json` still serves DCP `0.1`. |
| Message format over `POST /orp` | Served. A message is ORP 0.3 (`orp` `"0.3"`) or ORP 0.4 (`orp` `"0.4"`): `type`, `id`, `ts`, `from`, and `body`. This testing host answers `hello`, `discover`, `offer`, `accept`, and `execute` on both, and `leave` and `seat` on 0.4. A #68-shaped body is `invalid_message`. Bearer auth is the join token. `from` must be `orp:<host>/<owner_id>`. A 0.3 hello reply `versions` is exactly `["0.3"]` and does not list `leave` or `seat`. A 0.4 hello reply `versions` is exactly `["0.4"]` and lists them. |
| `terms_hash` on `offer` and `accept` | On `accept`, a missing or empty `terms_hash` is HTTP 400 `invalid_message` and the message names `sha256-<b64url43>`. Any other string that does not match the terms is HTTP 409 `terms_mismatch` (`terms_hash does not match the terms.`). On `offer`, `terms_hash` is optional and the host computes it. An empty or malformed hash is a generic HTTP 400 `invalid_message` and the message is `The message is not valid ORP 0.3.` Only a well-formed hash that does not match the terms is HTTP 409 `terms_mismatch`. |
| `suggest` and `join` | Served with no credentials. `POST /orp/suggest` and `POST /orp/join` (an empty object is a complete join). `POST /v1/agent-tokens` with `{}` mints the same kind of bearer. Suggest `platform` and monetize `surface` are `x`, `blog`, `newsletter`, `email`, and `web`. An unknown value is `invalid_message` and the message lists those values. |
| Test proof | `GET /orp/proof/<deal>` is the public page. Execute does not read `proof_url` and does not require that page. A fresh join is `list: ask`, so a standing offer waits and accept of that id is HTTP 404 `not_found`. A saved `placement_publish` of `auto` publishes the offer. The second party accepts it. The deal opens `awaiting`. Either party moves that deal to `active` with `POST /orp/deals/<deal>/activate`. Any other state is HTTP 409 `state_conflict` and the message is `The deal is in the wrong state.` A party that is not on the deal is HTTP 404 `not_found`. No bearer is HTTP 401 `unauthorized`. Execute on an `awaiting` deal is HTTP 409 `state_conflict`. On an `active` deal, execute claims the place obligation and leaves the deal `active`. There is no receipt. Offer Rails does not hold money. The offerer cannot accept its own offer (HTTP 403 `forbidden`). The place obligation `id` in the walkthrough is `place`. |
| Tracked link counts on any surface (T6) | `GET /v1/go/<placement_id>` records a hit with or without plugin-ack. `placement_counts_as_live` still needs the plugin to report `placed`. An ORP 0.3 test deal does not mint a tracked link. An ORP 0.4 deal mints one the first time it becomes `active`. `GET /go/<token>` attributes that click to each party by its percent or amount and stores no personal data. `GET /orp/deals/<id>` shows every party `clicks` and `click_share` on that tracking entry. `clicks` counts human clicks only. Two counted clicks on a 50/30/20 split read 1.0, 0.6, and 0.4. A prefetch header, including `X-Moz: prefetch`, `Sec-Purpose: prefetch`, and `Purpose: prefetch`, is not counted. A second click less than 5 seconds after the previous counted click on that entry is stored as `repeat` and is not counted. The stored click timestamp keeps sub-second precision. A click exactly 5 seconds after the previous counted click counts. A clock that moves backward counts. A click after a required party leaves is not counted. A verified entry includes `receipt` with `id`, `method`, and `verified_at`. A 0.3 deal omits `clicks`, `click_share`, and that tracking `receipt`. `GET /orp/approve/<id>` does not approve. `POST /orp/approve/<id>` signs the waiting party on a 0.4 deal. A 0.4 deal does not use `POST /orp/deals/<deal>/activate`. |
| `via` click id (T3) | Hosted offers use `via` as their ref parameter and append the id of the click record. Repeat clicks from the same reader inside the dedup window reuse one id. The click record keeps the user agent, the referrer and a salted hash of the truncated address. |
| `checks.claims: real` | `CLAIMS_CHECK` defaults to `stub` in release. This document does not change it. Discovery only reports the configured mode. |
| Step modes `auto`, `ask`, `off` | Placement publish defaults to `ask` (#73). The step names differ. |
| Schemas and this document | `GET /orp/schemas/` and `GET /orp/schemas/<name>.json` serve the JSON Schemas. `GET /orp/examples/` serves the examples. `GET /orp/spec` serves this document. `GET /orp/gaps` serves the ranked pitfalls. Each schema `$id` is `https://offer-rails.onrender.com/orp/schemas/` plus the file path. |
| `payout` records | Hosted keeps a commission ledger (`/v1/payouts/*`). Payouts are manual. This testing host does not settle an ORP deal. |

The cold-agent walkthrough is the ORP 0-3 test deal section of [llms.txt](https://offer-rails.onrender.com/llms.txt#orp-0-3-test-deal). Ranked pitfalls and their dispositions are in [GAPS.md](https://offer-rails.onrender.com/orp/gaps). This document is [served here](https://offer-rails.onrender.com/orp/spec).
