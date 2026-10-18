// Real product facts only (shared/business/design/offer-rails.md). No invented metrics.
export const brand = 'Offer Rails'
export const tagline = 'Distribution your agents can negotiate.'
export const pitch = 'Makers list what they sell with a referral offer. Publishers place it in their own content. Agents on both sides do the discovery and the matchmaking. One account can be both.'
export const pilotLine = 'Offer Rails is in an early pilot with a small group of makers and sites. We start with WordPress sites. One account can both list products and publish offers.'

export const example = { product: 'Hand-bound notebook', price: '€24.00', commission: '10%', publisher: '€2.40', maker: '€21.60' }

export const hardChecks = [
  'Disclosure label next to the link',
  'rel="sponsored" on every link',
  'Claims come from the offer data only',
  'Copy capped at 140 characters',
  'One box per post',
  'Relevance score 25 or higher',
  'Offer is live',
  'Revoke works',
  'Monthly cap respected',
]

export const modes = [
  { name: 'Ask me every time', note: 'Every placement waits for your yes.' },
  { name: 'Approve within my rules', note: 'Allowed categories, minimum relevance, positions, a 30-day cap.' },
  { name: 'Approve anything that passes the checks', note: 'Your agent decides. You get a digest with one-tap undo.' },
]

export const surfaces = [
  { name: 'WordPress', status: 'First', note: 'Plugin places one sponsored box in the matching section of a post.' },
  { name: 'X', status: 'Coming', note: 'Share links and posts you approve. Posting stays with you. Not live yet.' },
  { name: 'Shopify', status: 'Coming', note: 'Storefront space for products that fit your shop. Not live yet.' },
]

export const feed = [
  { kind: 'listed', who: 'maker', text: 'Hand-bound notebook listed', meta: '€24.00 · 10%' },
  { kind: 'matched', who: 'agent', text: 'Matched to “Journaling for beginners”', meta: 'relevance 71' },
  { kind: 'checks', who: 'agent', text: '9 of 9 hard checks passed', meta: 'auto' },
  { kind: 'placed', who: 'pub', text: 'Placed after the intro', meta: 'sponsored link' },
  { kind: 'clicked', who: 'pub', text: 'Reader clicked the go link', meta: 'no cookies' },
  { kind: 'earned', who: 'pub', text: 'Commission recorded', meta: 'publisher €2.40' },
  { kind: 'exchange', who: 'agent', text: 'Traffic trade is coming', meta: 'not live' },
]

export const faq = [
  { q: 'Do you hold or move the money?', a: 'No. Stripe handles checkout. Polar, Lemon Squeezy, Paddle, and other payment adapters are coming. We record attribution, refunds and commission, and show whether a sale was provider-attested or seller-reported.' },
  { q: 'What does it cost?', a: 'During the pilot the maker pays a fee of 10% of the publisher commission. On a €24 sale with 10% commission the publisher earns €2.40 and the fee is €0.24.' },
  { q: 'What does my agent share?', a: 'An Offer card (project, category, commission terms) or an Audience card (surface, topic, size bucket). Never customer names, emails or lists. Enforced in code, not policy text.' },
  { q: 'Can I take a placement back?', a: 'Yes. Undo is one tap in the digest. The box leaves the page at the plugin’s next sync (every 5 minutes) plus any page-cache delay, and clicks stop at the link right away.' },
  { q: 'What if I have no sales yet?', a: 'Traffic trade is coming. Two sites with overlapping topics would swap a mention through the same rails, no money involved. That trade is not live yet.' },
]

// Drifting hero chips: the real example only (same facts as feed/example). No invented products or metrics.
export const driftChips = [
  { who: 'maker', label: 'Listed', text: 'Hand-bound notebook · €24.00 · 10%' },
  { who: 'agent', label: 'Matched', text: '“Journaling for beginners” · relevance 71' },
  { who: 'agent', label: 'Checked', text: '9 of 9 hard checks passed' },
  { who: 'pub', label: 'Placed', text: 'After the intro · rel="sponsored"' },
  { who: 'pub', label: 'Earned', text: 'Publisher €2.40 · maker €21.60' },
  { who: 'agent', label: 'Swap', text: 'Traffic trade is coming' },
] as const

// Deep-dive advantages. Every line traces to the product spec facts above.
export const makerAdvantages = [
  { k: 'pay', t: 'Pay only when a sale is attributed', b: 'No listing fee, no ad budget. During the pilot the fee is 10% of the publisher commission: €0.24 on the €24 notebook example.', big: true },
  { k: 'local', t: 'Your code stays on your machine', b: 'Your agent scans pricing, payments and landing locally. It never reads .env, keys or anything gitignored, and shows you the exact listing card before the first upload.' },
  { k: 'mcp', t: 'One install, one MCP server', b: 'One install command, pasted as is. Untested: Claude Code and Generic, both MCP over HTTP. Coming: Cursor, VS Code, Windsurf, ChatGPT and Grok.' },
  { k: 'terms', t: 'You set the terms', b: 'Pick the commission and how much control you keep: ask every time, approve within rules, or approve anything that passes the checks.' },
  { k: 'found', t: 'Publishers’ agents find you', b: 'Discovery runs both ways: publisher agents look for offers that match their posts, so you don’t pitch blogs one by one.' },
  { k: 'brand', t: 'Your brand is described honestly', b: 'Copy comes only from your offer data, capped at 140 characters, always labelled and rel="sponsored". Nine hard checks run in every mode.' },
  { k: 'money', t: 'We never touch the money', b: 'Stripe handles checkout. Polar, Lemon Squeezy, Paddle, and other payment adapters are coming. We record attribution, refunds and commission, and mark each sale provider-attested or seller-reported.' },
  { k: 'digest', t: 'Every placement, click and refund in one digest', b: 'See where your offer runs. Revoke works, and a monthly cap keeps spend predictable.' },
  { k: 'privacy', t: 'No customer data leaves', b: 'Your agent shares an Offer card (project, category, commission terms). Never customer names, emails or lists. Enforced in code.' },
  { k: 'swap', t: 'No sales yet? Swap visibility', b: 'Cross-promote with a related site through the same rails. Visibility for visibility, no money involved.' },
] as const

export const publisherAdvantages = [
  { k: 'free', t: 'You earn per sale', b: 'The pilot fee is paid by the maker. You earn per sale: €2.40 on a €24 sale at 10%.', big: true },
  { k: 'one', t: 'One honest box per post', b: 'At most one offer per post, placed in the matching section, labelled and rel="sponsored". Never a wall of ads.' },
  { k: 'fit', t: 'Only offers that fit', b: 'An offer needs a relevance score of 25 or higher and must be live before it can be placed.' },
  { k: 'switch', t: 'You hold the switch', b: 'Ask every time, approve within your rules (categories, minimum relevance, positions, a 30-day cap), or approve anything that passes the checks.' },
  { k: 'stricter', t: 'Agents can only tighten', b: 'Your agent can make settings stricter. Loosening a mode needs you, signed in.' },
  { k: 'undo', t: 'Undo in one tap', b: 'Clicks stop at the link right away; the box leaves the page at the plugin’s next sync (every 5 minutes) plus any page-cache delay.' },
  { k: 'readers', t: 'Readers aren’t tracked by cookies', b: 'The go link records the click without cookies. Your Audience card shares surface, topic and a size bucket, never subscriber lists.' },
  { k: 'surfaces', t: 'The space you already have', b: 'WordPress first, with the plugin. X and Shopify are coming. Posting on X would stay with you.' },
  { k: 'swap', t: 'No sale nearby? Traffic trade is coming', b: 'A mention swap with a related site is not live yet. Visibility for visibility, when it is.' },
] as const
