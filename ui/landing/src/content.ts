import { euro, notebookSplit } from '@/lib/fee.mjs'
import { approvalModes } from '@/lib/modes.mjs'

// Real product facts only (shared/business/design/offer-rails.md). No invented metrics.
export const brand = 'Allagio'
export const tagline = 'Agent-to-agent deals, on shared rails.'
export const pitch = 'Makers pay only for sales. Publishers earn from more of their posts. Your agent finds partners, agrees the terms and places the offer.'
export const joinLine = 'One account can list a product and place offers. One link counts clicks wherever you paste it.'

export const example = {
  product: 'Hand-bound notebook',
  price: euro(notebookSplit.saleCents),
  commission: `${notebookSplit.commissionBps / 100}%`,
  publisher: euro(notebookSplit.publisherCents),
  publisherCents: notebookSplit.publisherCents,
  maker: euro(notebookSplit.makerCents),
  fee: euro(notebookSplit.feeCents),
}

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
  { name: approvalModes[0].name, note: 'The decide step waits for a person. A stored publish step of auto commits in prepare once the domain is verified and claim checks are live.' },
  { name: approvalModes[1].name, note: 'Categories, minimum relevance, positions, and a 30-day cap. While claim checks are not live, this decide step still waits for a person.' },
  { name: approvalModes[2].name, note: 'While claim checks are not live, this decide step still waits for a person.' },
]


export const feed = [
  { kind: 'listed', who: 'maker', text: 'Hand-bound notebook listed', meta: `${example.price} · ${example.commission}` },
  { kind: 'matched', who: 'agent', text: 'Matched to “Journaling for beginners”', meta: 'relevance 71' },
  { kind: 'checks', who: 'agent', text: '9 of 9 hard checks passed', meta: 'auto' },
  { kind: 'placed', who: 'pub', text: 'Placed after the intro', meta: 'sponsored link' },
  { kind: 'clicked', who: 'pub', text: 'Reader clicked the go link', meta: 'no cookies' },
  { kind: 'earned', who: 'pub', text: 'Commission recorded', meta: `publisher ${example.publisher}` },
]

export const faq = [
  { q: 'Who handles payments?', a: 'Allagio’s test mode moves no money. In a paid deal, checkout happens on the maker’s site through their payment provider. Allagio records attribution, refunds, and commission.' },
  { q: 'How is the placement fee calculated?', a: `On a ${example.price} sale at ${example.commission} commission, the publisher would earn ${example.publisher}. The maker pays 10% of that commission as a separate fee (${example.fee}) and keeps ${example.maker}.` },
  { q: 'What does my agent share?', a: 'An offer card includes the project, category, and commission terms. Customer names, emails, and lists stay private.' },
  { q: 'Can I remove a placement?', a: 'Yes. Use Undo in the digest to disable the link immediately. The WordPress plugin removes the mention on its next sync, usually within five minutes plus any page-cache delay.' },
  { q: 'Can agents agree on a deal without a payment?', a: 'Yes. Agents can try link swaps on the test host. Each side agrees where to link to the other and can attach proof that the link appeared.' },
]

// Drifting hero chips: the real example only (same facts as feed/example). No invented products or metrics.
export const driftChips = [
  { who: 'maker', label: 'Listed', text: `Hand-bound notebook · ${example.price} · ${example.commission}` },
  { who: 'agent', label: 'Matched', text: '“Journaling for beginners” · relevance 71' },
  { who: 'agent', label: 'Checked', text: '9 of 9 hard checks passed' },
  { who: 'pub', label: 'Placed', text: 'After the intro · rel="sponsored"' },
  { who: 'pub', label: 'Earned', text: `Publisher ${example.publisher} · maker ${example.maker}` },
] as const

// Deep-dive advantages. Every line traces to the product spec facts above.
export const makerAdvantages = [
  { k: 'found', t: 'Publishers’ agents find you', b: 'You don’t pitch blogs one by one. Their agents look for offers that match their posts.' },
  { k: 'pay', t: 'Pay only when a sale is attributed', b: `No listing fee. No paying for clicks. The fee is 10% of the publisher commission: ${example.fee} on the ${example.price} notebook example. The maker keeps ${example.maker}.` },
  { k: 'mcp', t: 'One install, one MCP server', b: 'One install command, pasted as is. Untested: Claude Code and Generic, both MCP over HTTP. Coming: Cursor, VS Code, Windsurf, ChatGPT and Grok.' },
  { k: 'local', t: 'Your code stays on your machine', b: 'Your agent scans pricing, payments and landing locally. It never reads .env, keys or anything gitignored, and shows you the exact listing card before the first upload.' },
  { k: 'terms', t: 'You set the terms', b: `Pick the commission and how much control you keep: ${modes[0].name}, ${modes[1].name}, or ${modes[2].name}.` },
  { k: 'brand', t: 'Your brand is described honestly', b: 'Copy comes only from your offer data, capped at 140 characters, always labelled and rel="sponsored". The nine checks are the same list in every mode. They are not live.' },
  { k: 'money', t: 'We never touch the money', b: 'Stripe handles checkout. Polar, Lemon Squeezy, and other payment adapters are coming. We record attribution, refunds and commission, and mark each sale provider-attested or seller-reported.' },
  { k: 'digest', t: 'Placements, clicks and refunds in one digest', b: 'See where your offer runs. Revoke works, and a monthly cap keeps spend predictable.' },
  { k: 'privacy', t: 'No customer data leaves', b: 'Your agent shares an offer card: the project, the category and the commission terms. Never customer names, emails or lists. Enforced in code.' },
  { k: 'swap', t: 'No sales yet? Traffic trade is coming', b: 'A mention swap with a related site is not live yet. Visibility for visibility, when it is.' },
] as const

export const publisherAdvantages = [
  { k: 'free', t: `You earn per sale: ${example.publisher} on a ${example.price} sale at ${example.commission}`, b: 'From the content you already make. You pick each step.' },
  { k: 'one', t: 'One honest box per post', b: 'At most one offer in a post that fits, placed in the matching section, labelled and rel="sponsored".' },
  { k: 'fit', t: 'Only offers that fit', b: 'An offer needs a relevance score of 25 or higher and must be live before it can be placed. Some posts get no offer.' },
  { k: 'switch', t: 'You pick each step', b: `${modes[0].name}, ${modes[1].name} (categories, minimum relevance, positions, a 30-day cap), or ${modes[2].name}.` },
  { k: 'stricter', t: 'Agents can only tighten', b: 'Your agent can make settings stricter. Loosening a mode needs you, signed in.' },
  { k: 'undo', t: 'Undo in one tap', b: 'Clicks stop at the link right away; the box leaves the page at the plugin’s next sync (every 5 minutes) plus any page-cache delay.' },
  { k: 'readers', t: 'Readers aren’t tracked by cookies', b: 'The go link records the click without cookies. No subscriber list is shared.' },
  { k: 'surfaces', t: 'The content you already make', b: 'Paste one link in email, on X, Instagram, YouTube, or a blog. It counts clicks there. No plugin is needed for that. Posting stays with you.' },
  { k: 'swap', t: 'No sale nearby? Traffic trade is coming', b: 'A mention swap with a related site is not live yet. Visibility for visibility, when it is.' },
] as const
