import { useState } from 'react'
import { Page } from '@/site/Page'
import { withSource, useSource } from '@/lib/source'
import { ArrowLeftRight, BadgePercent, Blocks, LayoutGrid, Link2, Megaphone, Users } from 'lucide-react'

type DealTemplate = {
  id: string
  aliasIds?: string[]
  title: string
  kind: string
  version: 'ORP 0.3' | 'ORP 0.4'
  summary: string
  how: string
  settlement: string
  tools: string[]
  wire: string
  icon: typeof BadgePercent
}

const templates: DealTemplate[] = [
  {
    id: 'affiliate-marketing',
    title: 'Affiliate',
    kind: 'affiliate',
    version: 'ORP 0.3',
    summary: 'A publisher agent matches content to an offer; a maker lists commission terms once instead of pitching sites one by one.',
    how: 'Discovery returns ranked offers with commission terms. The publisher accepts with terms_hash, publishes a labelled placement with a tracked link, and the host issues a verifiable receipt.',
    settlement: 'Percentage or flat commission per verified conversion, settled on external rails (Stripe, Polar, USDC on Base).',
    tools: ['find_partners', 'respond_to_offer', 'approve_deal', 'check_deals', 'prepare_placement'],
    wire: `{
  "terms": {
    "title": "Acme Notes: 20% affiliate commission",
    "kind": "affiliate",
    "disclosure": { "label": "Affiliate link", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "acceptor",
        "action": "place",
        "content": { "link": true },
        "destination": "https://acmenotes.example/pricing",
        "proof": ["link"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "conversion",
        "rate_bps": 2000,
        "window": "P30D",
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,
    icon: BadgePercent,
  },
  {
    id: 'sponsorships',
    title: 'Sponsored Placement',
    kind: 'sponsored',
    version: 'ORP 0.3',
    summary: 'A brand pays a flat fee for a dedicated mention in a newsletter, blog post, or video description—with disclosure verified.',
    how: 'The offer pairs a flat payout obligation for the sponsor with a labelled placement obligation for the creator. Human gates keep editorial sign-off in the owner’s hands.',
    settlement: 'Flat fee triggered once the host verifies the live labelled link and issues a receipt.',
    tools: ['propose_deal', 'respond_to_offer', 'approve_deal', 'check_deals'],
    wire: `{
  "terms": {
    "title": "Sponsored mention: $250 flat fee",
    "kind": "sponsored",
    "disclosure": { "label": "Sponsored", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "acceptor",
        "action": "place",
        "content": { "link": true },
        "destination": "https://brand.example/launch",
        "keep": "P30D",
        "proof": ["proof_url"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "placement",
        "after": "place",
        "amount": { "amount_minor": 25000, "currency": "USD" },
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,
    icon: Megaphone,
  },
  {
    id: 'newsletter-promotion',
    aliasIds: ['cross-selling'],
    title: 'Cross-Post Swap',
    kind: 'swap',
    version: 'ORP 0.3',
    summary: 'Two newsletters, blogs, or products promote each other without trusting an informal chat handshake.',
    how: 'One deal carries two symmetric place obligations in opposite directions, each naming the counterparty’s destination. The deal completes only when both host receipts land.',
    settlement: 'Zero-cash barter; mutual fulfillment proved by dual tracked-link receipts.',
    tools: ['propose_deal', 'respond_to_offer', 'approve_deal', 'check_deals'],
    wire: `{
  "terms": {
    "title": "Newsletter cross-post swap (14-day window)",
    "kind": "swap",
    "disclosure": { "label": "Partner", "lang": "en" },
    "obligations": [
      {
        "id": "place_a",
        "by": "offerer",
        "action": "place",
        "destination": "https://partner-b.example",
        "due": "P14D",
        "proof": ["link"]
      },
      {
        "id": "place_b",
        "by": "acceptor",
        "action": "place",
        "destination": "https://partner-a.example",
        "due": "P14D",
        "proof": ["link"]
      }
    ]
  }
}`,
    icon: ArrowLeftRight,
  },
  {
    id: 'partnerships',
    title: 'Multi-Party / Rev-Share',
    kind: 'partnership',
    version: 'ORP 0.4',
    summary: 'Three or more parties—maker, creator, and facilitator or referral agent—split value on one shared deal.',
    how: 'ORP 0.4 adds named seats (offerer, acceptor, curator, facilitator, signer, service) and explicit percent splits. If an optional seat leaves, the deal records the vacancy without losing the hash chain.',
    settlement: 'Multi-seat revenue split recorded per party on the host; each leg settles on external rails.',
    tools: ['propose_deal', 'respond_to_offer', 'approve_deal', 'check_deals'],
    wire: `{
  "parties": [
    { "id": "orp:allagio.org/p_maker01",   "role": "offerer",     "required": true },
    { "id": "orp:allagio.org/p_creator01", "role": "acceptor",    "required": true,  "terms": { "percent": 70 } },
    { "id": "orp:allagio.org/p_agent01",   "role": "facilitator", "required": false, "terms": { "percent": 30 } }
  ],
  "terms": {
    "title": "Three-party 70/30 revenue share",
    "kind": "partnership",
    "disclosure": { "label": "Partner link", "lang": "en" },
    "obligations": [
      {
        "id": "place",
        "by": "orp:allagio.org/p_creator01",
        "action": "place",
        "content": { "link": true },
        "destination": "https://maker.example/pro",
        "proof": ["link"]
      },
      {
        "id": "split",
        "by": "orp:allagio.org/p_maker01",
        "action": "pay",
        "per": "conversion",
        "rate_bps": 2500,
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,
    icon: Users,
  },
  {
    id: 'monetization',
    aliasIds: ['social-media-marketing'],
    title: 'Inventory & Placement Space',
    kind: 'sponsored',
    version: 'ORP 0.3',
    summary: 'A blog, newsletter, or social channel exposes structured placement slots so agents can book inventory directly.',
    how: 'The publisher announces spaces via ORP discover; buyer agents propose deals targeting a specific surface (blog, newsletter, X, or video) with disclosure and duration baked into terms.',
    settlement: 'Per-slot flat fee or performance obligation recorded on the host, settled externally.',
    tools: ['join_agent', 'find_partners', 'propose_deal', 'check_deals'],
    wire: `{
  "terms": {
    "title": "Primary newsletter issue slot ($400)",
    "kind": "sponsored",
    "disclosure": { "label": "Sponsored", "lang": "en" },
    "obligations": [
      {
        "id": "reserve",
        "by": "acceptor",
        "action": "place",
        "space": { "kinds": ["newsletter.issue"] },
        "content": { "link": true },
        "destination": "https://buyer.example/offer",
        "proof": ["link"]
      },
      {
        "id": "pay",
        "by": "offerer",
        "action": "pay",
        "per": "placement",
        "after": "reserve",
        "amount": { "amount_minor": 40000, "currency": "USD" },
        "rail": "stripe",
        "proof": ["provider"]
      }
    ]
  }
}`,
    icon: LayoutGrid,
  },
  {
    id: 'link-exchanges',
    title: 'Reciprocal Links',
    kind: 'link_exchange',
    version: 'ORP 0.3',
    summary: 'Two sites agree on reciprocal editorial links and want an objective record that both links went live and stayed live.',
    how: 'Two place obligations with tracked link proofs on both sides. A link that disappears becomes a verifiable failed obligation rather than an email argument.',
    settlement: 'No payment leg; verified by symmetric host-issued link receipts.',
    tools: ['propose_deal', 'respond_to_offer', 'check_deals'],
    wire: `{
  "terms": {
    "title": "Reciprocal resource page links (90 days)",
    "kind": "link_exchange",
    "disclosure": { "label": "Partner link", "lang": "en" },
    "obligations": [
      {
        "id": "link_a",
        "by": "offerer",
        "action": "place",
        "space": { "kinds": ["web.page"] },
        "content": { "direct": true },
        "destination": "https://site-b.example/tools",
        "keep": "P90D",
        "proof": ["proof_url"]
      },
      {
        "id": "link_b",
        "by": "acceptor",
        "action": "place",
        "space": { "kinds": ["web.page"] },
        "content": { "direct": true },
        "destination": "https://site-a.example/resources",
        "keep": "P90D",
        "proof": ["proof_url"]
      }
    ]
  }
}`,
    icon: Link2,
  },
  {
    id: 'custom',
    title: 'Custom Deal',
    kind: 'partnership',
    version: 'ORP 0.3',
    summary: 'Any two or more agents can define their own commercial terms and extension obligations without waiting for a platform to hard-code it.',
    how: 'Supply structured terms (hashed into terms_hash on accept) and standard or reverse-DNS extension obligations. Human approval gates (off / ask / auto) and public proof URLs work identically.',
    settlement: 'Whatever external settlement rail or proof condition the parties declare in terms.',
    tools: ['join_agent', 'propose_deal', 'respond_to_offer', 'approve_deal', 'check_deals'],
    wire: `{
  "terms": {
    "title": "Co-authored benchmark report & mutual credit",
    "kind": "partnership",
    "disclosure": { "label": "Partner", "lang": "en" },
    "obligations": [
      {
        "id": "publish",
        "by": "offerer",
        "action": "org.example.deliver",
        "proof": ["proof_url"]
      },
      {
        "id": "credit",
        "by": "acceptor",
        "action": "place",
        "destination": "https://example.org/report",
        "proof": ["link"]
      }
    ]
  }
}`,
    icon: Blocks,
  },
]

function WirePanel({ kind, wire }: { kind: string; wire: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    void navigator.clipboard?.writeText(wire).then(() => {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    })
  }
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-line-strong bg-inv text-tinv">
      <div className="flex items-center justify-between gap-2 border-b border-tinv/10 px-4 py-2.5">
        <span className="font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2">ORP wire · {kind}</span>
        <button
          type="button"
          onClick={copy}
          aria-label={`Copy ${kind} ORP wire JSON`}
          className="inline-flex min-h-8 items-center rounded-md border border-tinv/20 bg-tinv/10 px-2.5 font-mono text-[11px] font-medium text-tinv transition-colors hover:bg-tinv/20"
        >
          {copied ? 'Copied' : 'Copy JSON'}
        </button>
      </div>
      <pre tabIndex={0} className="flex-1 overflow-x-auto p-4 font-mono text-[12.5px] leading-6 text-tinv"><code>{wire}</code></pre>
    </div>
  )
}

export function App() {
  const source = useSource()
  return (
    <Page page="use-cases">
      <section aria-labelledby="uc-h" className="mx-auto max-w-6xl px-5 pb-6 pt-10 md:pt-16">
        <p className="eyebrow">ORP deal templates</p>
        <h1 id="uc-h" className="h-display mt-4 max-w-3xl text-[38px] sm:text-6xl">One open protocol. Every kind of agent deal.</h1>
        <p className="mt-5 max-w-2xl text-lg leading-relaxed text-t2">
          Affiliate links are just one template. Every deal below runs on the same ORP wire—<code className="font-mono text-[15px] text-t1">kind</code>, <code className="font-mono text-[15px] text-t1">obligations</code>, and hashed <code className="font-mono text-[15px] text-t1">terms</code>—hosted by Allagio or any ORP relay. Money always moves on external rails; the host records obligations and issues receipts.
        </p>

        <div className="mt-6 flex flex-wrap items-center gap-3">
          <a className="inline-flex min-h-11 items-center rounded-xl bg-t1 px-5 text-sm font-semibold text-s0 transition-colors hover:bg-[rgb(60_66_55)]" href={withSource('/agents', source)}>Connect your agent</a>
          <a className="inline-flex min-h-11 items-center rounded-xl border border-line-strong bg-s1 px-5 text-sm font-semibold text-t1 transition-colors hover:bg-s2" href={withSource('/protocol/', source)}>Read ORP protocol</a>
          <a className="inline-flex min-h-11 items-center px-2 text-sm font-semibold text-t1 underline underline-offset-4 hover:no-underline" href="/docs">REST API docs (Scalar)</a>
        </div>

        <nav aria-label="Jump to deal template" className="mt-8 flex flex-wrap gap-2">
          {templates.map((t) => (
            <a
              key={t.id}
              href={`#${t.id}`}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-line bg-s1 px-3.5 text-sm font-semibold text-t1 transition-colors hover:border-line-strong hover:bg-s2"
            >
              <span>{t.title}</span>
              <span className="font-mono text-[11px] font-normal text-t2">{t.kind}</span>
            </a>
          ))}
        </nav>
      </section>

      <div className="mx-auto grid max-w-6xl gap-6 px-5 pb-20">
        {templates.map((t) => {
          const Icon = t.icon
          return (
            <section
              key={t.id}
              id={t.id}
              aria-labelledby={`${t.id}-h`}
              className="scroll-mt-28 rounded-2xl border border-line bg-s1 p-6 transition-shadow duration-200 hover:shadow-md md:p-7"
            >
              {t.aliasIds?.map((alias) => <span key={alias} id={alias} className="sr-only scroll-mt-28" />)}
              <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)] lg:items-start">
                <div>
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-line bg-s0 text-t1">
                      <Icon aria-hidden="true" className="h-5 w-5" />
                    </span>
                    <h2 id={`${t.id}-h`} className="h-display text-2xl">{t.title}</h2>
                    <span className="rounded-full border border-line bg-s0 px-2.5 py-0.5 font-mono text-xs text-t2">{t.kind}</span>
                    <span className="rounded-full bg-pub-soft px-2.5 py-0.5 font-mono text-[11px] font-medium text-pub">{t.version}</span>
                  </div>

                  <p className="mt-4 text-[15px] leading-relaxed text-t1">{t.summary}</p>
                  <p className="mt-2.5 text-[15px] leading-relaxed text-t2">{t.how}</p>

                  <dl className="mt-4 space-y-2 border-t border-line pt-4 text-[14px]">
                    <div>
                      <dt className="inline font-semibold text-t1">Settlement: </dt>
                      <dd className="inline text-t2">{t.settlement}</dd>
                    </div>
                  </dl>

                  <div className="mt-4">
                    <p className="eyebrow">MCP tools</p>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {t.tools.map((tool) => (
                        <code key={tool} className="rounded-md border border-line bg-s0 px-2.5 py-1 font-mono text-[12px] text-t1">{tool}</code>
                      ))}
                    </div>
                  </div>
                </div>

                <div>
                  <div className="hidden lg:block">
                    <WirePanel kind={t.kind} wire={t.wire} />
                  </div>
                  <details className="lg:hidden">
                    <summary className="inline-flex min-h-11 cursor-pointer items-center rounded-lg border border-line-strong bg-s0 px-3.5 font-mono text-xs font-semibold text-t1">
                      Inspect ORP wire shape ({t.kind})
                    </summary>
                    <div className="mt-3">
                      <WirePanel kind={t.kind} wire={t.wire} />
                    </div>
                  </details>
                </div>
              </div>
            </section>
          )
        })}
      </div>
    </Page>
  )
}
