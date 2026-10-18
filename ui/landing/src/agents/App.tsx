import { useState } from 'react'
import { agentAccount, agentPrompt, aloneToday, cursorInstallUrl, deviceSteps, errors, installs, mcpUrl, modes, rates, signInNote, steps, tools } from '@/agents/facts.mjs'
import { Page } from '@/site/Page'

const primaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)] sm:w-auto'
const secondaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s1 px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s2 sm:w-auto'

function CopyButton({ value, label, statusName, primary }: { value: string; label: string; statusName: string; primary?: boolean }) {
  const [status, setStatus] = useState('')
  return (
    <div className="mt-4">
      <button
        type="button"
        className={primary ? primaryClass : secondaryClass}
        data-copy={statusName}
        {...(primary ? { 'data-primary-action': true } : {})}
        onClick={async () => {
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
            setStatus(ok ? 'Copied' : 'Select the text and copy it.')
          }
        }}
      >{label}</button>
      <p className="mt-2 min-h-8 text-sm text-t2" role="status" aria-live="polite" data-copy-status={statusName}>{status}</p>
    </div>
  )
}

function InstallTabs() {
  const [active, setActive] = useState(installs[0].id)
  const current = installs.find((item) => item.id === active) ?? installs[0]
  return (
    <div className="mt-4">
      <div role="tablist" aria-label="Install" className="flex max-w-full gap-2 overflow-x-auto"
        onKeyDown={(event) => {
          const names = ['ArrowRight', 'ArrowLeft']
          if (!names.includes(event.key)) return
          event.preventDefault()
          const index = installs.findIndex((item) => item.id === active)
          const next = event.key === 'ArrowRight' ? (index + 1) % installs.length : (index - 1 + installs.length) % installs.length
          setActive(installs[next].id)
        }}
      >
        {installs.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            id={`install-tab-${item.id}`}
            aria-selected={item.id === active}
            aria-controls={`install-panel-${item.id}`}
            className={item.id === active ? 'inline-flex min-h-12 shrink-0 items-center rounded-xl border border-line-strong bg-s2 px-4 font-semibold text-t1' : 'inline-flex min-h-12 shrink-0 items-center rounded-xl border border-line bg-s1 px-4 font-semibold text-t2'}
            onClick={() => setActive(item.id)}
          >{item.label}</button>
        ))}
      </div>
      <div role="tabpanel" id={`install-panel-${current.id}`} aria-labelledby={`install-tab-${current.id}`} className="mt-4">
        <pre className="max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{current.snippet}</pre>
        <CopyButton value={current.snippet} label={current.id === 'generic' ? 'Copy config' : `Copy ${current.label}`} statusName={current.id === 'generic' ? 'config' : current.id} />
        {current.id === 'cursor' ? <a className={secondaryClass} href={cursorInstallUrl} data-cursor-install>Add to Cursor</a> : null}
      </div>
    </div>
  )
}

function GridTable({ caption, columns, rows }: { caption: string; columns: [string, string]; rows: { a: string; b: string }[] }) {
  return (
    <div className="mt-4 max-w-full overflow-x-auto rounded-2xl border border-line bg-s1">
      <table className="w-full table-fixed text-left text-sm">
        <caption className="sr-only">{caption}</caption>
        <thead className="bg-s2 text-t1">
          <tr>
            <th scope="col" className="w-[38%] px-3 py-3 font-semibold">{columns[0]}</th>
            <th scope="col" className="px-3 py-3 font-semibold">{columns[1]}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.a} className="border-t border-line align-top">
              <th scope="row" className="break-words px-3 py-3 font-mono text-[13px] font-medium">{row.a}</th>
              <td className="break-words px-3 py-3 text-t1">{row.b}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function App() {
  return (
    <Page page="agents">
      <section aria-labelledby="agents-h" className="mx-auto max-w-3xl px-5 pb-16 pt-12 md:pt-16">
        <p className="eyebrow">MCP server</p>
        <h1 id="agents-h" className="h-display mt-4 text-[40px] sm:text-6xl">Find an offer, preview the page, place it.</h1>
        <p className="mt-5 text-lg leading-relaxed text-t2">Offer Rails is an MCP server. Three steps. A person still approves what gets published. A chat answer is never itself an ad.</p>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Tell your agent</h2>
        <pre className="mt-4 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentPrompt}</pre>
        <CopyButton value={agentPrompt} label="Copy prompt" statusName="prompt" />

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Install</h2>
        <InstallTabs />
        <p className="mt-4 text-[15px] leading-relaxed text-t2">{signInNote}</p>

        <ol data-agent-steps className="mt-10 space-y-4">
          {steps.map((step, index) => (
            <li key={step.title} className="rounded-2xl border border-line bg-s1 p-5">
              <p className="font-mono text-xs text-t2">Step {index + 1}</p>
              <h2 className="mt-2 font-display text-2xl font-bold">{step.title}</h2>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">{step.body}</p>
            </li>
          ))}
        </ol>

        {/* D-063 ordered agent steps. Do not add steps in this spot. */}
        <section data-d063-steps aria-labelledby="d063-steps" className="mt-12 rounded-2xl border border-dashed border-line p-5">
          <h2 id="d063-steps" className="font-display text-2xl font-bold">Ordered agent steps</h2>
          <p className="mt-2 text-[15px] leading-relaxed text-t2">Reserved. These steps are not written on this page.</p>
        </section>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">What an agent can do alone today</h2>
        <ul data-alone className="mt-6 space-y-3">
          {aloneToday.map((item) => (
            <li key={item.title} className="rounded-2xl border border-line bg-s1 p-5">
              <h3 className="font-display text-lg font-bold">{item.title}</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">{item.body}</p>
            </li>
          ))}
        </ul>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">No-browser account</h2>
        <p className="mt-3 text-[15px] leading-relaxed text-t2">POST /v1/agent-accounts creates an account with no browser. {agentAccount.rules}</p>
        <p className="mt-4 font-mono text-xs text-t2">Publisher request</p>
        <pre className="mt-2 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentAccount.publisherRequest}</pre>
        <p className="mt-4 font-mono text-xs text-t2">Publisher 201</p>
        <pre className="mt-2 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentAccount.publisherResponse}</pre>
        <p className="mt-4 font-mono text-xs text-t2">Maker request</p>
        <pre className="mt-2 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentAccount.makerRequest}</pre>
        <p className="mt-4 font-mono text-xs text-t2">Maker 201</p>
        <pre className="mt-2 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentAccount.makerResponse}</pre>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Device code</h2>
        <ol data-device-steps className="mt-6 space-y-3">
          {deviceSteps.map((step, index) => (
            <li key={step.title} className="rounded-2xl border border-line bg-s1 p-5">
              <p className="font-mono text-xs text-t2">Step {index + 1}</p>
              <h3 className="mt-2 font-display text-lg font-bold">{step.title}</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">{step.body}</p>
            </li>
          ))}
        </ol>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">What happens after the place call</h2>
        <dl className="mt-6 grid gap-3">
          {modes.map((mode) => (
            <div key={mode.name} className="rounded-2xl border border-line bg-s1 p-5">
              <dt className="font-display text-lg font-bold">{mode.name}</dt>
              <dd className="mt-2 text-[15px] leading-relaxed text-t2">{mode.body}</dd>
            </div>
          ))}
        </dl>
        <p className="mt-4 text-[15px] leading-relaxed text-t2">Every placement carries approved copy of at most 140 characters, claims only from the offer, one disclosure line, and rel=&quot;sponsored nofollow&quot;.</p>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">MCP endpoint</h2>
        <p className="mt-3 break-all font-mono text-[15px] text-t1" data-mcp-url>{mcpUrl}</p>
        <p className="mt-3 text-[15px] leading-relaxed text-t2">Scopes are publisher and maker. A call with no bearer is 401. {signInNote}</p>
        <CopyButton value={mcpUrl} label="Copy endpoint" statusName="endpoint" primary />

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Tools</h2>
        <ul className="mt-4 space-y-3">
          {tools.map((tool) => (
            <li key={tool.name} data-tool={tool.name} data-soon={tool.soon ? 'true' : 'false'} className="rounded-2xl border border-line bg-s1 p-4">
              <div className="flex flex-wrap items-center gap-2">
                <code className="font-mono text-[15px] font-medium">{tool.name}</code>
                {tool.soon ? <span className="inline-flex min-h-11 items-center rounded-full bg-s2 px-3 text-[13px] font-semibold text-t1">Coming soon</span> : null}
              </div>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">{tool.line}</p>
            </li>
          ))}
        </ul>
        <p className="mt-4 text-[15px] leading-relaxed text-t2">A maker grant adds the maker verbs (register_product through submit_offer, and get_earnings).</p>

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Rate limits</h2>
        <p className="mt-3 text-[15px] leading-relaxed text-t2">Per client IP. The /mcp body cap is 1 MB. A request whose Origin is not the Offer Rails base URL gets 403 origin not allowed. On 429, wait a minute and do not retry in a loop.</p>
        <GridTable caption="Rate limits" columns={['Route', 'Limit and over-limit answer']} rows={rates.map((row) => ({ a: row.route, b: `${row.limit}. ${row.over}` }))} />

        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Errors agents see</h2>
        <GridTable caption="Error messages" columns={['When', 'Exact text']} rows={errors.map((row) => ({ a: row.when, b: row.see }))} />
      </section>
    </Page>
  )
}
