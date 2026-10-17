import { useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Input } from '@/components/ui/input'
import { roleReturn, signInHref, useSource } from '@/lib/source'

// Signed-out preview. The URL stays in the field. POST /api/preview is D-046a and
// may 404; previewFixture on the page URL is the proof stand-in and does not fetch.
type Offer = { name: string; url: string; blurb: string }
type Preview = { title: string; offer: Offer | null; slot: string }
type Phase = 'idle' | 'loading' | 'invalid' | 'success' | 'nomatch' | 'error'
type Fixture = '' | 'success' | 'nomatch' | 'error'

const outlineClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s0 px-5 text-center font-semibold text-t1 transition-colors duration-150 hover:bg-s2 disabled:text-t1 sm:w-auto'
const barWidth = ['w-full', 'w-11/12', 'w-4/5', 'w-3/4', 'w-5/6', 'w-2/3']
const journal: Preview = {
  title: 'How to start a journal',
  offer: { name: 'Field Notes', url: 'https://example.com/notes', blurb: 'A pocket notebook for the habit.' },
  slot: 'after_p2',
}

function readFixture(): Fixture {
  if (typeof window === 'undefined') return ''
  const value = new URLSearchParams(window.location.search).get('previewFixture')
  if (value === 'success' || value === 'nomatch' || value === 'error') return value
  return ''
}

function validPostUrl(value: string) {
  try {
    const url = new URL(value.trim())
    return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname.includes('.')
  } catch {
    return false
  }
}

function isOffer(value: unknown): value is Offer {
  if (!value || typeof value !== 'object') return false
  const offer = value as Record<string, unknown>
  return typeof offer.name === 'string' && offer.name.trim() !== '' && typeof offer.url === 'string' && offer.url.trim() !== '' && typeof offer.blurb === 'string' && offer.blurb.trim() !== ''
}

function isPreview(value: unknown): value is Preview {
  if (!value || typeof value !== 'object') return false
  const body = value as Record<string, unknown>
  if (typeof body.title !== 'string' || body.title.trim() === '') return false
  if (typeof body.slot !== 'string' || body.slot.trim() === '') return false
  if (body.offer === null) return true
  return isOffer(body.offer)
}

function slotParagraph(slot: string) {
  const match = /^after_p(\d+)$/.exec(slot)
  if (!match) return null
  const n = Number(match[1])
  if (!Number.isInteger(n) || n < 1) return null
  return n
}

function slotWords(slot: string) {
  const n = slotParagraph(slot)
  if (!n) return 'In the post'
  return `After paragraph ${n}`
}

function explainPreviewError(message: string) {
  const text = message.toLowerCase()
  if (text.includes('rate')) return 'Too many previews just now. Wait a moment and try again.'
  if (text.includes('html')) return 'That page is not an article we can read.'
  if (text.includes('unreach') || text.includes('failed to fetch') || text.includes('not found') || text.includes('network')) return 'We could not reach that page.'
  return 'We could not read that page.'
}

function errorText(body: unknown) {
  if (!body || typeof body !== 'object' || !('error' in body)) return ''
  const error = (body as { error: unknown }).error
  return typeof error === 'string' ? error : ''
}

async function loadPreview(url: string): Promise<Preview> {
  const fixture = readFixture()
  if (fixture) {
    await new Promise((resolve) => setTimeout(resolve, 700))
    if (fixture === 'error') throw new Error('rate limited')
    if (fixture === 'nomatch') return { title: journal.title, offer: null, slot: journal.slot }
    return journal
  }
  let response: Response
  try {
    response = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'content-type': 'application/json', accept: 'application/json' },
      body: JSON.stringify({ url }),
      cache: 'no-store',
    })
  } catch {
    throw new Error('unreachable')
  }
  const raw = await response.text()
  let body: unknown = null
  if (raw) {
    try { body = JSON.parse(raw) } catch { body = null }
  }
  if (!response.ok) throw new Error(errorText(body) || 'unreachable')
  if (!isPreview(body)) throw new Error('not HTML')
  return body
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
  const after = slotParagraph(preview.slot)
  const total = after ? Math.min(6, Math.max(4, after + 1)) : 4
  const at = after && after <= total ? after : total + 1
  const rows: ReactNode[] = []
  for (let i = 1; i <= total; i++) {
    rows.push(<div key={i} data-paragraph aria-hidden="true" className={`h-2.5 rounded-full bg-s2 ${barWidth[i - 1]}`} />)
    if (i === at) rows.push(<OfferCard key="offer" offer={preview.offer} label={slotWords(preview.slot)} />)
  }
  if (at > total) rows.push(<OfferCard key="offer" offer={preview.offer} label={slotWords(preview.slot)} />)
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
  const [url, setUrl] = useState('')
  const [phase, setPhase] = useState<Phase>('idle')
  const [preview, setPreview] = useState<Preview | null>(null)
  const [error, setError] = useState('')
  const approveHref = signInHref(roleReturn('publisher'), source)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    const value = url.trim()
    if (!validPostUrl(value)) {
      ticket.current += 1
      setPreview(null)
      setError('')
      setPhase('invalid')
      return
    }
    const mine = ticket.current + 1
    ticket.current = mine
    setPreview(null)
    setError('')
    setPhase('loading')
    try {
      const result = await loadPreview(value)
      if (ticket.current !== mine) return
      setPreview(result)
      setPhase(result.offer ? 'success' : 'nomatch')
    } catch (err) {
      if (ticket.current !== mine) return
      setError(err instanceof Error ? err.message : 'unreachable')
      setPhase('error')
    }
  }

  function tryAnother() {
    ticket.current += 1
    setPreview(null)
    setError('')
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
          aria-invalid={phase === 'invalid'}
          aria-describedby={`${id}-status`}
          onChange={(event) => setUrl(event.target.value)}
          className="min-h-12 min-w-0 bg-s0"
        />
        <button type="submit" disabled={phase === 'loading'} className={outlineClass}>Preview this post</button>
      </form>
      <div id={`${id}-status`} role="status" aria-live="polite" aria-atomic="true" aria-busy={phase === 'loading'} className="mt-4 text-[15px] leading-relaxed text-t1">
        {phase === 'loading' && (
          <>
            <p>Reading your post, up to 30s</p>
            <div aria-hidden="true" className="mt-4 grid gap-2">
              <div className="h-2.5 w-full rounded-full bg-s2" />
              <div className="h-2.5 w-11/12 rounded-full bg-s2" />
              <div className="h-2.5 w-4/5 rounded-full bg-s2" />
              <div className="h-2.5 w-3/4 rounded-full bg-s2" />
            </div>
          </>
        )}
        {phase === 'invalid' && <p>Enter a full post URL, starting with https://.</p>}
        {phase === 'error' && <p>{explainPreviewError(error)}</p>}
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
