// Real product facts only (shared/business/design/offer-rails.md). No invented metrics.
export const brand = 'Offer Rails'
export const tagline = 'Affiliate marketing, evolved for agents.'
export const pitch = 'Makers pay only for sales. Publishers earn from more of their posts. Agents find, match and place.'
export const joinLine = 'One account can list a product and place offers. One link counts clicks wherever you paste it.'
export const painLine = 'Agents do the slow part: finding partners, matching them to content, and placing the offer.'

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


export const feed = [
  { kind: 'listed', who: 'maker', text: 'Hand-bound notebook listed', meta: '€24.00 · 10%' },
  { kind: 'matched', who: 'agent', text: 'Matched to “Journaling for beginners”', meta: 'relevance 71' },
  { kind: 'checks', who: 'agent', text: '9 of 9 hard checks passed', meta: 'auto' },
  { kind: 'placed', who: 'pub', text: 'Placed after the intro', meta: 'sponsored link' },
  { kind: 'clicked', who: 'pub', text: 'Reader clicked the go link', meta: 'no cookies' },
  { kind: 'earned', who: 'pub', text: 'Commission recorded', meta: 'publisher €2.40' },
]

export const faq = [
  { q: 'Do you hold or move the money?', a: 'No. Stripe handles checkout. Polar, Lemon Squeezy, and other payment adapters are coming. We record attribution, refunds and commission, and show whether a sale was provider-attested or seller-reported.' },
  { q: 'What does it cost?', a: 'You earn per sale: €2.40 on a €24 sale at 10%. The maker pays a separate fee of 10% of that commission: €0.24 on this example.' },
  { q: 'What does my agent share?', a: 'An offer card: the project, the category and the commission terms. Never customer names, emails or lists. Enforced in code, not policy text.' },
  { q: 'Can I take a placement back?', a: 'Yes. Undo is one tap in the digest. The box leaves the page at the plugin’s next sync (every 5 minutes) plus any page-cache delay, and clicks stop at the link right away.' },
  { q: 'What if I have no sales yet?', a: 'Later: Traffic trade. Two sites with overlapping topics could mention each other, no money involved. That is not live yet.' },
]

// Drifting hero chips: the real example only (same facts as feed/example). No invented products or metrics.
export const driftChips = [
  { who: 'maker', label: 'Listed', text: 'Hand-bound notebook · €24.00 · 10%' },
  { who: 'agent', label: 'Matched', text: '“Journaling for beginners” · relevance 71' },
  { who: 'agent', label: 'Checked', text: '9 of 9 hard checks passed' },
  { who: 'pub', label: 'Placed', text: 'After the intro · rel="sponsored"' },
  { who: 'pub', label: 'Earned', text: 'Publisher €2.40 · maker €21.60' },
] as const

// Deep-dive advantages. Every line traces to the product spec facts above.
export const makerAdvantages = [
  { k: 'found', t: 'Publishers’ agents find you', b: 'You don’t pitch blogs one by one. Their agents look for offers that match their posts.' },
  { k: 'pay', t: 'Pay only when a sale is attributed', b: 'No listing fee. No paying for clicks. The fee is 10% of the publisher commission: €0.24 on the €24 notebook example.' },
  { k: 'mcp', t: 'One install, one MCP server', b: 'One install command, pasted as is. Untested: Claude Code and Generic, both MCP over HTTP. Coming: Cursor, VS Code, Windsurf, ChatGPT and Grok.' },
  { k: 'local', t: 'Your code stays on your machine', b: 'Your agent scans pricing, payments and landing locally. It never reads .env, keys or anything gitignored, and shows you the exact listing card before the first upload.' },
  { k: 'terms', t: 'You set the terms', b: 'Pick the commission and how much control you keep: ask every time, approve within rules, or approve anything that passes the checks.' },
  { k: 'brand', t: 'Your brand is described honestly', b: 'Copy comes only from your offer data, capped at 140 characters, always labelled and rel="sponsored". Nine hard checks run in every mode.' },
  { k: 'money', t: 'We never touch the money', b: 'Stripe handles checkout. Polar, Lemon Squeezy, and other payment adapters are coming. We record attribution, refunds and commission, and mark each sale provider-attested or seller-reported.' },
  { k: 'digest', t: 'Placements, clicks and refunds in one digest', b: 'See where your offer runs. Revoke works, and a monthly cap keeps spend predictable.' },
  { k: 'privacy', t: 'No customer data leaves', b: 'Your agent shares an offer card: the project, the category and the commission terms. Never customer names, emails or lists. Enforced in code.' },
  { k: 'swap', t: 'No sales yet? Traffic trade is coming', b: 'A mention swap with a related site is not live yet. Visibility for visibility, when it is.' },
] as const

export const publisherAdvantages = [
  { k: 'free', t: 'You earn per sale: €2.40 on a €24 sale at 10%', b: 'From the content you already make. You pick each step.' },
  { k: 'one', t: 'One honest box per post', b: 'At most one offer in a post that fits, placed in the matching section, labelled and rel="sponsored".' },
  { k: 'fit', t: 'Only offers that fit', b: 'An offer needs a relevance score of 25 or higher and must be live before it can be placed. Some posts get no offer.' },
  { k: 'switch', t: 'You pick each step', b: 'Ask every time, approve within your rules (categories, minimum relevance, positions, a 30-day cap), or approve anything that passes the checks.' },
  { k: 'stricter', t: 'Agents can only tighten', b: 'Your agent can make settings stricter. Loosening a mode needs you, signed in.' },
  { k: 'undo', t: 'Undo in one tap', b: 'Clicks stop at the link right away; the box leaves the page at the plugin’s next sync (every 5 minutes) plus any page-cache delay.' },
  { k: 'readers', t: 'Readers aren’t tracked by cookies', b: 'The go link records the click without cookies. No subscriber list is shared.' },
  { k: 'surfaces', t: 'The content you already make', b: 'Paste one link in email, on X, Instagram, YouTube, or a blog. It counts clicks there. No plugin is needed for that. Posting stays with you.' },
  { k: 'swap', t: 'No sale nearby? Traffic trade is coming', b: 'A mention swap with a related site is not live yet. Visibility for visibility, when it is.' },
] as const
