import { readAndPersistSource } from '@/lib/source'

// Conversion stub. No network: logs only. Elements opt in with data-track="event" (+ data-track-*).
// v9 events kept: page_view, cta_click, nav_click, nav_cta, motion_toggle, path_pick,
// secondary_click, role_pick, join_submit, join_invalid, palette_open, palette_command,
// palette_button, shortcut, faq_open, side_toggle, demo_pause.
// sign_in_click and zip_download are the landing conversions D-044 sends to the network.
export type TrackProps = Record<string, string | number | boolean | undefined>
export function track(event: string, props: TrackProps = {}) {
  const source = readAndPersistSource()
  const entry = { event, ...props, ...(source ? { source } : {}), page: document.body.dataset.version, ts: new Date().toISOString() }
  ;(window as unknown as { __track: unknown[] }).__track ??= []
  ;(window as unknown as { __track: unknown[] }).__track.push(entry)
  console.info('[track]', entry)
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
