import { useState } from 'react'

// A landing tag is a short slug from /from/<tag>, ?source=, or the or_source cookie.
// "direct" is the fallback and is not copied onto links. Owner signup still keeps
// its own allow-list; this one only has to survive a strict-origin referrer.
const TAG = /^[a-z0-9][a-z0-9_-]{0,63}$/

export function normalizeSource(raw: string | null | undefined) {
  const value = String(raw || '').trim().toLowerCase()
  if (value === 'direct' || !TAG.test(value)) return ''
  return value
}

function readCookie(name: string) {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return match ? decodeURIComponent(match[1]) : ''
}

export function readAndPersistSource() {
  if (typeof window === 'undefined') return ''
  const params = new URLSearchParams(window.location.search)
  let source = normalizeSource(params.get('source'))
  if (!source) source = normalizeSource(readCookie('or_source'))
  if (!source && document.referrer) {
    try {
      const ref = new URL(document.referrer)
      const match = ref.pathname.match(/\/from\/([^/]+)/i)
      if (match && ref.origin === window.location.origin) source = normalizeSource(decodeURIComponent(match[1]))
    } catch {
      source = ''
    }
  }
  if (!source) {
    try { source = normalizeSource(sessionStorage.getItem('or_source')) } catch { source = '' }
  }
  if (source) {
    try { sessionStorage.setItem('or_source', source) } catch { /* private mode */ }
  }
  return source
}

export function withSource(href: string, source: string) {
  if (!source || href.startsWith('#')) return href
  try {
    const url = new URL(href, 'https://offer-rails.example')
    if (url.origin !== 'https://offer-rails.example') return href
    const path = url.pathname
    const tracked = path === '/' || path.startsWith('/makers') || path.startsWith('/publishers') || path.startsWith('/protocol') || path.startsWith('/privacy') || path.startsWith('/terms') || path.startsWith('/agents') || path.startsWith('/owner') || path.startsWith('/auth/login') || path.startsWith('/landing-v9/agents')
    if (!tracked) return href
    url.searchParams.set('source', source)
    return url.pathname + url.search + url.hash
  } catch {
    return href
  }
}

export function ownerHref(path: string, source: string) {
  return withSource(path, source)
}

export function signInHref(returnTo: string, source: string) {
  const dest = withSource(returnTo, source)
  const params = new URLSearchParams()
  params.set('return_to', dest)
  if (source) params.set('source', source)
  return '/auth/login?' + params.toString()
}

export function roleReturn(role: 'maker' | 'publisher' | 'both') {
  if (role === 'publisher') return '/publishers/setup'
  return '/owner?setup=maker'
}

export function readInstallConfig() {
  if (typeof document === 'undefined') return { install: '', plugin: false }
  const install = document.querySelector('meta[name="or-install"]')?.getAttribute('content')?.trim() ?? ''
  const plugin = document.querySelector('meta[name="or-plugin"]')?.getAttribute('content') === '1'
  return { install, plugin }
}

export function useSource() {
  const [source] = useState(() => readAndPersistSource())
  return source
}

export function useOfferConfig() {
  const [config] = useState(() => readInstallConfig())
  return config
}
