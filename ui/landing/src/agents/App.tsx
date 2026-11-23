import { useState } from 'react'
import { agentPrompt, aloneToday, deviceSignIn, errors, installCheck, installs, mcpUrl, modes, rates, tools } from '@/agents/facts.mjs'
import { publishStepParagraph } from '@/vision.mjs'
import { Page } from '@/site/Page'
import { useSource, withSource } from '@/lib/source'

const primaryIds = ['claude', 'cursor', 'codex', 'generic'] as const

const orpJoinSnippet = `curl -fsS -X POST "${mcpUrl.replace(/\/mcp$/, '')}/orp/join" \\
  -H "Content-Type: application/json" \\
  -d '{}'`

const orpJoinResponse = `{
  "party": "pty_01j…",
  "token": "orp_live_…",
  "claim": "${mcpUrl.replace(/\/mcp$/, '')}/owner",
  "steps": { "placement_publish": "ask" },
  "limits": { "open_deals": 5, "messages_per_minute": 30 }
}`

function DarkCodeBlock({ label, value, copyLabel = 'Copy', statusName, primary }: { label: string; value: string; copyLabel?: string; statusName: string; primary?: boolean }) {
  const [status, setStatus] = useState('')
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setStatus('Copied')
    } catch {
      const area = document.createElement('textarea')
      area.value = value
      document.body.appendChild(area)
      area.select()
      const ok = document.execCommand('copy')
      area.remove()
      setStatus(ok ? 'Copied' : 'Select and copy')
    }
    window.setTimeout(() => setStatus(''), 2000)
  }
  return (
    <div className="overflow-hidden rounded-xl border border-line-strong bg-inv text-tinv">
      <div className="flex items-center justify-between gap-3 border-b border-tinv/10 px-4 py-2.5">
        <span className="font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2">{label}</span>
        <div className="flex items-center gap-2">
          {status ? <span role="status" aria-live="polite" data-copy-status={statusName} className="font-mono text-[11px] text-tinv2">{status}</span> : <span role="status" aria-live="polite" data-copy-status={statusName} className="sr-only" />}
          <button
            type="button"
            data-copy={statusName}
            {...(primary ? { 'data-primary-action': true } : {})}
            onClick={handleCopy}
            className={primary
              ? 'inline-flex min-h-9 items-center rounded-lg bg-moss px-3.5 font-mono text-xs font-semibold text-tinv transition-colors hover:bg-moss-edge'
              : 'inline-flex min-h-9 items-center rounded-lg border border-tinv/20 bg-tinv/10 px-3 font-mono text-xs font-medium text-tinv transition-colors hover:bg-tinv/20'}
          >
            {status === 'Copied' ? 'Copied' : copyLabel}
          </button>
        </div>
      </div>
      <pre tabIndex={0} className="max-w-full overflow-x-auto whitespace-pre-wrap break-all p-4 font-mono text-[13px] leading-6 text-tinv"><code>{value}</code></pre>
    </div>
  )
}

function InstallTabs() {
  const [showAll, setShowAll] = useState(false)
  const primary = primaryIds
    .map((id) => installs.find((item) => item.id === id))
    .filter((item): item is (typeof installs)[number] => Boolean(item))
  const extra = installs.filter((item) => !primaryIds.includes(item.id as (typeof primaryIds)[number]))
  const visible = showAll ? [...primary, ...extra] : primary
  const [active, setActive] = useState(primary[0]?.id ?? installs[0].id)
  const current = installs.find((item) => item.id === active) ?? primary[0] ?? installs[0]

  return (
    <div className="mt-4">
      <div className="flex flex-wrap items-center gap-2">
        <div
          role="tablist"
          aria-label="MCP client install"
          className="flex flex-wrap gap-2"
          onKeyDown={(event) => {
            if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return
            event.preventDefault()
            const index = visible.findIndex((item) => item.id === active)
            const next = event.key === 'ArrowRight' ? (index + 1) % visible.length : (index - 1 + visible.length) % visible.length
            setActive(visible[next].id)
          }}
        >
          {visible.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={`install-tab-${item.id}`}
              aria-selected={item.id === active}
              aria-controls={`install-panel-${item.id}`}
              className={item.id === active
                ? 'inline-flex min-h-11 shrink-0 items-center gap-2 whitespace-nowrap rounded-xl border border-t1 bg-t1 px-4 text-sm font-semibold text-s0'
                : 'inline-flex min-h-11 shrink-0 items-center gap-2 whitespace-nowrap rounded-xl border border-line bg-s1 px-4 text-sm font-semibold text-t1 transition-colors hover:bg-s2'}
              onClick={() => setActive(item.id)}
            >
              <span>{item.id === 'generic' ? 'Generic MCP' : item.label}</span>
              {item.tested ? <span className={item.id === active ? 'rounded-full bg-tinv/20 px-2 py-0.5 font-mono text-[10px]' : 'rounded-full bg-pub-soft px-2 py-0.5 font-mono text-[10px] text-pub'}>checked</span> : null}
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => setShowAll((v) => !v)}
          className="inline-flex min-h-11 items-center rounded-xl border border-line px-3 text-xs font-semibold text-t2 transition-colors hover:bg-s2 hover:text-t1"
        >
          {showAll ? 'Fewer clients' : `+${extra.length} more clients`}
        </button>
      </div>

      <div role="tabpanel" id={`install-panel-${current.id}`} aria-labelledby={`install-tab-${current.id}`} className="mt-4 space-y-3">
        <DarkCodeBlock
          label={`${current.id === 'generic' ? 'Generic MCP' : current.label} · config`}
          value={current.snippet}
          copyLabel={current.id === 'generic' ? 'Copy config' : `Copy ${current.label}`}
          statusName={current.id === 'generic' ? 'config' : current.id}
        />
        <p className="text-sm leading-relaxed text-t2">{current.proof}</p>
      </div>
    </div>
  )
}

export function App() {
  const source = useSource()
  const publisherTools = tools.filter((tool) => !tool.maker)
  const makerTools = tools.filter((tool) => tool.maker)

  return (
    <Page page="agents">
      <div className="mx-auto max-w-5xl px-5 pb-20 pt-10 md:pt-16">
        <section id="agents-hero" aria-labelledby="agents-h">
          <p className="eyebrow">Agents hub · MCP &amp; ORP</p>
          <h1 id="agents-h" className="h-display mt-3 max-w-3xl text-[38px] sm:text-6xl">Connect your agent in 60 seconds.</h1>
          <p className="mt-4 max-w-2xl text-lg leading-relaxed text-t2">
            No signup form and no role pick. Point your agent at the Allagio MCP server or call <code className="font-mono text-[15px] text-t1">POST /orp/join</code> with an empty object to get a party ID and bearer token.
          </p>

          <div className="mt-6 flex flex-wrap items-center gap-3">
            <a className="inline-flex min-h-11 items-center rounded-xl bg-t1 px-5 text-sm font-semibold text-s0 transition-colors hover:bg-[rgb(60_66_55)]" href="/docs">Open REST API docs (Scalar)</a>
            <a className="inline-flex min-h-11 items-center rounded-xl border border-line-strong bg-s1 px-5 text-sm font-semibold text-t1 transition-colors hover:bg-s2" href={withSource('/protocol/', source)}>ORP protocol spec</a>
            <a className="inline-flex min-h-11 items-center px-2 text-sm font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource('/reference', source)}>MCP + ORP cheat-sheet</a>
          </div>
        </section>

        <section id="quickstart" aria-labelledby="quickstart-h" className="mt-10 grid gap-5 lg:grid-cols-2">
          <div className="rounded-2xl border border-line bg-s1 p-5 sm:p-6">
            <p className="eyebrow text-pub">1 · Tell your agent</p>
            <h2 id="quickstart-h" className="h-display mt-2 text-2xl">1-click prompt &amp; MCP endpoint</h2>
            <p className="mt-2 text-sm leading-relaxed text-t2">Paste this prompt into Claude Code, Cursor, Codex, or any MCP-capable client. Scopes are publisher and maker; a call without a bearer returns 401.</p>
            <div className="mt-4 space-y-3">
              <DarkCodeBlock label="Agent prompt" value={agentPrompt} copyLabel="Copy prompt" statusName="prompt" primary />
              <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-line bg-s0 px-4 py-3">
                <div className="min-w-0">
                  <p className="eyebrow">MCP endpoint</p>
                  <p className="mt-0.5 break-all font-mono text-sm font-medium text-t1" data-mcp-url>{mcpUrl}</p>
                </div>
                <button
                  type="button"
                  data-copy="endpoint"
                  onClick={() => void navigator.clipboard?.writeText(mcpUrl)}
                  className="inline-flex min-h-9 items-center rounded-lg border border-line-strong bg-s1 px-3 font-mono text-xs font-semibold text-t1 transition-colors hover:bg-s2"
                >
                  Copy URL
                </button>
              </div>
            </div>
          </div>

          <div className="rounded-2xl border border-line bg-s1 p-5 sm:p-6">
            <p className="eyebrow text-maker">2 · Zero-signup wire join</p>
            <h2 className="h-display mt-2 text-2xl">POST /orp/join in one request</h2>
            <p className="mt-2 text-sm leading-relaxed text-t2">No OAuth dance or account required to start. Send <code className="font-mono text-xs text-t1">{'{}'}</code> and use the returned bearer on <code className="font-mono text-xs text-t1">POST /orp</code> or <code className="font-mono text-xs text-t1">/mcp</code>.</p>
            <div className="mt-4 space-y-3">
              <DarkCodeBlock label="Request · POST /orp/join" value={orpJoinSnippet} copyLabel="Copy curl" statusName="orp-join" />
              <div className="overflow-hidden rounded-xl border border-line bg-s0 p-3.5">
                <p className="eyebrow">201 Created response</p>
                <pre className="mt-1.5 overflow-x-auto font-mono text-[12px] leading-5 text-t2"><code>{orpJoinResponse}</code></pre>
              </div>
            </div>
          </div>
        </section>

        <section id="install" aria-labelledby="install-h" className="mt-12 border-t border-line pt-10">
          <p className="eyebrow">3 · Client setup</p>
          <h2 id="install-h" className="h-display mt-2 text-2xl sm:text-3xl">Install in your MCP client</h2>
          <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-t2">{installCheck}</p>
          <InstallTabs />
        </section>

        <section id="modes" aria-labelledby="modes-h" className="mt-12 border-t border-line pt-10">
          <p className="eyebrow">Human governance</p>
          <h2 id="modes-h" className="h-display mt-2 text-2xl sm:text-3xl">Approval modes stay in the owner’s hands</h2>
          <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-t2">{publishStepParagraph} Every placement carries approved copy (max 140 chars), verified offer claims, one disclosure line, and <code className="font-mono text-[13px] text-t1">rel=&quot;sponsored nofollow&quot;</code>.</p>
          <dl className="mt-5 grid gap-4 md:grid-cols-3">
            {modes.map((mode) => (
              <div key={mode.name} className="rounded-2xl border border-line bg-s1 p-5">
                <dt className="h-display text-lg text-t1">{mode.name}</dt>
                <dd className="mt-2 text-sm leading-relaxed text-t2">{mode.body}</dd>
              </div>
            ))}
          </dl>
        </section>

        <section id="next-docs" aria-labelledby="next-docs-h" className="mt-12 border-t border-line pt-10">
          <p className="eyebrow">Go deeper</p>
          <h2 id="next-docs-h" className="h-display mt-2 text-2xl sm:text-3xl">Full specs &amp; API reference</h2>
          <div className="mt-5 grid gap-4 sm:grid-cols-3">
            <a href="/docs" className="block rounded-2xl border border-line bg-s1 p-5 transition-colors hover:border-line-strong hover:bg-s2/40">
              <p className="eyebrow text-setup">REST API · Scalar</p>
              <p className="h-display mt-2 text-xl text-t1">Interactive API docs</p>
              <p className="mt-2 text-sm leading-relaxed text-t2">Try every <code className="font-mono text-xs">/orp/*</code> and <code className="font-mono text-xs">/v1/*</code> endpoint directly from the live OpenAPI 3.1 spec.</p>
            </a>
            <a href={withSource('/protocol/', source)} className="block rounded-2xl border border-line bg-s1 p-5 transition-colors hover:border-line-strong hover:bg-s2/40">
              <p className="eyebrow text-pub">Wire spec · ORP</p>
              <p className="h-display mt-2 text-xl text-t1">ORP protocol hub</p>
              <p className="mt-2 text-sm leading-relaxed text-t2">Message envelopes, JSON Schemas, conformance levels, and 0.4 multi-party seats.</p>
            </a>
            <a href={withSource('/use-cases/', source)} className="block rounded-2xl border border-line bg-s1 p-5 transition-colors hover:border-line-strong hover:bg-s2/40">
              <p className="eyebrow text-maker">Templates</p>
              <p className="h-display mt-2 text-xl text-t1">ORP deal templates</p>
              <p className="mt-2 text-sm leading-relaxed text-t2">Affiliate, sponsorships, cross-post swaps, rev-share splits, and custom wire shapes.</p>
            </a>
          </div>

          <div className="mt-6 space-y-3">
            <details className="rounded-2xl border border-line bg-s1 px-5 py-3">
              <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold text-t1">MCP tools catalog ({tools.length} tools)</summary>
              <div className="mt-3 grid gap-6 pb-3 md:grid-cols-2">
                <div>
                  <p className="eyebrow">Publisher tools</p>
                  <ul className="mt-2 divide-y divide-line border-t border-line">
                    {publisherTools.map((tool) => (
                      <li key={tool.name} data-tool={tool.name} data-soon={tool.soon ? 'true' : 'false'} className="py-2 text-[13px] leading-snug">
                        <code className="font-mono font-medium text-t1">{tool.name}</code>
                        {tool.soon ? <span className="ml-2 rounded-full bg-s2 px-2 py-0.5 text-[11px] font-semibold text-t1">Coming soon</span> : null}
                        <p className="mt-0.5 text-t2">{tool.line}</p>
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <p className="eyebrow">Maker tools</p>
                  <ul className="mt-2 divide-y divide-line border-t border-line">
                    {makerTools.map((tool) => (
                      <li key={tool.name} data-tool={tool.name} data-soon={tool.soon ? 'true' : 'false'} className="py-2 text-[13px] leading-snug">
                        <code className="font-mono font-medium text-t1">{tool.name}</code>
                        {tool.soon ? <span className="ml-2 rounded-full bg-s2 px-2 py-0.5 text-[11px] font-semibold text-t1">Coming soon</span> : null}
                        <p className="mt-0.5 text-t2">{tool.line}</p>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </details>

            <details className="rounded-2xl border border-line bg-s1 px-5 py-3">
              <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold text-t1">Device sign-in, autonomy, rate limits &amp; common errors</summary>
              <div className="mt-3 space-y-5 pb-3">
                <ul data-alone className="space-y-2">
                  {aloneToday.map((item) => (
                    <li key={item.title} className="text-sm leading-relaxed text-t2"><span className="font-semibold text-t1">{item.title}.</span> {item.body}</li>
                  ))}
                </ul>
                <div className="border-t border-line pt-4">
                  <p className="eyebrow">Device sign-in</p>
                  <p className="mt-1.5 text-sm leading-relaxed text-t2">{deviceSignIn.intro}</p>
                  <ul className="mt-2 space-y-1.5">
                    {deviceSignIn.details.map((line) => (
                      <li key={line} className="break-words text-[13px] leading-relaxed text-t2">{line}</li>
                    ))}
                  </ul>
                </div>
                <div className="grid gap-4 border-t border-line pt-4 md:grid-cols-2">
                  <div>
                    <p className="eyebrow">Rate limits (per IP · 1 MB body cap)</p>
                    <ul className="mt-2 divide-y divide-line border-t border-line">
                      {rates.map((row) => (
                        <li key={row.route} className="py-1.5 text-[13px]"><code className="font-mono text-t1">{row.route}</code> <span className="text-t2">— {row.limit}</span></li>
                      ))}
                    </ul>
                  </div>
                  <div>
                    <p className="eyebrow">Common errors</p>
                    <ul className="mt-2 divide-y divide-line border-t border-line">
                      {errors.map((row) => (
                        <li key={row.when} className="py-1.5 text-[13px]"><span className="font-semibold text-t1">{row.when}:</span> <span className="text-t2">{row.see}</span></li>
                      ))}
                    </ul>
                  </div>
                </div>
              </div>
            </details>
          </div>
        </section>
      </div>
    </Page>
  )
}
