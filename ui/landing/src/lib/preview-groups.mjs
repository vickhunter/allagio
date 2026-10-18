// Adapts POST /api/preview into verified and suggested groups.
// D-072 will send verified, source_url, checked_at, and program_url.
// Until then, an existing offer defaults to verified, and the unverified
// catalog below is included only when a dev or test flag asks for it.

import { previewErrorCopy, samplePreview } from './preview-map.mjs'

export const publicLinksNote = "Public links only. Private posts and accounts can't be read."
export const suggestedNote = "Not on Offer Rails yet. Payouts go through the company's own affiliate program."
export const closestLead = 'No program lines up with this post yet.'
export const notifyLabel = 'Notify me when a match joins'
export const notifyStorageKey = 'offer-rails.preview.notify'
export const notifyLimit = 20
export const readNext = 'Make the post public or paste another link'

// TODO: no notify endpoint exists. Callers store the URL locally and do not contact the server.
export const unverifiedFixture = [
  {
    verified: false,
    name: 'Baron Fig',
    blurb: 'A clothbound notebook for daily pages.',
    program_url: 'https://programs.example/baron-fig',
    source_url: 'https://sources.example/baron-fig',
    checked_at: '2026-10-01',
    topics: ['journal', 'notes', 'notebook'],
  },
  {
    verified: false,
    name: 'Leuchtturm',
    blurb: 'A structured journal with numbered pages.',
    program_url: 'https://programs.example/leuchtturm',
    source_url: 'https://sources.example/leuchtturm',
    checked_at: '2026-10-01',
    topics: ['journal', 'planner'],
  },
  {
    verified: false,
    name: 'Skillshare',
    blurb: 'Classes for learning a craft.',
    program_url: 'https://programs.example/skillshare',
    source_url: 'https://sources.example/skillshare',
    checked_at: '2026-09-12',
    topics: ['class', 'course'],
  },
  {
    verified: false,
    name: 'ConvertKit',
    blurb: 'Email for a writing practice.',
    program_url: 'https://programs.example/convertkit',
    source_url: 'https://sources.example/convertkit',
    checked_at: '2026-08-02',
    topics: ['email', 'newsletter'],
  },
]

const emptyCopy = {
  message: 'That page returned no text.',
  next: readNext,
}

export function readFailureCopy(reason) {
  if (reason === 'private') return previewErrorCopy(403)
  if (reason === 'unread') return previewErrorCopy(422)
  return emptyCopy
}

export function classifyReadFailure(body) {
  if (!body || typeof body !== 'object') return null
  const result = typeof body.result === 'string' ? body.result.toLowerCase() : ''
  if (result === 'unreadable_or_private') return 'unread'
  if (body.text === '') return 'empty'
  const reason = typeof body.reason === 'string' ? body.reason.toLowerCase() : ''
  if (reason === 'empty' || reason.includes('empty text') || reason.includes('no text')) return 'empty'
  if (reason.includes('private')) return 'private'
  const error = typeof body.error === 'string' ? body.error.toLowerCase() : ''
  if (error.includes('empty')) return 'empty'
  if (error.includes('private') || error === 'blocked address') return 'private'
  if (error === 'fetch failed' || error.includes('could not read') || error.includes("couldn't read")) return 'unread'
  return null
}

export function includeUnverifiedFixture(search) {
  const fixture = new URLSearchParams(search || '').get('previewFixture') || ''
  return fixture === 'groups' || fixture === 'nomatch'
}

function textOf(value) {
  return typeof value === 'string' ? value.trim() : ''
}

function topicTokens(text) {
  return new Set(text.toLowerCase().split(/[^a-z0-9]+/).filter((word) => word.length > 3))
}

function topicScore(tokens, program) {
  let score = 0
  for (const topic of program.topics || []) {
    if (tokens.has(String(topic).toLowerCase())) score += 1
  }
  return score
}

function programUrl(value) {
  const raw = textOf(value)
  if (!raw) return ''
  let parsed
  try { parsed = new URL(raw) } catch { return '' }
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return ''
  parsed.username = ''
  parsed.password = ''
  const path = parsed.pathname.toLowerCase()
  if (path === '/go' || path.startsWith('/go/') || path.includes('/v1/go')) return ''
  parsed.hash = ''
  for (const key of [...parsed.searchParams.keys()]) {
    const name = key.toLowerCase()
    if (name === 'via' || name === 'ref' || name === 'source' || name.startsWith('utm_')) parsed.searchParams.delete(key)
  }
  return parsed.search ? parsed.toString() : `${parsed.origin}${parsed.pathname}`
}

function asOffer(raw) {
  if (!raw || typeof raw !== 'object') return null
  const name = textOf(raw.name)
  const blurb = textOf(raw.summary) || textOf(raw.blurb)
  if (!name || !blurb) return null
  if (raw.verified === false) {
    const program = programUrl(raw.program_url)
    if (!program) return null
    return {
      verified: false,
      name,
      blurb,
      program_url: program,
      source_url: textOf(raw.source_url),
      checked_at: textOf(raw.checked_at),
      topics: Array.isArray(raw.topics) ? raw.topics.map((topic) => textOf(topic)).filter(Boolean) : [],
    }
  }
  return { verified: true, name, blurb }
}

function collectOffers(body) {
  const rows = []
  if (body.offer && typeof body.offer === 'object') rows.push(body.offer)
  if (Array.isArray(body.offers)) rows.push(...body.offers)
  const offers = []
  for (const row of rows) {
    const offer = asOffer(row)
    if (offer) offers.push(offer)
  }
  return offers
}

function rankPrograms(programs, tokens) {
  return programs
    .map((program, index) => ({ program, index, score: topicScore(tokens, program) }))
    .sort((a, b) => b.score - a.score || a.index - b.index)
    .map((row) => row.program)
}

export function groupsFixtureBody() {
  return {
    title: samplePreview.title,
    text: 'How to start a journal with a pocket notebook.',
    offer: { ...samplePreview.offer, verified: true },
    slot: samplePreview.slot,
  }
}

export function groupPreview(body, options = {}) {
  const includeFixture = options.includeFixture === true
  const title = textOf(body && body.title)
  const slot = body && typeof body.slot === 'string' ? body.slot : null
  const offers = collectOffers(body || {})
  const verified = offers.filter((offer) => offer.verified !== false)
  const suggested = offers.filter((offer) => offer.verified === false)
  const postText = `${title} ${textOf(body && body.text)}`.trim()
  const tokens = topicTokens(postText)
  let closest = []
  if (includeFixture) {
    for (const program of unverifiedFixture) {
      if (suggested.some((item) => item.program_url === program.program_url)) continue
      if (topicScore(tokens, program) > 0) suggested.push({ ...program })
    }
    if (verified.length === 0 && suggested.length === 0) {
      closest = rankPrograms(unverifiedFixture, tokens).slice(0, 3).map((program) => ({ ...program }))
    }
  }
  const kind = verified.length || suggested.length ? 'groups' : 'closest'
  return {
    title,
    verified,
    suggested,
    closest,
    readFailure: null,
    kind,
    slot: verified.length ? slot : null,
  }
}

export function adaptPreview(body, options = {}) {
  const readFailure = classifyReadFailure(body)
  const title = textOf(body && body.title)
  if (readFailure) {
    return { title, verified: [], suggested: [], closest: [], readFailure, kind: 'unreadable', slot: null }
  }
  return groupPreview(body, options)
}

export function orderedNames(grouped) {
  const rows = grouped.kind === 'closest' ? grouped.closest : [...grouped.verified, ...grouped.suggested]
  return rows.map((row) => row.name)
}

export function readNotifyIntent(storage) {
  try {
    const parsed = JSON.parse(storage.getItem(notifyStorageKey) || '[]')
    return Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string' && item.trim()) : []
  } catch {
    return []
  }
}

export function storeNotifyIntent(storage, url) {
  // TODO: no notify endpoint exists. This writes the post URL in this browser and does not call a server.
  const value = textOf(url)
  const urls = readNotifyIntent(storage).filter((item) => item !== value)
  if (value) urls.push(value)
  const capped = urls.slice(-notifyLimit)
  storage.setItem(notifyStorageKey, JSON.stringify(capped))
  return capped
}
