# DCP — Distribution Context Protocol, v0.1

DCP is a vocabulary and semantics standard for agent-driven distribution
networks, layered on MCP the way MCP is layered on JSON-RPC: MCP standardizes
*how* agents call tools, DCP standardizes *which* tools a distribution network
exposes and what its attribution guarantees mean. An agent that speaks DCP can
take a product from zero → assessed → listed → earning on any compliant
network without learning a proprietary dashboard.

v0.1 is **extracted from the working Offer Rails network in this repository**
(spec follows implementation). Every verb, schema, and semantic documented here
is implemented and tested in `affilio-api/`. The name an agent sees is
`offer-rails-mcp`.

| Document | Contents |
|---|---|
| [entities.md](entities.md) | The six entities and their fields |
| [verbs.md](verbs.md) | The MCP tool contract (maker verbs) and HTTP-bound publisher verbs |
| [attribution.md](attribution.md) | Tracked-link, conversion, and ledger semantics |
| [discovery.md](discovery.md) | The `/.well-known/dcp.json` manifest |

## Conformance, v0.1

A DCP endpoint MUST:

1. Expose the maker verb set of [verbs.md](verbs.md) as MCP tools with the
   documented input fields and result shapes.
2. Implement the attribution semantics of [attribution.md](attribution.md):
   click-id ref parameter, idempotent conversion ingestion, refund-window
   holds, honest `unattributed` reporting.
3. Serve a discovery manifest at `/.well-known/dcp.json`.
4. Gate offer listing on a readiness assessment whose failing checks carry
   machine-actionable `remediation` strings.

Reference implementation: this repository. Reference client + MCP server:
`affilio-api/internal/dcp`, `affilio-api/cmd/distributoor-mcp` (that directory
name stays; the server reports `offer-rails-mcp`). The npm package
`offer-rails` is not published yet.
