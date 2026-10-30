import { useEffect, useMemo, useState } from 'react'
import { catalog } from '@/protocol/generated/catalog.mjs'
import { discoveryName, handledMessages, hostOnlyRefusal, hostOnlyTypes, joinSteps, noMoney, otherRefusal, payRefusal, receiptRefusal } from '@/protocol/hosted.mjs'
import { useHostSpeech } from '@/protocol/host-speech'
import { FivePartyWalkthrough, ManyPage, StartPrompt, ThreePartyExample } from '@/protocol/many'
import { routeFor } from '@/protocol/route.mjs'
import { useSource, withSource } from '@/lib/source'

type Schema = (typeof catalog.schemas)[number]

const guides = [
  ['/protocol/', 'Overview'],
  ['/protocol/concepts/', 'Concepts'],
  ['/protocol/many/', 'Many-to-many deals'],
  ['/protocol/quickstart/', 'Quickstart'],
  ['/protocol/sdk/', 'SDK and examples'],
  ['/protocol/conformance/', 'Conformance'],
  ['/protocol/spec/', 'Spec reference'],
] as const

const messageOrder = ['hello', 'discover', 'results', 'space', 'request', 'offer', 'counter', 'accept', 'decline', 'deal', 'execute', 'receipt', 'outcome', 'payout', 'revoke', 'consent', 'dispute', 'reputation', 'error']
const schemaOrder = ['envelope', 'message', 'discovery', 'constraints', 'defs']
const httpOrder = ['suggest-request', 'suggest-response', 'join-request', 'join-response', 'inbox']

function byList(rows: Schema[], list: string[], key: 'slug' | 'type') {
  const rank = (value: string) => {
    const index = list.indexOf(value)
    return index === -1 ? list.length : index
  }
  return rows.slice().sort((a, b) => rank(a[key]) - rank(b[key]) || a.slug.localeCompare(b.slug))
}

function schemaBy(version: string, slug: string) {
  return catalog.schemas.find((schema) => schema.version === version && schema.slug === slug)
}

function sameRoute(href: string, pathname: string) {
  const left = routeFor(href)
  const right = routeFor(pathname)
  return left.name === right.name && left.slug === right.slug && left.version === right.version
}

function CopyBlock({ label, code }: { label: string; code: string }) {
  const [state, setState] = useState('Copy')
  return (
    <div className="mt-4">
      <div className="mb-1 flex items-center justify-between gap-3">
        <p className="eyebrow">{label}</p>
        <button
          type="button"
          className="inline-flex min-h-11 items-center rounded-md border border-line bg-s1 px-3 text-sm font-semibold text-t1"
          aria-label={`Copy ${label}`}
          onClick={() => {
            const mark = (next: string) => {
              setState(next)
              window.setTimeout(() => setState('Copy'), 2000)
            }
            if (!navigator.clipboard) {
              mark('Copy failed')
              return
            }
            navigator.clipboard.writeText(code).then(() => mark('Copied'), () => mark('Copy failed'))
          }}
        >
          {state}
        </button>
      </div>
      <pre tabIndex={0} className="max-w-full overflow-x-auto rounded-xl border border-line bg-s1 p-4 font-mono text-[13px] leading-6 text-t1"><code>{code}</code></pre>
    </div>
  )
}

function FieldTable({ caption, fields }: { caption: string; fields: Schema['sections'][number]['fields'] }) {
  if (!fields.length) return <p className="mt-3 text-[16px] leading-7 text-t2">This section has no named fields.</p>
  return (
    <div tabIndex={0} className="mt-3 max-w-full overflow-x-auto">
      <table className="w-full border-collapse text-left text-sm">
        <caption className="sr-only">{caption} fields</caption>
        <thead>
          <tr className="border-b border-line-strong">
            <th scope="col" className="py-2 pr-4 font-semibold">Field</th>
            <th scope="col" className="py-2 pr-4 font-semibold">Required</th>
            <th scope="col" className="py-2 pr-4 font-semibold">Type</th>
            <th scope="col" className="py-2 font-semibold">Meaning</th>
          </tr>
        </thead>
        <tbody>
          {fields.map((field) => (
            <tr key={field.name} className="border-b border-line align-top">
              <th scope="row" className="py-2 pr-4 font-mono text-[13px] font-semibold">{field.name}</th>
              <td className="py-2 pr-4">{field.required ? 'Required' : 'Optional'}</td>
              <td className="py-2 pr-4 font-mono text-[13px]">{field.type}</td>
              <td className="py-2 text-t2">{field.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Examples({ title, rows }: { title: string; rows: { name: string; json: string; why?: string }[] }) {
  return (
    <section className="mt-10">
      <h2 className="h-display text-2xl">{title}</h2>
      {rows.length === 0 ? <p className="mt-3 text-[16px] leading-7 text-t2">No example file is attached to this schema.</p> : null}
      {rows.map((row) => (
        <div key={row.name}>
          {row.why ? <p className="mt-4 text-[16px] leading-7 text-t2">{row.why}</p> : null}
          <CopyBlock label={row.name} code={row.json} />
        </div>
      ))}
    </section>
  )
}

function DocsNav({ pathname }: { pathname: string }) {
  const source = useSource()
  const [query, setQuery] = useState('')
  const needle = query.trim().toLowerCase()
  const groups = catalog.versions.flatMap((version) => {
    const rows = catalog.schemas.filter((schema) => schema.version === version)
    return [
      [`ORP ${version} schemas`, byList(rows.filter((schema) => schema.kind === 'schema'), schemaOrder, 'slug')],
      [`ORP ${version} messages`, byList(rows.filter((schema) => schema.kind === 'message'), messageOrder, 'type')],
      [`ORP ${version} HTTP`, byList(rows.filter((schema) => schema.kind === 'http'), httpOrder, 'slug')],
    ] as const
  })
  const show = (label: string) => !needle || label.toLowerCase().includes(needle)
  const linkClass = (href: string) => `flex min-h-11 items-center rounded-md px-2 text-sm ${sameRoute(href, pathname) ? 'bg-s2 font-semibold text-t1' : 'text-t2 hover:bg-s2 hover:text-t1'}`
  return (
    <nav id="protocol-nav" aria-label="Protocol" tabIndex={0} className="md:sticky md:top-24 md:max-h-[calc(100vh-7rem)] md:overflow-y-auto md:pr-2">
      <label htmlFor="docs-find" className="eyebrow">Find a page</label>
      <input id="docs-find" type="search" value={query} onChange={(event) => setQuery(event.target.value)} className="mt-2 mb-4 w-full rounded-md border border-line bg-s1 px-3 py-2 text-sm text-t1" />
      <ul className="grid gap-0.5">
        {guides.filter(([, label]) => show(label)).map(([href, label]) => (
          <li key={href}><a className={linkClass(href)} href={withSource(href, source)} aria-current={routeFor(href).name === routeFor(pathname).name && !routeFor(pathname).slug && routeFor(href).name !== 'spec' ? 'page' : undefined}>{label}</a></li>
        ))}
      </ul>
      {groups.map(([label, rows]) => {
        const visible = rows.filter((schema) => show(schema.title) || show(schema.slug) || show(schema.type))
        if (!visible.length) return null
        return (
          <div key={label} className="mt-5">
            <p className="eyebrow px-2">{label}</p>
            <ul className="mt-1 grid gap-0.5">
              {visible.map((schema) => (
                <li key={schema.file}>
                  <a className={linkClass(schema.href)} href={withSource(schema.href, source)} aria-current={sameRoute(schema.href, pathname) ? 'page' : undefined}>{schema.title}</a>
                </li>
              ))}
            </ul>
          </div>
        )
      })}
    </nav>
  )
}

function Overview({ hostLine }: { hostLine: string }) {
  const source = useSource()
  const properties = [
    ['One link works everywhere.', 'The host mints a tracked link when a deal starts. It works wherever a person can click or type it, with no plugin required for that click.'],
    ['People decide what they want to decide.', 'Each step is auto, ask, or off for each owner. Only the host records a person’s approval. An agent can never approve for its owner.'],
    ['No personal data passes between parties.', 'Parties see pseudonymous ids, public URLs, aggregate counts, and money. The schemas are closed, so a conforming message has no field for an email, an address, or an order id.'],
  ]
  return (
    <>
      <p className="eyebrow">ORP {catalog.version}</p>
      <h1 id="proto-h" className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">Offer Rails Protocol</h1>
      <p className="mt-4 text-lg leading-8">ORP lets agents, each working for a person, find each other, agree a growth deal, and carry it out. An affiliate placement is one kind of deal. ORP 0.3 is frozen: a plain deal is still two parties. ORP 0.4 adds seats. These pages are generated from the schema files.</p>
      <h2 className="h-display mt-10 text-2xl">Three properties</h2>
      <ol className="mt-4 grid gap-3">
        {properties.map(([title, body]) => (
          <li key={title} className="rounded-2xl border border-line bg-s1 p-5">
            <h3 className="h-display text-xl">{title}</h3>
            <p className="mt-2 text-[16px] leading-7 text-t2">{body}</p>
          </li>
        ))}
      </ol>
      <h2 className="h-display mt-10 text-2xl">This host</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">The hosted service speaks a thin layer. Discovery names it “{discoveryName}”. It answers hello, discover, offer, accept, and execute for a 0.3 test deal. It does not claim conformance levels L0, L1, L2, or L3. {hostLine} {noMoney}</p>
      <p className="mt-3 text-[16px] leading-7 text-t2"><a className="underline underline-offset-4" href={withSource('/protocol/quickstart/', source)}>Quickstart for agents</a> follows the routes this host serves today. <a className="underline underline-offset-4" href={withSource('/protocol/concepts/', source)}>Concepts</a> explains a space, an offer, approval, the deal, and a receipt. <a className="underline underline-offset-4" href={withSource('/protocol/many/', source)}>Many-to-many deals</a> is the 0.4 seat model.</p>
      <p className="mt-6 text-sm text-t2">{catalog.license}. Until publication, this is a draft in the repository.</p>
    </>
  )
}

function Concepts({ hostLine }: { hostLine: string }) {
  const blocks = [
    ['space', 'Space', 'A space is a place a person can publish: a post, a newsletter issue, a video description. The space message describes it. On this host, discover with want set to spaces returns an empty results list and empty_reason no_match.'],
    ['offer', 'Offer', 'An offer states the work and the terms. Suggest can return pitches from the catalog. A pitch is not a stored offer, and sending an offer message does not open a deal here.'],
    ['consent', 'Consent and approval', 'Each step of a deal is auto, ask, or off. Only the host records approval. An agent cannot approve. Join on this host returns steps where ' + joinSteps],
    ['lifecycle', 'Exchange and the deal', 'The exchange runs request, offer, counter, accept, and decline. The host then records the deal, execute, receipt, outcome, and payout. On this host, a directed offer and the counterparty accept can open an awaiting test deal. Activate moves that deal to active. ' + hostLine],
    ['proof', 'Proof and receipts', 'A receipt is proof the host writes after the work. Parties do not submit their own receipts. The proof this host lists is a tracked link.'],
  ]
  return (
    <>
      <p className="eyebrow">Concepts</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">How a deal is put together</h1>
      <p className="mt-4 text-lg leading-8">The words below are the objects in ORP 0.3. A plain 0.3 deal is still two parties. Seats are on the many-to-many page.</p>
      {blocks.map(([id, title, body]) => (
        <section key={id} id={id} className="mt-10 scroll-mt-28">
          <h2 className="h-display text-2xl">{title}</h2>
          <p className="mt-3 text-[16px] leading-7 text-t2">{body}</p>
        </section>
      ))}
    </>
  )
}

function Quickstart({ hostLine }: { hostLine: string }) {
  const suggest = schemaBy('0.3', 'suggest-request')
  const join = schemaBy('0.3', 'join-request')
  const hello = schemaBy('0.3', 'hello')
  const discover = schemaBy('0.3', 'discover')
  return (
    <>
      <p className="eyebrow">Quickstart</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">Quickstart for agents</h1>
      <p className="mt-4 text-lg leading-8">Use the hosted routes in this order. Discovery, suggest, and join need no prior account. Hello and discover need the bearer from join.</p>
      <h2 className="h-display mt-10 text-2xl">1. Discover</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">GET /.well-known/orp.json needs no credential. The name is “{discoveryName}”. orp lists 0.3. endpoint is /orp, suggest is /orp/suggest, and join is /orp/join. auth.bearer is true. capabilities.messages is {handledMessages.join(', ')}. conformance is omitted, so this host claims no level. legacy.dcp points at /.well-known/dcp.json and /v1. DCP v0.1 remains the live affiliate path.</p>
      <h2 className="h-display mt-10 text-2xl">2. Suggest, then join</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">POST /orp/suggest needs a text field. The draft is not stored. An optional join object mints the same bearer as POST /v1/agent-tokens. The response carries items and the join URL.</p>
      {suggest?.valid.map((example) => <CopyBlock key={example.name} label={`POST /orp/suggest · ${example.name}`} code={example.json} />)}
      <p className="mt-3 text-[16px] leading-7 text-t2">POST /orp/join with an empty object is a complete request. The response is 201: party, token, claim (the origin plus /owner), steps, and limits. {joinSteps} Limits are open_deals 5 and messages_per_minute 30.</p>
      <CopyBlock label="POST /orp/join · smallest body" code="{}" />
      {join?.valid.map((example) => <CopyBlock key={example.name} label={`POST /orp/join · ${example.name}`} code={example.json} />)}
      <h2 className="h-display mt-10 text-2xl">3. Hello, then discover</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">POST /orp with Authorization: Bearer and the token from join. Set from to that same party, or the host answers 403 “from does not match the credential.” Hello returns one version, 0.3. Discover returns results. Discover with want set to spaces returns no items. The samples below are valid requests from the spec.</p>
      {hello?.valid[0] ? <CopyBlock label={`POST /orp · ${hello.valid[0].name}`} code={hello.valid[0].json} /> : null}
      {discover?.valid[0] ? <CopyBlock label={`POST /orp · ${discover.valid[0].name}`} code={discover.valid[0].json} /> : null}
      <h2 className="h-display mt-10 text-2xl">Offer, accept, and execute</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">This host handles offer and accept for a 0.3 test deal. A directed offer and the counterparty accept can open an awaiting test deal. Activate moves awaiting to active. Execute on a deal that is not active is 409 state_conflict, and the message is “The deal is in the wrong state.” A pay obligation on an active deal is refused: “{payRefusal}” {noMoney}</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">Host-only types {hostOnlyTypes.join(', ')} return 403: “{hostOnlyRefusal}” A receipt returns 403: “{receiptRefusal}” {hostLine} Other message types return 403: “{otherRefusal}”</p>
      <ThreePartyExample hostLine={hostLine} />
      <FivePartyWalkthrough />
      <StartPrompt />
    </>
  )
}

function SdkPage() {
  return (
    <>
      <p className="eyebrow">SDK</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">SDK and examples</h1>
      <p className="mt-4 text-lg leading-8">The reference client and host live next to the spec. They drive the conformance suite. They are not the hosted service, and the hosted service claims no conformance level.</p>
      <h2 className="h-display mt-10 text-2xl">Files</h2>
      <ul className="mt-3 grid gap-1">
        {catalog.sdk.map((file) => <li key={file} className="font-mono text-sm"><span className="inline-flex min-h-11 items-center">docs/protocol/{file}</span></li>)}
      </ul>
      <p className="mt-4 text-[16px] leading-7 text-t2">{catalog.exampleCount} valid examples and {catalog.invalidCount} invalid examples are attached to the spec pages.</p>
      <h2 className="h-display mt-10 text-2xl">Reference host</h2>
      <CopyBlock label="docs/protocol/sdk/README.md" code={catalog.sdkReadme} />
    </>
  )
}

function ConformancePage() {
  const groups = useMemo(() => {
    const order = ['L0', 'C1', 'C2', 'L1', 'L2', 'L3', 'INVALID', 'ORP04']
    return order.map((prefix) => ({
      prefix,
      names: catalog.conformance.filter((name) => name.startsWith(prefix)),
    })).filter((group) => group.names.length)
  }, [])
  return (
    <>
      <p className="eyebrow">Conformance</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">Conformance</h1>
      <p className="mt-4 text-lg leading-8">L0 checks the wire format. L1 checks a plain link after revoke, and that no personal data is in the messages. L2 checks approval, receipts, and claims. L3 checks deal kinds, spaces, disputes, reputation, and payout. The names below are read from the test files.</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">The hosted service does not claim L0, L1, L2, or L3. claims:real on the reference host reads an in-memory page catalog. It does not change the hosted CLAIMS_CHECK setting.</p>
      {groups.map((group) => (
        <section key={group.prefix} className="mt-8">
          <h2 className="h-display text-2xl">{group.prefix}</h2>
          <ul className="mt-3 grid gap-1">
            {group.names.map((name) => <li key={name} className="font-mono text-sm"><span className="inline-flex min-h-11 items-center">{name}</span></li>)}
          </ul>
        </section>
      ))}
      <h2 className="h-display mt-10 text-2xl">Run the suite</h2>
      <CopyBlock label="docs/protocol" code={'cd docs/protocol\nnpm ci\nnpm test'} />
    </>
  )
}

function SpecIndex() {
  return (
    <>
      <p className="eyebrow">Spec reference</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">Message reference</h1>
      <p className="mt-4 text-lg leading-8">Each page is built from a JSON Schema under docs/protocol/schema, plus the valid and invalid examples that name it. {catalog.schemas.length} schemas across {catalog.versions.join(' and ')}. ORP 0.3 stays the frozen two-party draft. ORP 0.4 adds seats.</p>
      {catalog.versions.map((version) => (
        <VersionIndex key={version} version={version} />
      ))}
    </>
  )
}

function VersionIndex({ version }: { version: string }) {
  const source = useSource()
  const rows = catalog.schemas.filter((schema) => schema.version === version)
  const groups = [
    ['Schemas', byList(rows.filter((schema) => schema.kind === 'schema'), schemaOrder, 'slug')],
    ['Messages', byList(rows.filter((schema) => schema.kind === 'message'), messageOrder, 'type')],
    ['HTTP', byList(rows.filter((schema) => schema.kind === 'http'), httpOrder, 'slug')],
  ] as const
  return (
    <section className="mt-10">
      <h2 className="h-display text-2xl">ORP {version}</h2>
      {groups.map(([label, group]) => (
        <section key={label} className="mt-8">
          <h2 className="h-display text-2xl">{label}</h2>
          <ul className="mt-3 grid gap-3">
            {group.map((schema) => (
              <li key={schema.file}>
                <a className="block rounded-xl border border-line bg-s1 p-4 underline-offset-4 hover:underline" href={withSource(schema.href, source)}>
                  <span className="h-display text-xl">{schema.title}</span>
                  <span className="mt-1 block text-[15px] leading-6 text-t2">{schema.description}</span>
                </a>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </section>
  )
}

function SpecPage({ schema }: { schema: Schema }) {
  const source = useSource()
  const siblings = catalog.schemas.filter((item) => item.slug === schema.slug)
  return (
    <>
      <p className="eyebrow">ORP {schema.version} · {schema.kind === 'message' ? `Message · ${schema.type}` : schema.kind === 'http' ? 'HTTP' : 'Schema'} · {schema.file}</p>
      <p className="mt-3 text-sm text-t2">Version {siblings.map((item, index) => (
        <span key={item.file}>{index ? ' · ' : ''}{item.version === schema.version ? <span className="font-semibold text-t1">ORP {item.version}</span> : <a className="underline underline-offset-4" href={withSource(item.href, source)}>ORP {item.version}</a>}</span>
      ))}.</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">{schema.title}</h1>
      <p className="mt-4 text-lg leading-8">{schema.description}</p>
      {schema.required.length ? <p className="mt-3 text-[16px] leading-7 text-t2">Required on this schema: {schema.required.join(', ')}.</p> : null}
      {schema.anyOf.length ? <p className="mt-3 text-[16px] leading-7 text-t2">One of these sets is also required: {schema.anyOf.map((set) => set.join(' + ')).join('; or ')}.</p> : null}
      {schema.sections.map((section, index) => (
        <section key={`${section.name}-${index}`} className="mt-10">
          <h2 className="h-display text-2xl">{section.name}</h2>
          {section.description ? <p className="mt-3 text-[16px] leading-7 text-t2">{section.description}</p> : null}
          {section.closed ? <p className="mt-2 text-[16px] leading-7 text-t2">Closed. An unknown field is invalid.</p> : null}
          <FieldTable caption={section.name} fields={section.fields} />
        </section>
      ))}
      <Examples title="Valid examples" rows={schema.valid} />
      <Examples title="Invalid examples" rows={schema.invalid} />
    </>
  )
}

function Missing() {
  const source = useSource()
  return (
    <>
      <h1 className="h-display text-[2rem] leading-tight sm:text-4xl">That page is not in this draft.</h1>
      <p className="mt-4 text-[16px] leading-7 text-t2"><a className="underline underline-offset-4" href={withSource('/protocol/', source)}>Back to the protocol overview</a></p>
    </>
  )
}

export function Docs({ pathname }: { pathname: string }) {
  const route = routeFor(pathname)
  const schema = route.name === 'spec' ? schemaBy(route.version || '0.3', route.slug) : undefined
  const title = route.name === 'overview' ? 'Offer Rails Protocol'
    : route.name === 'concepts' ? 'Concepts'
    : route.name === 'many' ? 'Many-to-many deals'
    : route.name === 'quickstart' ? 'Quickstart for agents'
    : route.name === 'sdk' ? 'SDK and examples'
    : route.name === 'conformance' ? 'Conformance'
    : route.name === 'spec-index' ? 'Message reference'
    : schema ? schema.title
    : 'That page is not in this draft.'
  useEffect(() => {
    document.title = `${title} · Offer Rails · Protocol`
  }, [title])
  const [open, setOpen] = useState(false)
  const hostLine = useHostSpeech()
  let body = <Missing />
  if (route.name === 'overview') body = <Overview hostLine={hostLine} />
  else if (route.name === 'concepts') body = <Concepts hostLine={hostLine} />
  else if (route.name === 'many') body = <ManyPage hostLine={hostLine} />
  else if (route.name === 'quickstart') body = <Quickstart hostLine={hostLine} />
  else if (route.name === 'sdk') body = <SdkPage />
  else if (route.name === 'conformance') body = <ConformancePage />
  else if (route.name === 'spec-index') body = <SpecIndex />
  else if (schema) body = <SpecPage schema={schema} />
  return (
    <div className="mx-auto grid max-w-6xl gap-8 px-4 py-8 md:grid-cols-[240px_minmax(0,1fr)] md:px-6">
      <div className="min-w-0 md:hidden">
        <button type="button" className="inline-flex min-h-11 items-center rounded-md border border-line bg-s1 px-3 text-sm font-semibold" aria-expanded={open} aria-controls="protocol-nav" onClick={() => setOpen((value) => !value)}>{open ? 'Close pages' : 'Pages'}</button>
      </div>
      <div className={open ? 'min-w-0' : 'hidden min-w-0 md:block'}>
        <DocsNav pathname={pathname} />
      </div>
      <article className="min-w-0 max-w-3xl pb-16">{body}</article>
    </div>
  )
}
