import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Input } from '@/components/ui/input'
import { roleReturn, signInHref, useSource } from '@/lib/source'
import { PreviewStatus, barCount, codeFromStatus, fixtureBody, parsePreview, previewErrorCopy, previewTimeoutMs, rateLimitPauseMs, readPreviewFixture, slotAfterBar, slotLabel } from '@/lib/preview-map.mjs'

// Signed-out preview. The URL stays in the field. previewFixture skips the network.
type Offer = { name: string; blurb: string }
type Preview = { title: string; offer: Offer | null; slot: string | null }
type Phase = 'idle' | 'loading' | 'success' | 'nomatch' | 'error'

const outlineClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s0 px-5 text-center font-semibold text-t1 transition-colors duration-150 hover:bg-s2 disabled:text-t1 sm:w-auto'
const barWidth = ['w-full', 'w-11/12', 'w-4/5', 'w-3/4']

function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function loadPreview(url: string, signal: AbortSignal): Promise<Preview> {
  const fixture = readPreviewFixture(window.location.search)
  if (fixture === '400' || fixture === '403' || fixture === '422' || fixture === '429') {
    await wait(700)
    throw new PreviewStatus(Number(fixture))
  }
  if (fixture) {
    await wait(700)
    const parsed = parsePreview(fixtureBody(fixture))
    if (!parsed) throw new PreviewStatus(422)
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
    const parsed = parsePreview(body)
    if (!parsed) throw new PreviewStatus(422)
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

function OfferCard({ offer, label }: { offer: Offer; label: string }) {
  return (
    <div data-offer className="my-1 rounded-xl border-2 border-pub bg-pub-soft p-4">
      <p className="text-[13px] font-semibold text-pub">{label}</p>
      <p className="mt-1 break-words font-display text-lg font-bold text-t1">{offer.name}</p>
      <p className="mt-1 break-words text-[15px] leading-relaxed text-t1">{offer.blurb}</p>
      <p className="mt-2 text-[13px] text-t1">Sponsored, via Offer Rails</p>
    </div>
  )
}

function SuccessView({ preview, approveHref }: { preview: Preview; approveHref: string }) {
  if (!preview.offer) return null
  const at = slotAfterBar(preview.slot || '')
  const rows: ReactNode[] = []
  for (let i = 1; i <= barCount; i++) {
    rows.push(<div key={i} data-paragraph aria-hidden="true" className={`h-2.5 rounded-full bg-s2 ${barWidth[i - 1]}`} />)
    if (i === at) rows.push(<OfferCard key="offer" offer={preview.offer} label={slotLabel(preview.slot || '')} />)
  }
  return (
    <div>
      <h3 className="break-words font-display text-2xl font-bold tracking-tight">{preview.title}</h3>
      <div className="mt-4 grid gap-2">{rows}</div>
      <a href={approveHref} className={`${outlineClass} mt-4`}>Approve your placements</a>
    </div>
  )
}

export function PostPreview() {
  const source = useSource()
  const id = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const ticket = useRef(0)
  const flight = useRef<AbortController | null>(null)
  const [url, setUrl] = useState('')
  const [phase, setPhase] = useState<Phase>('idle')
  const [preview, setPreview] = useState<Preview | null>(null)
  const [errorCode, setErrorCode] = useState<number | null>(null)
  const [paused, setPaused] = useState(false)
  const [pauseNonce, setPauseNonce] = useState(0)
  const approveHref = signInHref(roleReturn('publisher'), source)
  const copy = errorCode == null ? null : previewErrorCopy(errorCode)

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

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (paused || phase === 'loading') return
    const value = url.trim()
    let parsedUrl: URL | null = null
    try { parsedUrl = new URL(value) } catch { parsedUrl = null }
    if (!parsedUrl || (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:') || !parsedUrl.hostname.includes('.')) {
      stopFlight()
      setPreview(null)
      setErrorCode(400)
      setPhase('error')
      return
    }
    stopFlight()
    const mine = ticket.current
    const controller = new AbortController()
    flight.current = controller
    setPreview(null)
    setErrorCode(null)
    setPhase('loading')
    try {
      const result = await loadPreview(value, controller.signal)
      if (ticket.current !== mine) return
      setPreview(result)
      setPhase(result.offer ? 'success' : 'nomatch')
    } catch (err) {
      if (ticket.current !== mine) return
      const code = err instanceof PreviewStatus ? err.code : 422
      setErrorCode(code)
      setPhase('error')
      if (code === 429) {
        setPaused(true)
        setPauseNonce((n) => n + 1)
      }
    }
  }

  function tryAnother() {
    stopFlight()
    setPreview(null)
    setErrorCode(null)
    setPhase('idle')
    inputRef.current?.focus()
  }

  return (
    <div data-post-preview data-preview-state={phase} className="min-w-0 max-w-full rounded-2xl border border-line bg-s1 p-5">
      <form noValidate onSubmit={onSubmit} className="grid gap-3">
        <label htmlFor={id} className="text-[15px] font-semibold text-t1">Paste a post URL</label>
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
          placeholder="https://example.com/your-post"
          aria-invalid={errorCode === 400}
          aria-describedby={`${id}-status`}
          onChange={(event) => setUrl(event.target.value)}
          className="min-h-12 min-w-0 bg-s0"
        />
        <button type="submit" disabled={phase === 'loading' || paused} aria-busy={phase === 'loading'} className={outlineClass}>Preview this post</button>
      </form>
      <div id={`${id}-status`} role="status" aria-live="polite" aria-atomic="true" aria-busy={phase === 'loading'} className="mt-4 min-h-8 break-words text-[15px] leading-relaxed text-t1">
        {phase === 'loading' && <p>Reading your post, up to 30s</p>}
        {phase === 'error' && copy && (
          <div>
            <p>{copy.message}</p>
            <p className="mt-1">{copy.next}</p>
            {errorCode !== 429 && <button type="button" onClick={tryAnother} className={`${outlineClass} mt-4`}>Try another URL</button>}
          </div>
        )}
        {phase === 'nomatch' && preview && (
          <div>
            <h3 className="break-words font-display text-2xl font-bold tracking-tight">{preview.title}</h3>
            <p className="mt-3">No offer fits this post yet.</p>
            <p className="mt-1 text-t2">Offer Rails places an offer only when it matches the post.</p>
            <button type="button" onClick={tryAnother} className={`${outlineClass} mt-4`}>Try another URL</button>
          </div>
        )}
        {phase === 'success' && preview?.offer && <SuccessView preview={preview} approveHref={approveHref} />}
      </div>
    </div>
  )
}
