import { useState } from 'react'
import { agentAccount, agentPrompt, aloneToday, approvalNote, deployNote, deviceSignIn, errors, installCheck, installs, mcpUrl, modes, orderedSteps, rates, signInNote, tools } from '@/agents/facts.mjs'
import { publishStepParagraph } from '@/vision.mjs'
import { Page } from '@/site/Page'

const primaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)] sm:w-auto'
const secondaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s1 px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s2 sm:w-auto'
const sectionClass = 'mt-6 scroll-mt-48 md:scroll-mt-24'

const toc = [
  { href: '#agents-hero', label: 'Hero' },
  { href: '#tell', label: 'Tell your agent' },
  { href: '#install', label: 'Install' },
  { href: '#steps', label: 'Steps' },
  { href: '#device', label: 'Device sign-in' },
  { href: '#alone', label: 'Alone today' },
  { href: '#modes', label: 'Modes' },
  { href: '#tools', label: 'Tools' },
  { href: '#limits', label: 'Limits' },
]

function CopyButton({ value, label, statusName, primary }: { value: string; label: string; statusName: string; primary?: boolean }) {
  const [status, setStatus] = useState('')
  return (
    <div className="mt-3">
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
      <p className="mt-2 min-h-6 text-sm text-t2" role="status" aria-live="polite" data-copy-status={statusName}>{status}</p>
    </div>
  )
}

function InstallTabs() {
  const [active, setActive] = useState(installs[0].id)
  const current = installs.find((item) => item.id === active) ?? installs[0]
  return (
    <div className="mt-4">
      <div role="tablist" aria-label="Install" className="flex flex-wrap gap-2"
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
            className={item.id === active ? 'inline-flex min-h-12 shrink-0 items-center gap-2 whitespace-nowrap rounded-xl border border-line-strong bg-s2 px-4 font-semibold text-t1' : 'inline-flex min-h-12 shrink-0 items-center gap-2 whitespace-nowrap rounded-xl border border-line bg-s1 px-4 font-semibold text-t2'}
            onClick={() => setActive(item.id)}
          >
            <span>{item.label}</span>
            {item.tested ? null : <span className="rounded-full bg-s0 px-2 py-0.5 text-[12px] font-semibold text-t2">· untested</span>}
          </button>
        ))}
      </div>
      <div role="tabpanel" id={`install-panel-${current.id}`} aria-labelledby={`install-tab-${current.id}`} className="mt-4">
        <p className="text-[15px] leading-relaxed text-t1">{current.proof}</p>
        <pre className="mt-3 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{current.snippet}</pre>
        <CopyButton value={current.snippet} label={current.id === 'generic' ? 'Copy config' : `Copy ${current.label}`} statusName={current.id === 'generic' ? 'config' : current.id} />
      </div>
    </div>
  )
}

function DetailLine({ line }: { line: string }) {
  const jsonAt = line.indexOf('{')
  if (jsonAt > 0 && line.trimEnd().endsWith('}')) {
    return (
      <div>
        <p className="font-mono text-xs text-t2">{line.slice(0, jsonAt).trim()}</p>
        <pre className="mt-1 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-xl border border-line bg-s0 p-3 font-mono text-[13px] leading-relaxed text-t1">{line.slice(jsonAt)}</pre>
      </div>
    )
  }
  return <p className="text-[15px] leading-relaxed text-t2">{line}</p>
}

function ToolList({ items }: { items: readonly { name: string; soon: boolean; line: string }[] }) {
  return (
    <ul className="mt-2 divide-y divide-line rounded-2xl border border-line bg-s1">
      {items.map((tool) => (
        <li key={tool.name} data-tool={tool.name} data-soon={tool.soon ? 'true' : 'false'} className="px-3 py-1 text-[13px] leading-tight">
          <code className="font-mono font-medium text-t1">{tool.name}</code>
          {tool.soon ? <span className="ml-2 rounded-full bg-s2 px-2 py-0.5 text-[12px] font-semibold text-t1">Coming soon</span> : null}
          <span className="text-t2"> {tool.line}</span>
        </li>
      ))}
    </ul>
  )
}

function FactList({ caption, rows }: { caption: string; rows: { a: string; b: string }[] }) {
  return (
    <ul className="mt-2 divide-y divide-line rounded-2xl border border-line bg-s1" aria-label={caption}>
      {rows.map((row) => (
        <li key={row.a} className="px-3 py-1 text-[13px] leading-tight">
          <span className="font-mono text-[12px] font-medium text-t1">{row.a}.</span> <span className="text-t2">{row.b}</span>
        </li>
      ))}
    </ul>
  )
}

export function App() {
  const publisherTools = tools.filter((tool) => !tool.maker)
  const makerTools = tools.filter((tool) => tool.maker)
  const signup = orderedSteps[0]
  return (
    <Page page="agents">
      <div className="mx-auto grid max-w-6xl grid-cols-[minmax(0,1fr)] md:grid-cols-[11rem_minmax(0,1fr)] md:gap-8">
        <nav id="on-this-page" aria-label="On this page" className="sticky top-32 z-20 min-w-0 border-b border-line bg-s0/95 px-5 py-2 backdrop-blur md:top-20 md:self-start md:border-0 md:bg-transparent md:px-5 md:py-12">
          <p className="eyebrow mb-3 hidden md:block">On this page</p>
          <div className="flex min-w-0 gap-2 overflow-x-auto md:flex-col md:overflow-visible">
            {toc.map((item) => (
              <a key={item.href} href={item.href} className="inline-flex min-h-11 shrink-0 items-center rounded-full border border-line bg-s1 px-3 text-sm font-semibold text-t1 md:w-full md:justify-start md:rounded-lg md:border-0 md:bg-transparent md:px-2">{item.label}</a>
            ))}
          </div>
        </nav>
        <div className="min-w-0 px-5 pb-8 pt-6 md:pt-12">
          <section id="agents-hero" aria-labelledby="agents-h" className={sectionClass}>
            <p className="eyebrow">MCP server</p>
            <h1 id="agents-h" className="h-display mt-3 text-[40px] sm:text-6xl">Find an offer, preview the page, place it.</h1>
            <p className="mt-4 text-lg leading-relaxed text-t2">Allagio is an MCP server. Three steps. {publishStepParagraph} A chat answer is never a paid placement.</p>
            <p className="mt-4 break-all font-mono text-[15px] text-t1" data-mcp-url>{mcpUrl}</p>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">Scopes are publisher and maker. A call with no bearer is 401.</p>
            <CopyButton value={mcpUrl} label="Copy endpoint" statusName="endpoint" primary />
          </section>

          <section id="tell" aria-labelledby="tell-h" className={sectionClass}>
            <h2 id="tell-h" className="h-display text-2xl sm:text-3xl">Tell your agent</h2>
            <pre className="mt-3 max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-2xl border border-line bg-s1 p-4 font-mono text-[13px] leading-relaxed text-t1">{agentPrompt}</pre>
            <CopyButton value={agentPrompt} label="Copy prompt" statusName="prompt" />
          </section>

          <section id="install" aria-labelledby="install-h" className={sectionClass}>
            <h2 id="install-h" className="h-display text-2xl sm:text-3xl">Install</h2>
            <InstallTabs />
            <p className="mt-3 text-[15px] leading-relaxed text-t2">{installCheck}</p>
            <p className="text-[15px] leading-relaxed text-t2">{signInNote}</p>
          </section>

          <section id="steps" data-d063-steps aria-labelledby="steps-h" className={sectionClass}>
            <h2 id="steps-h" className="h-display text-2xl sm:text-3xl">Ordered agent steps</h2>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">{deployNote}</p>
            <ol className="mt-4 divide-y divide-line border-y border-line">
              {orderedSteps.map((step) => (
                <li key={step.n} className="py-1">
                  <p className="text-[14px] leading-snug text-t1"><span className="font-semibold">{step.n}. {step.title}.</span> {step.line}</p>
                  {step.n === 5 ? <p className="text-[15px] leading-relaxed text-t1">{approvalNote}</p> : null}
                  {step.n === 4 || step.n === 7 || step.n === 10 ? (
                    <details className="mt-1">
                      <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold">What the server expects</summary>
                      <p className="pb-2 text-[15px] leading-relaxed text-t2">{step.body}</p>
                    </details>
                  ) : null}
                  {step.n === 1 ? (
                    <details className="mt-1">
                      <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold">Publisher and maker 201 fields</summary>
                      <div className="space-y-3 pb-2">
                        {(signup.details || []).map((line) => <DetailLine key={line} line={line} />)}
                        <DetailLine line={`Publisher request: ${agentAccount.publisherRequest}`} />
                        <DetailLine line={`Publisher 201 JSON: ${agentAccount.publisherResponse}`} />
                        <DetailLine line={`Maker request: ${agentAccount.makerRequest}`} />
                        <DetailLine line={`Maker 201 JSON: ${agentAccount.makerResponse}`} />
                        <p className="text-[15px] leading-relaxed text-t2">{agentAccount.rules}</p>
                      </div>
                    </details>
                  ) : null}
                </li>
              ))}
            </ol>
          </section>

          <section id="device" aria-labelledby="device-h" className={sectionClass}>
            <h2 id="device-h" className="h-display text-2xl sm:text-3xl">Device sign-in</h2>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">{deviceSignIn.intro}</p>
            <ul className="mt-3 space-y-2">
              {deviceSignIn.points.map((point) => (
                <li key={point.text || point.before} className="break-words text-[14px] leading-snug text-t2">
                  {point.link ? <>{point.before}<a href={point.link.href} className="inline-flex min-h-11 items-center px-1 underline underline-offset-4">{point.link.label}</a>{point.after}</> : point.text}
                </li>
              ))}
            </ul>
            <details className="mt-2">
              <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold">Return path, CSRF, and Origin</summary>
              <ul className="space-y-2 pb-2">
                {deviceSignIn.details.map((line) => <li key={line} className="break-words text-[15px] leading-relaxed text-t2">{line}</li>)}
              </ul>
            </details>
          </section>

          <section id="alone" aria-labelledby="alone-h" className={sectionClass}>
            <h2 id="alone-h" className="h-display text-2xl sm:text-3xl">What an agent can do alone today</h2>
            <ul data-alone className="mt-3 space-y-2">
              {aloneToday.map((item) => (
                <li key={item.title} className="text-[14px] leading-snug text-t2"><span className="font-semibold text-t1">{item.title}.</span> {item.body}</li>
              ))}
            </ul>
          </section>

          <section id="modes" aria-labelledby="modes-h" className={sectionClass}>
            <h2 id="modes-h" className="h-display text-2xl sm:text-3xl">What happens after the place call</h2>
            <p className="mt-3 text-[15px] leading-relaxed text-t2">{publishStepParagraph}</p>
            <dl className="mt-3 space-y-2">
              {modes.map((mode) => (
                <div key={mode.name}>
                  <dt className="font-semibold text-t1">{mode.name}</dt>
                  <dd className="text-[14px] leading-snug text-t2">{mode.body}</dd>
                </div>
              ))}
            </dl>
            <p className="mt-3 text-[15px] leading-relaxed text-t2">Every placement carries approved copy of at most 140 characters, claims only from the offer, one disclosure line, and rel=&quot;sponsored nofollow&quot;.</p>
          </section>

          <section id="tools" aria-labelledby="tools-h" className={sectionClass}>
            <h2 id="tools-h" className="h-display text-2xl sm:text-3xl">Tools</h2>
            <ToolList items={publisherTools} />
            <h3 className="mt-4 font-display text-lg font-bold">Maker tools</h3>
            <ToolList items={makerTools} />
            <p className="mt-4 text-[15px] leading-snug text-t2">
              Agents can read the rules before they deal: <a className="font-semibold underline underline-offset-4 hover:no-underline" href="/orp/spec">the protocol text</a>, the published <a className="font-semibold underline underline-offset-4 hover:no-underline" href="/orp/schemas">JSON schemas</a> every message validates against, and the <a className="font-semibold underline underline-offset-4 hover:no-underline" href="/openapi.json">OpenAPI document</a> for the HTTP surface.
            </p>
          </section>

          <section id="limits" aria-labelledby="limits-h" className={sectionClass}>
            <h2 id="limits-h" className="h-display text-2xl sm:text-3xl">Rate limits and errors</h2>
            <p className="mt-2 text-[15px] leading-snug text-t2">Per client IP. The /mcp body cap is 1 MB. A request whose Origin is not the Allagio base URL gets 403 origin not allowed. On 429, wait a minute and do not retry in a loop.</p>
            <FactList caption="Rate limits" rows={rates.map((row) => ({ a: row.route, b: `${row.limit}. ${row.over}` }))} />
            <h3 className="mt-4 font-display text-lg font-bold">Errors agents see</h3>
            <FactList caption="Error messages" rows={errors.map((row) => ({ a: row.when, b: row.see }))} />
          </section>
        </div>
      </div>
    </Page>
  )
}
