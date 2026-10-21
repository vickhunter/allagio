# ORP 0.3 JSON Schemas

JSON Schema (draft 2020-12) for every Offer Rails Protocol message, plus the discovery document, the owner's private constraints, and the HTTP inbox page. The prose lives in [`../spec`](../spec/README.md).

| File | What it checks |
|------|----------------|
| `0.3/envelope.schema.json` | The six required envelope fields and the optional ones |
| `0.3/defs.schema.json` | Shared types: party ids, money, terms, obligations, tracking, spaces |
| `0.3/messages/<type>.schema.json` | One file per message type (17) |
| `0.3/message.schema.json` | Any message: the envelope, then the body rules for its `type` |
| `0.3/discovery.schema.json` | `/.well-known/orp.json` |
| `0.3/constraints.schema.json` | An owner's limits for its agent (kept by the host, never sent to a counterparty) |
| `0.3/http/inbox.schema.json` | `GET {endpoint}/inbox` |

`examples/messages/` is one story told in 30 messages: a standing affiliate offer found, accepted and tracked, then a newsletter-for-repost swap negotiated with a counter, escalated to a person, approved, executed, receipted, disputed and resolved. The `terms_hash` values are the real hashes of the terms in those files. `examples/invalid/` holds documents that must fail, each with the reason and the place it must fail.

```bash
cd schema
npm install
npm test
```

The test compiles every schema with Ajv in strict mode, validates every example, checks each invalid example fails where it says, and validates every ```json block in `spec/*.md` that looks like an ORP document.

Core objects are closed (`additionalProperties: false`): extension data goes in the envelope's `ext` field under a reverse-DNS name, so a closed schema is also the privacy check. An outcome row has no field that could hold an address, a user agent or an email.
