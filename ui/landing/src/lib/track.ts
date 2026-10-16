import { readAndPersistSource } from '@/lib/source'

// Conversion events go to POST /api/events. Other v9 events stay on the page.
// landing_view, signin_click, and zip_download are the names the server stores.
// sendBeacon is used so a sign-in navigation still records the click. fetch
// keepalive covers browsers that refuse the beacon.
export type TrackProps = Record<string, string | number | boolean | undefined>

const NETWORK: Record<string, string> = {
  page_view: 'landing_view',
  landing_view: 'landing_view',
  zip_download: 'zip_download',
  signin_click: 'signin_click',
  sign_in_click: 'signin_click',
  nav_cta: 'signin_click',
  join_submit: 'signin_click',
}

function conversionEvent(event: string, props: TrackProps) {
  if (event === 'cta_click' && (props.kind === 'sign_in_click' || props.kind === 'signin_click')) return 'signin_click'
  return NETWORK[event] ?? ''
}

function postConversion(name: string, source: string) {
  const payload = JSON.stringify(source ? { event: name, source } : { event: name })
  try {
    if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
      const blob = new Blob([payload], { type: 'application/json' })
      if (navigator.sendBeacon('/api/events', blob)) return
    }
  } catch {
    // fetch keepalive below
  }
  if (typeof fetch !== 'function') return
  fetch('/api/events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: payload,
    keepalive: true,
  }).catch(() => {})
}

export function track(event: string, props: TrackProps = {}) {
  const source = readAndPersistSource()
  const entry = { event, ...props, ...(source ? { source } : {}), page: document.body.dataset.version, ts: new Date().toISOString() }
  ;(window as unknown as { __track: unknown[] }).__track ??= []
  ;(window as unknown as { __track: unknown[] }).__track.push(entry)
  const name = conversionEvent(event, props)
  if (name) postConversion(name, source)
}

export function installTracking() {
  document.addEventListener('click', (e) => {
    const el = (e.target as HTMLElement).closest<HTMLElement>('[data-track]')
    if (!el) return
    const props: TrackProps = {}
    for (const [k, v] of Object.entries(el.dataset)) if (k.startsWith('track') && k !== 'track') props[k.slice(5).toLowerCase()] = v
    track(el.dataset.track!, props)
  })
  track('page_view')
}
