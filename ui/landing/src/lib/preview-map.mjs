// Maps the live POST /api/preview body onto the paste-a-URL card.
// Slot names and error statuses are the ones the handler returns.

export const slots = ['after_intro', 'mid_article', 'before_conclusion', 'end_of_article']
export const barCount = 4
export const previewTimeoutMs = 30000
export const rateLimitPauseMs = 8000

const labels = {
  after_intro: 'After the intro',
  mid_article: 'Mid-article',
  before_conclusion: 'Before the conclusion',
  end_of_article: 'End of article',
}

const afterBar = {
  after_intro: 1,
  mid_article: 2,
  before_conclusion: 3,
  end_of_article: 4,
}

export const previewErrors = {
  400: {
    message: "That doesn't look like a post URL.",
    next: 'Try one like https://example.com/your-post.',
  },
  403: {
    message: 'That page is private.',
    next: 'Make the post public or paste another link',
  },
  422: {
    message: "We couldn't read that page.",
    next: 'Make the post public or paste another link',
  },
  429: {
    message: 'Too many tries.',
    next: 'Wait a minute, then try again.',
  },
}

const knownStatus = new Set([400, 403, 422, 429])

export function slotLabel(slot) {
  return labels[slot] || 'In the post'
}

export function slotAfterBar(slot) {
  return afterBar[slot] || barCount
}

export function previewErrorCopy(code) {
  return previewErrors[code] || previewErrors[422]
}

export function codeFromStatus(status, aborted) {
  if (aborted) return 422
  if (knownStatus.has(status)) return status
  return 422
}

export function parsePreview(body) {
  if (!body || typeof body !== 'object') return null
  if (typeof body.title !== 'string' || body.title.trim() === '') return null
  const title = body.title.trim()
  if (body.offer == null) return { title, offer: null, slot: null }
  if (typeof body.offer !== 'object') return null
  const name = typeof body.offer.name === 'string' ? body.offer.name.trim() : ''
  const summary = typeof body.offer.summary === 'string' ? body.offer.summary.trim() : ''
  const blurb = summary || (typeof body.offer.blurb === 'string' ? body.offer.blurb.trim() : '')
  if (!name || !blurb) return null
  const slot = typeof body.slot === 'string' ? body.slot : ''
  return { title, offer: { name, blurb }, slot }
}

const fixtureNames = new Set(['success', 'nomatch', 'groups', 'empty', '400', '403', '422', '429', ...slots])

export function readPreviewFixture(search) {
  const value = new URLSearchParams(search || '').get('previewFixture') || ''
  return fixtureNames.has(value) ? value : ''
}

export const samplePreview = {
  title: 'How to start a journal',
  offer: {
    offer_id: '00000000-0000-0000-0000-000000000001',
    name: 'Field Notes',
    summary: 'A pocket notebook for the habit.',
    category: 'notes',
    website_url: 'https://example.com/notes',
    relevance: 80,
    reason: 'Product topics overlap this content.',
    commission_type: 'percentage',
    commission_value: 20,
    commission_currency: 'USD',
  },
  slot: 'after_intro',
}

export function fixtureBody(name) {
  if (name === 'nomatch') return { title: 'Sourdough timing', text: 'A crumb and a starter.', offer: null, slot: null }
  const slot = slots.includes(name) ? name : samplePreview.slot
  return {
    title: samplePreview.title,
    offer: { ...samplePreview.offer },
    slot,
  }
}

export class PreviewStatus extends Error {
  constructor(code) {
    super(String(code))
    this.name = 'PreviewStatus'
    this.code = codeFromStatus(code, false)
  }
}
