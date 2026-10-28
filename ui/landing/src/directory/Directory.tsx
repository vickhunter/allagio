import { useEffect, useRef, useState } from 'react'
import { Page } from '@/site/Page'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useSource, withSource } from '@/lib/source'
import { DirectoryFailure, getMember, httpsUrl, listDirectory, logoSrc, memberRoute, wantsFixtures } from '@/directory/client.mjs'
import { directoryChrome, directoryFailureCopy } from '@/directory/state.mjs'
import { agentPrompt, installLead, installUrl, makerPrompt } from '@/directory/prompt.mjs'
import type { DirectoryCard, DirectoryKind } from '@/directory/types'

type View = 'loading' | 'empty' | 'nomatch' | 'results' | 'error'
type Query = { q: string; category: string; topic: string; lang: string }

function readQuery(): Query {
  const params = new URLSearchParams(window.location.search)
  return {
    q: params.get('q') || '',
    category: params.get('category') || '',
    topic: params.get('topic') || '',
    lang: params.get('lang') || '',
  }
}

function filtered(kind: DirectoryKind, query: Query) {
  return Boolean(query.q || query.lang || (kind === 'maker' ? query.category : query.topic))
}

function countLine(kind: DirectoryKind, item: DirectoryCard) {
  if (kind === 'maker') {
    const count = item.open_offers ?? 0
    return `${count} open ${count === 1 ? 'offer' : 'offers'}`
  }
  const count = item.monthly_pages ?? 0
  return `${count} ${count === 1 ? 'page' : 'pages'} a month`
}

function profileHref(kind: DirectoryKind, slug: string, source: string) {
  const base = kind === 'maker' ? '/makers/' : '/publishers/'
  const href = base + encodeURIComponent(slug) + (wantsFixtures() ? '?directoryFixtures=1' : '')
  return withSource(href, source)
}

function Mark({ name, logo }: { name: string; logo: string | null }) {
  const [failed, setFailed] = useState(false)
  const letter = Array.from(name.trim())[0]?.toUpperCase() || '·'
  if (!logo || failed) {
    return <span aria-hidden className="grid size-12 shrink-0 place-items-center rounded-xl border border-line bg-s2 font-display text-lg text-t1">{letter}</span>
  }
  return <img src={logo} alt="" width={48} height={48} className="size-12 shrink-0 rounded-xl border border-line object-cover" onError={() => setFailed(true)} />
}

function AddYours({ source, kind, level = 'h3' }: { source: string; kind: DirectoryKind; level?: 'h2' | 'h3' }) {
  const [copied, setCopied] = useState(false)
  const prompt = kind === 'maker' ? makerPrompt : agentPrompt
  const heading = kind === 'maker' ? 'Add your product' : 'Add your site or newsletter'
  const lead = kind === 'maker' ? 'Add my product to the Offer Rails directory\n' : 'Add my site or newsletter to the Offer Rails directory\n'
  const Title = level === 'h2' ? 'h2' : 'h3'
  return (
    <div id="add-yours" className="mt-8 max-w-3xl rounded-2xl border border-line bg-s1 p-5">
      <Title className="font-display text-lg font-semibold">{heading}</Title>
      {kind === 'maker' ? (
        <p className="mt-2 text-sm leading-relaxed text-t2">This needs an MCP client. There is no add-product form on this page. Paste this to Claude Code. The same command is in the install list above, one command per client.</p>
      ) : (
        <p className="mt-2 text-sm leading-relaxed text-t2">Agent only for now. There is no form here that saves a newsletter or a site, and this page does not mint a link. Paste this to an MCP client. Your agent prepares a placement. You approve it, then paste the tracked link.</p>
      )}
      <pre className="mt-4 min-w-0 whitespace-pre-wrap break-normal rounded-xl border border-line bg-s0 p-4 font-mono text-sm leading-relaxed text-t1">{lead}{installLead}<span className="[overflow-wrap:anywhere]">{installUrl}</span></pre>
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Button type="button" variant="outline" onClick={() => {
          navigator.clipboard.writeText(prompt).then(() => setCopied(true)).catch(() => setCopied(false))
        }}>Copy prompt</Button>
        <a className="inline-flex min-h-11 min-w-11 items-center underline underline-offset-4 hover:underline" href={withSource('/agents', source)}>Agents</a>
      </div>
      <p role="status" aria-live="polite" className="mt-3 min-h-6 text-sm text-t1">{copied ? 'Copied' : ''}</p>
    </div>
  )
}

function MemberCard({ kind, item, source }: { kind: DirectoryKind; item: DirectoryCard; source: string }) {
  const groups = kind === 'maker' ? item.categories || [] : item.topics || []
  return (
    <Card data-member className="p-5">
      <a href={profileHref(kind, item.slug, source)} className="flex min-h-11 min-w-0 flex-col gap-3">
        <span className="flex items-start gap-3">
          <Mark name={item.name} logo={logoSrc(item.logo_url)} />
          <span className="min-w-0">
            <span className="flex flex-wrap items-center gap-2">
              <h3 className="font-display text-lg font-semibold">{item.name}</h3>
              {item.verified ? <Badge variant="neutral">Verified</Badge> : null}
            </span>
            <span className="mt-1 block text-sm leading-relaxed text-t2">{item.summary}</span>
          </span>
        </span>
        <span className="grid gap-1 text-sm">
          <span><span className="text-t2">Languages </span>{item.languages.join(', ')}</span>
          <span><span className="text-t2">{kind === 'maker' ? 'Categories ' : 'Topics '}</span>{groups.join(', ')}</span>
          <span>{countLine(kind, item)}</span>
        </span>
      </a>
    </Card>
  )
}

export function DirectorySection({ kind }: { kind: DirectoryKind }) {
  const source = useSource()
  const preview = wantsFixtures()
  const [draft, setDraft] = useState<Query>(readQuery)
  const [view, setView] = useState<View>('loading')
  const [items, setItems] = useState<DirectoryCard[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [total, setTotal] = useState(0)
  const [failure, setFailure] = useState<DirectoryFailure | null>(null)
  const ticket = useRef(0)
  const applied = useRef<Query>(readQuery())
  const shown = useRef<DirectoryCard[]>([])
  const facet = kind === 'maker' ? 'Category' : 'Topic'
  const facetKey = kind === 'maker' ? 'category' : 'topic'

  function load(query: Query, next: string, append: boolean) {
    const used = append ? applied.current : query
    if (!append) applied.current = used
    const id = ++ticket.current
    if (!append) {
      setView('loading')
      setFailure(null)
    }
    const limit = wantsFixtures() ? 1 : 24
    listDirectory(kind, { q: used.q, lang: used.lang, cursor: next, limit, [facetKey]: used[facetKey] }).then((page) => {
      if (id !== ticket.current) return
      const rows = append ? shown.current.concat(page.items) : page.items
      shown.current = rows
      setItems(rows)
      setCursor(page.next_cursor)
      setTotal(page.total)
      setFailure(null)
      setView(!rows.length && filtered(kind, used) ? 'nomatch' : rows.length ? 'results' : 'empty')
    }).catch((err) => {
      if (id !== ticket.current || append) return
      shown.current = []
      setItems([])
      setCursor(null)
      setTotal(0)
      setFailure(err instanceof DirectoryFailure ? err : new DirectoryFailure(0, 'network'))
      setView('error')
    })
  }

  useEffect(() => { load(readQuery(), '', false) }, [kind])

  const chrome = directoryChrome(view, total)
  return (
    <section id="directory" aria-labelledby={chrome.heading ? 'directory-h' : undefined} aria-label={chrome.heading ? undefined : 'Directory'} data-directory-view={view} data-directory-kind={kind} className="border-t border-line">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        {chrome.heading ? (
          <>
            <h2 id="directory-h" className="h-display text-3xl sm:text-4xl">{kind === 'maker' ? 'Makers on Offer Rails' : 'Publishers on Offer Rails'}</h2>
            <p className="mt-4 max-w-2xl text-lg leading-relaxed text-t2">{kind === 'maker' ? 'Makers add a product through their AI agent. Each maker is listed by its owner.' : 'Sites add themselves through their AI agent. Each one is listed by its owner.'}</p>
          </>
        ) : null}
        {preview && chrome.heading ? <p className="mt-4 inline-flex min-h-11 items-center rounded-md border border-line-strong bg-s2 px-3 font-mono text-sm text-t1">TEST data</p> : null}
        {chrome.search ? (
          <form className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_auto] lg:items-end" onSubmit={(event) => { event.preventDefault(); load(draft, '', false) }}>
            <label className="block text-sm font-semibold" htmlFor="directory-q">Search
              <Input id="directory-q" className="mt-1 font-normal" name="q" value={draft.q} onChange={(event) => setDraft({ ...draft, q: event.target.value })} />
            </label>
            <label className="block text-sm font-semibold" htmlFor="directory-facet">{facet}
              <Input id="directory-facet" className="mt-1 font-normal" name={facetKey} value={draft[facetKey]} onChange={(event) => setDraft({ ...draft, [facetKey]: event.target.value })} />
            </label>
            <label className="block text-sm font-semibold" htmlFor="directory-lang">Language
              <Input id="directory-lang" className="mt-1 font-normal" name="lang" value={draft.lang} onChange={(event) => setDraft({ ...draft, lang: event.target.value })} />
            </label>
            <Button type="submit" variant="outline">Search</Button>
          </form>
        ) : null}
        {view === 'loading' ? <p className="mt-8 text-t2" role="status">Loading the directory.</p> : null}
        {chrome.error ? <p className="mt-8 text-t1" role="status">{directoryFailureCopy(failure)}</p> : null}
        {chrome.nomatch ? <p className="mt-8 text-t1" role="status">{kind === 'maker' ? 'No makers match this search.' : 'No sites match this search.'}</p> : null}
        {chrome.list ? (
          <>
            <p className="mt-8 text-sm text-t2">{total} {kind === 'maker' ? (total === 1 ? 'maker' : 'makers') : (total === 1 ? 'site' : 'sites')}</p>
            <ul className="mt-4 grid gap-3 sm:grid-cols-2">
              {items.map((item) => <li key={item.id}><MemberCard kind={kind} item={item} source={source} /></li>)}
            </ul>
            {cursor ? <Button type="button" variant="outline" className="mt-4" onClick={() => load(applied.current, cursor, true)}>Load more</Button> : null}
          </>
        ) : null}
        {chrome.add ? <AddYours source={source} kind={kind} level={chrome.heading ? 'h3' : 'h2'} /> : null}
      </div>
    </section>
  )
}

function MissingProfile({ source }: { source: string }) {
  return (
    <Page page="missing">
      <section data-not-found className="mx-auto max-w-3xl px-5 pb-20 pt-16">
        <h1 className="h-display text-4xl sm:text-6xl">That page is not on Offer Rails.</h1>
        <p className="mt-4 text-lg leading-relaxed text-t2">The address does not match a page on this site.</p>
        <p className="mt-8 flex flex-wrap gap-3">
          <a className="inline-flex min-h-11 min-w-11 items-center rounded-xl bg-t1 px-5 font-semibold text-s0" href={withSource('/', source)}>Home</a>
          <a className="inline-flex min-h-11 min-w-11 items-center rounded-xl border border-line-strong px-5 font-semibold" href={withSource('/agents', source)}>Agents</a>
        </p>
      </section>
    </Page>
  )
}

export function DirectoryDetail() {
  const source = useSource()
  const [route] = useState(() => memberRoute(window.location.pathname))
  const [member, setMember] = useState<DirectoryCard | null>(null)
  const [missing, setMissing] = useState(false)

  useEffect(() => {
    if (!route) {
      setMissing(true)
      document.title = 'Offer Rails · Page not found'
      return
    }
    let gone = false
    getMember(route.kind, route.slug).then((item) => {
      if (gone) return
      setMember(item)
      document.title = `Offer Rails · ${item.name}`
    }).catch(() => {
      if (gone) return
      setMissing(true)
      document.title = 'Offer Rails · Page not found'
    })
    return () => { gone = true }
  }, [route])

  if (missing || !route) return <MissingProfile source={source} />
  if (!member) {
    return (
      <Page page="missing">
        <p role="status" className="mx-auto max-w-6xl px-5 py-16 text-t2">Loading the directory.</p>
      </Page>
    )
  }
  const groups = member.kind === 'maker' ? member.categories || [] : member.topics || []
  const site = httpsUrl(member.url)
  return (
    <Page page={member.kind === 'publisher' ? 'publishers' : 'makers'}>
      <article data-directory-detail className="mx-auto max-w-3xl px-5 py-16 md:py-20">
        <p className="eyebrow">{member.kind === 'maker' ? 'Maker' : 'Publisher'}</p>
        <div className="mt-4 flex items-start gap-4">
          <Mark name={member.name} logo={logoSrc(member.logo_url)} />
          <div className="min-w-0">
            <h1 className="h-display break-words text-4xl sm:text-5xl">{member.name}</h1>
            {member.verified ? <Badge variant="neutral" className="mt-3">Verified</Badge> : null}
          </div>
        </div>
        <p className="mt-6 text-lg leading-relaxed text-t2">{member.summary}</p>
        <p className="mt-4 text-sm"><span className="text-t2">Languages </span>{member.languages.join(', ')}</p>
        <p className="mt-1 text-sm"><span className="text-t2">{member.kind === 'maker' ? 'Categories ' : 'Topics '}</span>{groups.join(', ')}</p>
        <p className="mt-1 text-sm">{countLine(member.kind, member)}</p>
        {site ? <p className="mt-4 break-all text-sm"><a className="inline-flex min-h-11 items-center underline underline-offset-4" href={site}>{site}</a></p> : null}
      </article>
    </Page>
  )
}
