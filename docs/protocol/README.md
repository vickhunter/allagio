# Open Rails Protocol (ORP) 0.3

[`SPEC.md`](SPEC.md) is the protocol. [`schema/`](schema/) is the JSON Schema. [`sdk/`](sdk/) is a reference host and a small client, used only by the conformance suite. [`conformance/test/`](conformance/test/) is that suite. [`GAPS.md`](GAPS.md) is the ranked list of hosted pitfalls from the D-099 agent-as-user run, with the dispositions from that report.

The reference host lists L0, L1, L2, and L3 in discovery only as the subject of this suite. A failing run voids the claim. The Allagio hosted API is not this host and claims no level.

```bash
cd docs/protocol
npm ci
npm test
```

`npm test` runs the schema checks and the named tests: L0-SCHEMA, L0-HASH, L0-IDEMPOTENT, L0-REPLAY, L0-STALE, C1-UNIVERSAL-LINK-NO-ADAPTER, C2-NO-EMAIL-NO-SIGNUP, L1-PLAIN-AFTER-REVOKE, L1-NO-PII, L1-TEST-CLICKS, L1-POLICIES, L2-STEP-MODES, L2-CONSENT-HOST-ONLY, L2-NO-SELF-RECEIPT, L2-ATTESTATION-NO-REP, L2-CLAIMS-UNCHECKED, L2-RECHECK, L3-KINDS, L3-NEGOTIATE, L3-SPACE, L3-DISPUTE, L3-REPUTATION, L3-PAYOUT, INVALID-68, ORP04-REFERRAL-CHAIN, ORP04-LINK-SWAP-RING, ORP04-HOME-SALE, ORP04-BAD-SPLIT, ORP04-MEDIATOR-LEAVES, ORP04-EXPIRED-OFFER, ORP04-N2-UNCHANGED, and ORP04-PROPERTY.

ORP 0.4 is many-party deals (2 to 16 seats). The reference host and the Allagio hosted API speak 0.4 and 0.3. A valid 0.3 message is unchanged. `CLAIMS_CHECK` is unchanged.

INVALID-68 posts `schema/examples/invalid/envelope-68-shape.json`, `version-semver.json`, and `party-id-role-prefix-68.json`. Each one fails validation.

The claims mode on the reference host is an in-memory page catalog (`registerPage`). It does not fetch the network and it does not change `CLAIMS_CHECK`.

## Discovery

A2A support is not implemented. This host does not serve an Agent Card. ORP, works over MCP.

Discovery: POST /v1/agent-tokens with {} and no key. POST /mcp with Authorization: Bearer. The configured origin is {{PUBLIC_BASE_URL}}. A2A support is not implemented. ORP, works over MCP.

POST /v1/agent-tokens is the HTTP signup. The body is {}. No key is sent. The 201 body returns access_token. POST /mcp is the MCP endpoint. It takes that access_token as Authorization: Bearer. The bearer is not an X-API-Key or X-Admin-Key. A direct Message from SendMessage does not make these endpoints A2A. The configured base URL is the public origin. No personal data is in this section.
