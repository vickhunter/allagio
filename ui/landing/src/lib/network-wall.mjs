// Public network wall client: recent deals (GET /v1/deals/recent) plus the
// open offers feed (GET /v1/offers). Deal ids are never exposed by the API.
// ?dealsFixture=1 renders fixture data for local testing without a backend.

const fixtures = {
  deals: [
    {
      kind: 'swap', state: 'active', updated_at: new Date(Date.now() - 42 * 60000).toISOString(), parties: 2,
      offerer: { listed: false, role: 'publisher' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'affiliate', state: 'active', updated_at: new Date(Date.now() - 3 * 3600000).toISOString(), parties: 2,
      offerer: { listed: true, name: 'Vibe Sales', url: 'https://vibesales.example' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'link_exchange', state: 'awaiting', updated_at: new Date(Date.now() - 26 * 3600000).toISOString(), parties: 2,
      offerer: { listed: false, role: 'agent', host: 'agents.example' },
      acceptor: { listed: true, name: 'Goji Analytics', url: 'https://goji.example' },
    },
  ],
  offers: [
    {
      offer_id: 'off_1', product_name: 'Vibe Sales', product_tagline: 'Checkout links your storefront can sell itself',
      website_url: 'https://vibesales.example', primary_category: 'software', keywords: [], tags: [],
      commission_type: 'percentage', commission_value: 30, commission_currency: 'EUR',
      cookie_duration_days: 30, refund_window_days: 14, terms: {},
    },
    {
      offer_id: 'off_2', product_name: 'Goji Analytics', product_tagline: 'Privacy-first analytics for small teams',
      website_url: 'https://goji.example', primary_category: 'software', keywords: [], tags: [],
      commission_type: 'flat', commission_value: 25, commission_currency: 'USD',
      cookie_duration_days: 45, refund_window_days: 30, terms: {},
    },
  ],
}

export function wantsDealsFixtures() {
  if (typeof window === 'undefined') return false
  return new URLSearchParams(window.location.search).get('dealsFixture') === '1'
}

export function dealsFixtureData() {
  return fixtures
}

export async function listRecentDeals(fetchImpl = fetch) {
  const res = await fetchImpl('/v1/deals/recent')
  if (!res.ok) throw new Error('deals ' + res.status)
  const body = await res.json()
  return Array.isArray(body && body.deals) ? body.deals : []
}

export async function listPublicOffers(fetchImpl = fetch) {
  const res = await fetchImpl('/v1/offers')
  if (!res.ok) throw new Error('offers ' + res.status)
  const body = await res.json()
  return Array.isArray(body && body.offers) ? body.offers : []
}

// partyText is the wall label for one party. A listed party shows its real
// name; everyone else is a role word ("a publisher", "an agent"). Host is
// appended for foreign parties ("an agent · agents.example").
export function partyText(face, roleWords) {
  if (face && face.listed && face.name) return { text: face.name, url: face.url || '', named: true }
  const role = (face && face.role) || 'agent'
  let text = roleWords[role] || roleWords.agent || 'an agent'
  if (face && face.host) text += ' · ' + face.host
  return { text, url: '', named: false }
}

export function commissionText(offer) {
  if (!offer || offer.commission_value == null || offer.commission_value === 0) return ''
  if (offer.commission_type === 'percentage') return offer.commission_value + '%'
  if (offer.commission_type === 'recurring') return offer.commission_value + '% recurring'
  // flat: commission_value is already in major currency units
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: offer.commission_currency || 'EUR' }).format(offer.commission_value)
}
