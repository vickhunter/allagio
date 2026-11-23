import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { FileText, Mail, ShieldCheck, Sparkles, Star } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { roleReturn, signInHref, useSource } from '@/lib/source'
import { PreviewStatus, barCount, codeFromStatus, fixtureBody, previewTimeoutMs, rateLimitPauseMs, readPreviewFixture, slotAfterBar } from '@/lib/preview-map.mjs'
import { adaptPreview, groupsFixtureBody, includeUnverifiedFixture, readNotifyIntent, storeNotifyIntent, type PreviewGroups, type ReadFailure, type UnverifiedProgram, type VerifiedOffer } from '@/lib/preview-groups.mjs'
import { useLocale } from '@/i18n'
import type { ErrorCopy, ShowcaseId } from '@/i18n/messages'
import { PlatformPostFrame, platformFromHostname } from '@/components/platform-frames'

// Signed-out preview. The URL stays in the field. previewFixture skips the network.
type Phase = 'idle' | 'loading' | 'ready' | 'unread' | 'error'

interface ExampleChip {
  id: string
  label: string
  url: string
  platform: ShowcaseId
  icon: typeof FileText
  body: Record<string, unknown>
}

const EXAMPLE_CHIPS: ExampleChip[] = [
  {
    id: 'dev-blog',
    label: 'Dev blog post',
    url: 'https://priya.dev/caching-the-dependency-graph',
    platform: 'blog',
    icon: FileText,
    body: {
      title: 'Caching the dependency graph in CI',
      author: 'Priya · priya.dev',
      excerpt: 'We cut monorepo build times from 14 minutes to 4 by hashing lockfile layers and persisting task outputs across runners.',
      slot: 'after_intro',
      demo: true,
      offer: {
        verified: true,
        name: 'Acme Deploy',
        summary: 'One-click branch previews with remote build graph caching — starts at €0/mo.',
      },
      suggestion: {
        kind: 'partner_mention',
        name: 'Acme Deploy',
        line: 'Sponsored · Acme Deploy — remote build graph caching and branch previews in 12 seconds.',
        note: '20% recurring · matched on CI caching & monorepo topics',
      },
    },
  },
  {
    id: 'newsletter',
    label: 'Newsletter issue',
    url: 'https://designweekly.substack.com/p/border-radius-debate',
    platform: 'newsletter',
    icon: Mail,
    body: {
      title: 'Issue #214: The border-radius debate',
      author: 'Design Weekly',
      excerpt: 'Why modern dashboards converged on the same corner geometry, plus three crisp reads for your Friday.',
      slot: 'mid_article',
      demo: true,
      offer: {
        verified: true,
        name: 'Frontend Notes',
        summary: 'Reciprocal newsletter swap — weekly CSS architecture and design systems notes for 14k readers.',
      },
      suggestion: {
        kind: 'partner_mention',
        name: 'Frontend Notes',
        line: 'Recommended partner · Frontend Notes — deep dives on CSS systems and UI craft.',
        note: '1:1 reciprocal swap · €0 moved · verified archive proof',
      },
    },
  },
  {
    id: 'product-review',
    label: 'Product review',
    url: 'https://deskgram.example/reviews/lumen-desk-lamp',
    platform: 'blog',
    icon: Star,
    body: {
      title: 'Lumen Desk Lamp review: six months on a small desk',
      author: 'Deskgram Studio',
      excerpt: 'Warm 3000K glare-free task lighting with a machined aluminum clamp that clears room for two monitors.',
      slot: 'before_conclusion',
      demo: true,
      offer: {
        verified: true,
        name: 'Lumen Studio',
        summary: 'Warm architectural task lighting built for compact home offices — 8% per sale.',
      },
      suggestion: {
        kind: 'partner_mention',
        name: 'Lumen Studio',
        line: 'Sponsored · Lumen Desk Lamp — warm light, tiny footprint, 30-day desk trial.',
        note: '8% per sale · matched on desk lighting & workspace gear',
      },
    },
  },
]

const outlineClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s0 px-5 text-center font-semibold text-t1 transition-colors duration-150 hover:bg-s2 disabled:text-t1 sm:w-auto'
const barWidth = ['w-full', 'w-11/12', 'w-4/5', 'w-3/4']

function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function failureCopy(reason: ReadFailure, errors: Record<string, ErrorCopy>) {
  switch (reason) {
    case 'private':
      return errors['403']
    case 'unread':
      return errors['422']
    case 'empty':
      return errors['empty']
    default: {
      const unexpected: never = reason
      return unexpected
    }
  }
}

async function loadPreview(url: string, signal: AbortSignal): Promise<PreviewGroups> {
  const search = window.location.search
  const fixture = readPreviewFixture(search)
  const includeFixture = includeUnverifiedFixture(search)
  if (fixture === '400' || fixture === '403' || fixture === '422' || fixture === '429') {
    await wait(700)
    throw new PreviewStatus(Number(fixture))
  }
  if (fixture === 'empty') {
    await wait(700)
    return adaptPreview({ title: 'Draft with no text', text: '', offer: null })
  }
  if (fixture) {
    await wait(700)
    const body = fixture === 'groups' ? groupsFixtureBody() : fixtureBody(fixture)
    const parsed = adaptPreview(body, { includeFixture: includeFixture || fixture === 'groups' || fixture === 'nomatch' })
    if (!parsed.title) throw new PreviewStatus(422)
    return parsed
  }
  const timeout = new AbortController()
  const timer = setTimeout(() => timeout.abort(), previewTimeoutMs)
  const onAbort = () => timeout.abort()
  signal.addEventListener('abort', onAbort)
  try {
    const response = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: JSON.stringify({ url }),
      cache: 'no-store',
      signal: timeout.signal,
    })
    const raw = await response.text()
    let body: unknown = null
    if (raw) {
      try { body = JSON.parse(raw) } catch { body = null }
    }
    if (!response.ok) throw new PreviewStatus(codeFromStatus(response.status, false))
    const parsed = adaptPreview(body, { includeFixture })
    if (parsed.readFailure) return parsed
    if (!parsed.title) throw new PreviewStatus(422)
    return parsed
  } catch (err) {
    if (err instanceof PreviewStatus) throw err
    const aborted = !!err && typeof err === 'object' && 'name' in err && err.name === 'AbortError'
    throw new PreviewStatus(codeFromStatus(0, aborted))
  } finally {
    clearTimeout(timer)
    signal.removeEventListener('abort', onAbort)
  }
}

function OfferCard({ offer, label }: { offer: VerifiedOffer; label: string }) {
  const { t } = useLocale()
  return (
    <div
      data-offer
      className="my-1.5 overflow-hidden rounded-xl border-2 border-pub bg-pub-soft/85 p-4 shadow-xs"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="inline-flex items-center gap-1.5 font-mono text-[11px] font-semibold uppercase tracking-wider text-pub">
          <Sparkles aria-hidden="true" className="h-3.5 w-3.5" />
          {label}
        </span>
        <span className="inline-flex items-center gap-1 rounded-full border border-pub/30 bg-s1 px-2 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wider text-pub">
          <ShieldCheck aria-hidden="true" className="h-3 w-3" />
          Verified match
        </span>
      </div>
      <p className="mt-2 break-words font-display text-lg font-bold text-t1">{offer.name}</p>
      <p className="mt-1 break-words text-[15px] leading-relaxed text-t1">{offer.blurb}</p>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-pub/20 pt-2.5 font-mono text-[11px] text-t2">
        <span>{t.preview.sponsored}</span>
        <span>rel=&quot;sponsored&quot;</span>
      </div>
    </div>
  )
}

function SuggestedCard({ program }: { program: UnverifiedProgram }) {
  const { t } = useLocale()
  return (
    <a href={program.program_url} data-suggested rel="noreferrer" className="mt-3 block min-h-12 min-w-0 rounded-xl border border-line bg-s0 p-4 text-t1 no-underline">
      <Badge variant="neutral">{t.preview.unverified}</Badge>
      <p className="mt-2 break-words font-display text-lg font-bold">{program.name}</p>
      <p className="mt-1 break-words text-[15px] leading-relaxed">{program.blurb}</p>
    </a>
  )
}

function PostCard({ preview, platform }: { preview: PreviewGroups; platform: ShowcaseId }) {
  const { t } = useLocale()
  const suggestion = preview.suggestion
  const mock = {
    author: preview.author || t.preview.postFallback,
    handle: '',
    title: preview.title,
    body: preview.excerpt || '',
    offer: suggestion?.line || preview.title,
    meta: suggestion?.note || '',
  }
  return (
    <article className="mt-3" aria-label={t.preview.ariaLabel}>
      {preview.image && (
        <img src={preview.image} alt="" loading="lazy" referrerPolicy="no-referrer" className="mb-3 aspect-video w-full rounded-xl border border-line object-cover" />
      )}
      <PlatformPostFrame platform={platform} mock={mock} tag={t.preview.sponsored} demo={t.preview.demo} />
    </article>
  )
}

function GroupHeading({ level, id, children }: { level: 3 | 4; id: string; children: ReactNode }) {
  const className = 'font-display text-lg font-bold'
  if (level === 3) return <h3 id={id} className={className}>{children}</h3>
  return <h4 id={id} className={className}>{children}</h4>
}

function SuggestedGroup({ id, heading, programs, level }: { id: string; heading: string; programs: UnverifiedProgram[]; level: 3 | 4 }) {
  const { t } = useLocale()
  if (!programs.length) return null
  return (
    <section aria-labelledby={id} className="mt-5">
      <GroupHeading level={level} id={id}>{heading}</GroupHeading>
      <p className="mt-1 text-[15px] leading-relaxed text-t1">{t.preview.suggestedNote}</p>
      {programs.map((program) => <SuggestedCard key={program.program_url} program={program} />)}
    </section>
  )
}

function GroupsView({ preview, approveHref, statusId, titleLevel }: { preview: PreviewGroups; approveHref: string; statusId: string; titleLevel: 2 | 3 }) {
  const { t } = useLocale()
  const at = slotAfterBar(preview.slot || '')
  const offer = preview.verified[0]
  const slotKey = (preview.slot || '') as keyof typeof t.preview.slotLabels
  const slotName = t.preview.slotLabels[slotKey] ?? t.preview.slotLabels.fallback
  const rows: ReactNode[] = []
  if (offer) {
    for (let i = 1; i <= barCount; i++) {
      rows.push(<div key={i} data-paragraph aria-hidden="true" className={`h-2.5 rounded-full bg-s2 ${barWidth[i - 1]}`} />)
      if (i === at) rows.push(<OfferCard key="offer" offer={offer} label={slotName} />)
    }
  }
  const groupLevel = titleLevel === 2 ? 3 : 4
  return (
    <div>
      {offer && (
        <section aria-labelledby={`${statusId}-verified`} className="mt-4">
          <GroupHeading level={groupLevel} id={`${statusId}-verified`}>{t.preview.verifiedHeading}</GroupHeading>
          <div className="mt-3 grid gap-2">{rows}</div>
        </section>
      )}
      <SuggestedGroup id={`${statusId}-suggested`} heading={t.preview.suggestedHeading} programs={preview.suggested} level={groupLevel} />
      {offer && <a href={approveHref} className={`${outlineClass} mt-4`}>{t.preview.approve}</a>}
    </div>
  )
}

export function PostPreview({ titleLevel = 3 }: { titleLevel?: 2 | 3 }) {
  const source = useSource()
  const { t } = useLocale()
  const id = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const ticket = useRef(0)
  const flight = useRef<AbortController | null>(null)
  const [url, setUrl] = useState('')
  const [platform, setPlatform] = useState<ShowcaseId>('blog')
  const [phase, setPhase] = useState<Phase>('idle')
  const [preview, setPreview] = useState<PreviewGroups | null>(null)
  const [errorCode, setErrorCode] = useState<number | null>(null)
  const [readFailure, setReadFailure] = useState<ReadFailure | null>(null)
  const [note, setNote] = useState<'saved' | 'failed' | null>(null)
  const [paused, setPaused] = useState(false)
  const [pauseNonce, setPauseNonce] = useState(0)
  const approveHref = signInHref(roleReturn('publisher'), source)
  const copy = errorCode == null ? null : t.preview.errors[String(errorCode) as keyof typeof t.preview.errors] ?? t.preview.errors['422']
  const unread = readFailure ? failureCopy(readFailure, t.preview.errors) : null
  const view = phase === 'ready' && preview ? preview.kind : phase

  useEffect(() => {
    if (!pauseNonce) return
    const timer = window.setTimeout(() => setPaused(false), rateLimitPauseMs)
    return () => window.clearTimeout(timer)
  }, [pauseNonce])

  function stopFlight() {
    ticket.current += 1
    flight.current?.abort()
    flight.current = null
  }

  function resetResult() {
    setPreview(null)
    setErrorCode(null)
    setReadFailure(null)
    setNote(null)
  }

  async function runPreviewForUrl(value: string, exampleChip?: ExampleChip) {
    if (paused || phase === 'loading') return
    let parsedUrl: URL | null = null
    try { parsedUrl = new URL(value) } catch { parsedUrl = null }
    if (!parsedUrl || (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:') || !parsedUrl.hostname.includes('.')) {
      stopFlight()
      resetResult()
      setErrorCode(400)
      setPhase('error')
      return
    }
    stopFlight()
    setPlatform(exampleChip ? exampleChip.platform : platformFromHostname(parsedUrl.hostname))
    const mine = ticket.current
    const controller = new AbortController()
    flight.current = controller
    resetResult()
    setPhase('loading')
    try {
      const hasUrlFixture = Boolean(readPreviewFixture(window.location.search))
      const result = exampleChip && !hasUrlFixture
        ? await wait(320).then(() => adaptPreview(exampleChip.body, { includeFixture: false }))
        : await loadPreview(value, controller.signal)
      if (ticket.current !== mine) return
      setPreview(result)
      if (result.readFailure) {
        setReadFailure(result.readFailure)
        setPhase('unread')
        return
      }
      setPhase('ready')
    } catch (err) {
      if (ticket.current !== mine) return
      const code = err instanceof PreviewStatus ? err.code : 422
      if (code === 403 || code === 422) {
        setReadFailure(code === 403 ? 'private' : 'unread')
        setPhase('unread')
        return
      }
      setErrorCode(code)
      setPhase('error')
      if (code === 429) {
        setPaused(true)
        setPauseNonce((n) => n + 1)
      }
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    await runPreviewForUrl(url.trim())
  }

  function handleExampleClick(chip: ExampleChip) {
    setUrl(chip.url)
    void runPreviewForUrl(chip.url, chip)
  }

  function tryAnother() {
    stopFlight()
    resetResult()
    setPhase('idle')
    inputRef.current?.focus()
  }

  function notify() {
    // TODO: no notify endpoint exists. Keep the intent in this browser until one does.
    try {
      const before = readNotifyIntent(window.localStorage)
      const after = storeNotifyIntent(window.localStorage)
      if (JSON.stringify(before) !== JSON.stringify(after)) setNote('saved')
    } catch {
      setNote('failed')
    }
  }

  return (
    <div data-post-preview data-preview-state={view} className="min-w-0 max-w-full rounded-2xl border border-line bg-s1 p-5">
      <form noValidate onSubmit={onSubmit} className="grid gap-3">
        <label htmlFor={id} className="scroll-mt-24 text-[15px] font-semibold text-t1">{t.preview.label}</label>
        <Input
          id={id}
          ref={inputRef}
          type="url"
          inputMode="url"
          name="post-url"
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          value={url}
          placeholder="https://example.com/your-post…"
          aria-invalid={errorCode === 400}
          aria-describedby={`${id}-public ${id}-status`}
          onChange={(event) => setUrl(event.target.value)}
          className="min-h-12 min-w-0 bg-s0"
        />
        <div className="flex flex-wrap items-center gap-2" aria-label="Try an example URL">
          <span className="font-mono text-[11px] uppercase tracking-wider text-t2">Try an example:</span>
          {EXAMPLE_CHIPS.map((chip) => {
            const Icon = chip.icon
            const active = url.trim() === chip.url
            return (
              <button
                key={chip.id}
                type="button"
                data-example-chip={chip.id}
                disabled={phase === 'loading' || paused}
                onClick={() => handleExampleClick(chip)}
                className={`inline-flex min-h-9 items-center gap-1.5 rounded-full border px-3 text-xs font-semibold transition-colors ${
                  active
                    ? 'border-pub bg-pub-soft text-pub'
                    : 'border-line-strong bg-s0 text-t1 hover:border-pub hover:bg-pub-soft/50'
                }`}
              >
                <Icon aria-hidden="true" className="h-3.5 w-3.5" />
                {chip.label}
              </button>
            )
          })}
        </div>
        <p id={`${id}-public`} className="text-[13px] leading-relaxed text-t2">{t.preview.publicNote}</p>
        <button type="submit" disabled={phase === 'loading' || paused} aria-busy={phase === 'loading'} className={outlineClass}>{t.preview.submit}</button>
      </form>
      <div id={`${id}-status`} role="status" aria-live="polite" aria-atomic="true" aria-busy={phase === 'loading'} className="mt-4 break-words text-[15px] leading-relaxed text-t1 empty:mt-0">
        {phase === 'loading' && <p>{t.preview.loading}</p>}
        {phase === 'error' && copy && (
          <div>
            <p>{copy.message}</p>
            <p className="mt-1">{copy.next}</p>
            {errorCode !== 429 && <button type="button" onClick={tryAnother} className={`${outlineClass} mt-4`}>{t.preview.tryAnother}</button>}
          </div>
        )}
        {phase === 'unread' && unread && (
          <div data-read-failure={readFailure}>
            <p>{unread.message}</p>
            <p className="mt-1">{unread.next}</p>
            <button type="button" onClick={tryAnother} className={`${outlineClass} mt-4`}>{t.preview.tryAnother}</button>
          </div>
        )}
        {phase === 'ready' && preview && <PostCard preview={preview} platform={platform} />}
        {phase === 'ready' && preview?.kind === 'groups' && <GroupsView preview={preview} approveHref={approveHref} statusId={id} titleLevel={titleLevel} />}
        {phase === 'ready' && preview?.kind === 'closest' && (
          <div>
            <p className="mt-3">{t.preview.closestLead}</p>
            <SuggestedGroup id={`${id}-closest`} heading={t.preview.closestHeading} programs={preview.closest} level={titleLevel === 2 ? 3 : 4} />
            <button type="button" onClick={notify} className={`${outlineClass} mt-4`}>{t.preview.notify}</button>
            {note === 'saved' && <p className="mt-2">{t.preview.saved}</p>}
            {note === 'failed' && <p className="mt-2">{t.preview.saveFailed}</p>}
            <button type="button" onClick={tryAnother} className={`${outlineClass} mt-4`}>{t.preview.tryAnother}</button>
          </div>
        )}
      </div>
    </div>
  )
}
