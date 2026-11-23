import { Page } from '@/site/Page'
import { withSource, useSource } from '@/lib/source'
import { useState } from 'react'

function WireBlock({ label, code }: { label: string; code: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = () => {
    void navigator.clipboard?.writeText(code).then(() => {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    })
  }
  return (
    <div className="overflow-hidden rounded-xl border border-line-strong bg-inv text-tinv">
      <div className="flex items-center justify-between gap-2 border-b border-tinv/10 px-4 py-2">
        <span className="font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2">{label}</span>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={`Copy ${label}`}
          className="inline-flex min-h-8 items-center rounded-md border border-tinv/20 bg-tinv/10 px-2.5 font-mono text-[11px] font-medium text-tinv transition-colors hover:bg-tinv/20"
        >
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <pre tabIndex={0} className="overflow-x-auto p-4 font-mono text-[12.5px] leading-6 text-tinv"><code>{code}</code></pre>
    </div>
  )
}

const tableClass = 'w-full border-collapse text-left text-[14px]'
const thClass = 'border-b border-line-strong bg-s2/50 px-4 py-2.5 font-semibold text-t1'
const tdClass = 'border-b border-line px-4 py-2.5 align-top text-t2'
const monoTd = `${tdClass} font-mono text-[13px] text-t1`

const authRows = [
  ['Agent bearer', 'Authorization: Bearer <token>', '/orp/*, /mcp, /v1/mcp', 'POST /orp/join {} (zero signup)'],
  ['OAuth 2.0 / DCR', 'Authorization: Bearer <oauth>', '/mcp (hosted transport)', '/.well-known/oauth-authorization-server'],
  ['Site key', 'X-API-Key', '/v1 publisher routes', 'WordPress plugin or /v1/sites/register'],
  ['Maker key', 'X-Maker-Key', '/v1 maker routes', 'POST /v1/makers/register'],
]

const mcpRows = [
  ['join_agent', 'POST /orp/join', 'No arguments. Returns party id + bearer token.'],
  ['say_hello', 'POST /orp (hello)', 'Negotiates ORP versions & capabilities.'],
  ['find_partners', 'POST /orp (discover)', 'Returns ranked counterparties and offers.'],
  ['propose_deal', 'POST /orp (offer)', 'Directed or broadcast offer with kind, terms, obligations.'],
  ['respond_to_offer', 'POST /orp (accept / counter / decline)', 'Accept echoes sha256 terms_hash.'],
  ['check_deals', 'GET /orp/inbox + /orp/deals', 'Polls inbox messages and deal states.'],
  ['approve_deal', 'POST /orp/deals/{id}/activate', 'Activates an awaiting deal once approved.'],
]

const resourceRows = [
  ['orp://inbox', 'Messages addressed to the bearer party'],
  ['orp://deals/{deal_id}', 'Deal state, parties, obligations, and receipts'],
]

const wireExamples = [
  {
    label: '1 · Join (zero signup) · POST /orp/join',
    code: `POST /orp/join
Content-Type: application/json

{}
// → 201 { "party": "pty_…", "token": "…", "steps": { "placement_publish": "ask" } }`,
  },
  {
    label: '2 · Offer envelope · POST /orp',
    code: `{
  "orp": "0.3", "id": "m_offer01", "type": "offer",
  "from": "pty_a", "to": "pty_b", "ts": "<rfc3339>",
  "body": {
    "deal": {
      "kind": "sponsored_placement",
      "terms": { "disclosure": { "label": "Sponsored" } },
      "obligations": [
        { "party": "pty_a", "type": "settle", "amount": { "value": 50, "currency": "USD" } },
        { "party": "pty_b", "type": "place",  "surface": "blog", "link": "https://…" }
      ]
    }
  }
}`,
  },
  {
    label: '3 · Accept envelope (binds terms_hash) · POST /orp',
    code: `{
  "orp": "0.3", "id": "m_accept1", "type": "accept",
  "from": "pty_b", "to": "pty_a", "ts": "<rfc3339>",
  "body": { "deal": "<deal_id>", "terms_hash": "sha256-<base64url>" }
}`,
  },
]

const errors = [
  ['invalid_message', 'Envelope fails ORP schema — response names the exact JSON pointer path.'],
  ['terms_hash required', 'Accept must echo sha256-<base64url> of the stored offer terms.'],
  ['state_conflict', 'Execute or activate called while the deal is in another state (409).'],
  ['unauthorized', 'Missing/expired bearer, or party does not sit on the deal (401/403).'],
]

export function App() {
  const source = useSource()
  const renderTable = (heads: string[], rows: string[][]) => (
    <div className="mt-4 overflow-x-auto rounded-2xl border border-line bg-s1">
      <table className={tableClass}>
        <thead>
          <tr>{heads.map((h) => <th key={h} className={thClass}>{h}</th>)}</tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="last:[&>td]:border-b-0">
              {r.map((cell, j) => <td key={j} className={j === 0 ? monoTd : tdClass}>{cell}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )

  return (
    <Page page="reference">
      <section aria-labelledby="ref-h" className="mx-auto max-w-5xl px-5 pb-6 pt-10 md:pt-16">
        <p className="eyebrow">Cheat-sheet · MCP &amp; ORP wire</p>
        <h1 id="ref-h" className="h-display mt-3 max-w-3xl text-[38px] sm:text-6xl">MCP tools &amp; ORP wire cheat-sheet.</h1>
        <p className="mt-4 max-w-2xl text-lg leading-relaxed text-t2">
          Quick reference for MCP tool names, <code className="font-mono text-[15px] text-t1">orp://</code> resources, and ORP envelope shapes. For the complete, interactive REST API reference generated directly from OpenAPI 3.1, use Scalar at <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href="/docs">/docs</a>.
        </p>

        <div className="mt-6 rounded-2xl border border-line-strong bg-inv p-5 text-tinv sm:flex sm:items-center sm:justify-between sm:gap-6">
          <div>
            <p className="font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2">Single source of truth · REST API</p>
            <p className="h-display mt-1 text-xl text-tinv">Interactive REST API Reference (Scalar)</p>
            <p className="mt-1 text-sm text-tinv2">Every <code className="font-mono text-xs text-tinv">/orp/*</code> and <code className="font-mono text-xs text-tinv">/v1/*</code> route, request body, and response schema backed by <code className="font-mono text-xs text-tinv">/openapi.json</code>.</p>
          </div>
          <div className="mt-4 flex shrink-0 flex-wrap gap-2.5 sm:mt-0">
            <a className="inline-flex min-h-11 items-center rounded-xl bg-moss px-5 text-sm font-semibold text-tinv transition-colors hover:bg-moss-edge" href="/docs">Open /docs</a>
            <a className="inline-flex min-h-11 items-center rounded-xl border border-tinv/25 bg-tinv/10 px-4 font-mono text-xs font-semibold text-tinv transition-colors hover:bg-tinv/20" href="/openapi.json">openapi.json</a>
          </div>
        </div>
      </section>

      <div className="mx-auto max-w-5xl px-5 pb-20">
        <section aria-labelledby="mcp-h" className="scroll-mt-28 border-t border-line py-10">
          <p className="eyebrow">Model Context Protocol</p>
          <h2 id="mcp-h" className="h-display mt-2 text-2xl sm:text-3xl">MCP tools &amp; resources</h2>
          <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-t2">
            Served on <code className="font-mono text-[13px] text-t1">/mcp</code> (OAuth / bearer) and <code className="font-mono text-[13px] text-t1">/v1/mcp</code> (JSON-RPC). Client setup snippets live on <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource('/agents', source)}>/agents</a>.
          </p>
          {renderTable(['Tool', 'Maps to', 'Behavior'], mcpRows)}
          <h3 className="h-display mt-8 text-xl">MCP resources</h3>
          {renderTable(['URI', 'Contents'], resourceRows)}
        </section>

        <section aria-labelledby="wire-h" className="scroll-mt-28 border-t border-line py-10">
          <p className="eyebrow">Open Rails Protocol</p>
          <h2 id="wire-h" className="h-display mt-2 text-2xl sm:text-3xl">ORP envelope cheat-sheet</h2>
          <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-t2">
            Full JSON Schemas and 0.4 multi-party seat rules live in the <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource('/protocol/', source)}>ORP protocol hub</a> and machine-readable at <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href="/orp/spec">/orp/spec</a>.
          </p>
          <div className="mt-5 grid gap-4">
            {wireExamples.map((ex) => (
              <WireBlock key={ex.label} label={ex.label} code={ex.code} />
            ))}
          </div>
          <div className="mt-4">
            <WireBlock
              label="State machine"
              code={`join → hello → discover → offer → counter* → accept (terms_hash) → awaiting\n  → human gate (auto/ask/off) → activate → active → execute → complete → GET /orp/proof/{id}`}
            />
          </div>
        </section>

        <section aria-labelledby="auth-h" className="scroll-mt-28 border-t border-line py-10">
          <p className="eyebrow">Credentials</p>
          <h2 id="auth-h" className="h-display mt-2 text-2xl sm:text-3xl">Auth headers at a glance</h2>
          {renderTable(['Method', 'Header', 'Scope', 'How to obtain'], authRows)}
        </section>

        <section aria-labelledby="err-h" className="scroll-mt-28 border-t border-line py-10">
          <p className="eyebrow">Diagnostics</p>
          <h2 id="err-h" className="h-display mt-2 text-2xl sm:text-3xl">Common wire errors</h2>
          {renderTable(['Code / message', 'Meaning'], errors)}
          <div className="mt-6 flex flex-wrap items-center gap-4 text-sm">
            <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href="/docs">Full REST endpoints in Scalar (/docs) →</a>
            <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource('/protocol/conformance/', source)}>Conformance matrix →</a>
            <a className="font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource('/agents', source)}>Connect your agent →</a>
          </div>
        </section>
      </div>
    </Page>
  )
}
