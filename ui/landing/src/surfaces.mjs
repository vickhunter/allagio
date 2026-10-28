// TODO(D-092): read adapterTests, e2e, and date from the adapters registry when that delivery exposes one.
// Until then this file is the only list the landing pages use.
// "Verified end to end" requires both adapterTests and e2e. WordPress used to say Live.
// That label was not these two checks, so both stay false and the status is In testing.

export const runsTitle = 'Works anywhere you can paste a link'
export const runsLead = 'One Offer Rails link counts clicks wherever it is shared: email, X, Instagram, YouTube, or a blog. No plugin is needed.'

const recorded = '2026-10-21'

export const surfaces = [
  { name: 'WordPress', audience: 'both', adapterTests: false, e2e: false, date: recorded, note: 'A plugin can place one labelled box in a post that fits. The link counts clicks without that plugin.' },
  { name: 'X', audience: 'both', adapterTests: false, e2e: false, date: recorded, note: 'Paste a link you approve. Posting stays with you.' },
  { name: 'Newsletter', audience: 'publisher', adapterTests: false, e2e: false, date: recorded, note: 'Agent only for now. Your agent prepares the link. Then paste it in an email.', href: '/publishers/#add-yours' },
  { name: 'YouTube', audience: 'publisher', adapterTests: false, e2e: false, date: recorded, note: 'Paste the link where the description allows.' },
  { name: 'Instagram', audience: 'publisher', adapterTests: false, e2e: false, date: recorded, note: 'Paste the link where the caption or bio allows.' },
  { name: 'TikTok', audience: 'publisher', adapterTests: false, e2e: false, date: recorded, note: 'Paste the link where the caption or bio allows.' },
  { name: 'Shopify', audience: 'maker', adapterTests: false, e2e: false, date: recorded, note: 'A shop can list products that fit.' },
]

export function surfaceStatus(item) {
  return item.adapterTests === true && item.e2e === true ? 'Verified end to end' : 'In testing'
}

export function surfacesFor(audience) {
  return surfaces.filter((item) => item.audience === audience || item.audience === 'both')
}

export function orderedSurfaces(audience) {
  const rows = surfacesFor(audience)
  return [
    ...rows.filter((item) => surfaceStatus(item) === 'Verified end to end'),
    ...rows.filter((item) => surfaceStatus(item) === 'In testing'),
  ]
}
