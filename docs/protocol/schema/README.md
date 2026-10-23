# ORP 0.3 JSON Schemas

JSON Schema (draft 2020-12) for every Offer Rails Protocol message, plus the discovery document, the owner's private constraints, and the HTTP suggest, join and inbox shapes. The prose is [`../SPEC.md`](../SPEC.md), the canonical ORP 0.3. These schemas replace the `schema/` directory of draft PR #71. The changes are listed in Appendix A of the spec.

| File | What it checks |
|------|----------------|
| `0.3/envelope.schema.json` | The six required envelope fields and the optional ones |
| `0.3/defs.schema.json` | Shared types: party ids, money, terms, obligations, tracking, spaces, capabilities, checks |
| `0.3/messages/<type>.schema.json` | One file per core message type (19) |
| `0.3/message.schema.json` | Any message: the envelope, then the body rules for its `type` |
| `0.3/discovery.schema.json` | `/.well-known/orp.json` |
| `0.3/constraints.schema.json` | An owner's limits for its agent (kept by the host, never sent to a counterparty) |
| `0.3/http/suggest-*.schema.json` | `POST {suggest}`, with optional `join` and `joined` |
| `0.3/http/join-*.schema.json` | `POST {join}` (an empty request is complete) |
| `0.3/http/inbox.schema.json` | `GET {endpoint}/inbox` |

`examples/messages/` tells one story in 40 messages. A space is listed. A standing affiliate offer is found, accepted, tracked, reported and paid out. Then a newsletter-for-repost swap is requested, offered, countered, escalated to a person, approved, executed, receipted, disputed and resolved. More files show traffic swaps, link exchanges, cross-sells and paid reposts. The `terms_hash` values are the real hashes of the terms in those files, and the test checks them. `examples/invalid/` holds documents that must fail, each with the reason and the place it must fail. Several are shapes from draft PR #68.

```bash
cd docs/protocol/schema
npm ci
npm test
```

The L0 to L3 behavioral suite lives next to this directory. `cd docs/protocol && npm ci && npm test` runs these schema checks and that suite.

The test compiles every schema with Ajv in strict mode and validates every example. It checks that each invalid example fails where it says, and recomputes every `terms_hash` next to its terms. It also validates every ```json block in `docs/protocol/*.md` that looks like an ORP document. A block marked `<!-- orp:invalid -->` must fail, and a block marked `<!-- orp:skip -->` is skipped.

Core objects are closed (`additionalProperties: false`), and extension data goes in the envelope's `ext` field under a reverse-DNS name. A closed schema is therefore also the privacy check: an outcome row has no field that could hold an address, a user agent or an email.
