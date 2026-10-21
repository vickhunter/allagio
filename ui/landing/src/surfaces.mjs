// TODO(D-092): read Live, Beta, and Planned from the adapters registry when that delivery exposes one.
// Until then this file is the only list the landing pages use.

export const surfaces = [
  { name: 'WordPress', state: 'Live', audience: 'both', note: 'The plugin places one labelled box in a post that fits.' },
  { name: 'X', state: 'Planned', audience: 'both', note: 'Share links you approve. Posting stays with you. Not live yet.' },
  { name: 'Newsletter', state: 'Planned', audience: 'publisher', note: 'Not live yet.' },
  { name: 'YouTube', state: 'Planned', audience: 'publisher', note: 'Not live yet.' },
  { name: 'Instagram', state: 'Planned', audience: 'publisher', note: 'Not live yet.' },
  { name: 'TikTok', state: 'Planned', audience: 'publisher', note: 'Not live yet.' },
  { name: 'Shopify', state: 'Planned', audience: 'maker', note: 'A shop can list products that fit. Not live yet.' },
]

export function surfacesFor(audience) {
  return surfaces.filter((item) => item.audience === audience || item.audience === 'both')
}

export function splitSurfaces(audience) {
  const rows = surfacesFor(audience)
  return {
    live: rows.filter((item) => item.state === 'Live' || item.state === 'Beta'),
    planned: rows.filter((item) => item.state === 'Planned'),
  }
}
