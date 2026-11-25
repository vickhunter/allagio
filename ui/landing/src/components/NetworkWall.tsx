import { useEffect, useState } from 'react'
import { ArrowLeftRight, ArrowRight, ArrowUpRight, Bot, CheckCircle2, FlaskConical, Radio, ShieldCheck, Users } from 'lucide-react'
import { useLocale, type Lang } from '@/i18n'
import { useSource, withSource } from '@/lib/source'
import {
  commissionText,
  dealsFixtureData,
  listPublicOffers,
  listRecentDeals,
  partyText,
  wantsDealsFixtures,
  type Face,
  type WallDeal,
  type WallOffer,
} from '@/lib/network-wall.mjs'

const ACTIVE = new Set(['active', 'completed', 'executed'])
const RECIPROCAL_KINDS = new Set(['swap', 'link_exchange', 'traffic_swap'])

const KIND_LABELS: Record<string, string> = {
  affiliate: 'Affiliate',
  sponsored: 'Sponsored',
  swap: 'Cross-Post Swap',
  link_exchange: 'Link Exchange',
  traffic_swap: 'Traffic Swap',
  partnership: 'Rev-Share',
  cross_sell: 'Cross-Sell',
  repost: 'Repost',
}

function getKindLabel(kind: string): string {
  const key = (kind || '').toLowerCase()
  return KIND_LABELS[key] ?? kind.replace(/_/g, ' ')
}

function ago(iso: string, lang: Lang) {
  const then = Date.parse(iso)
  if (Number.isNaN(then)) return ''
  const seconds = Math.round((then - Date.now()) / 1000)
  const abs = Math.abs(seconds)
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' })
  if (abs < 3600) return rtf.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(seconds / 3600), 'hour')
  return rtf.format(Math.round(seconds / 86400), 'day')
}

function buildSandboxDeals(): WallDeal[] {
  return [
    {
      kind: 'affiliate',
      state: 'active',
      updated_at: '',
      parties: 2,
      offerer: { listed: true, name: 'Vibe Sales' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'swap',
      state: 'executed',
      updated_at: '',
      parties: 2,
      offerer: { listed: false, role: 'publisher' },
      acceptor: { listed: false, role: 'publisher', host: 'agents.frontend.example' },
    },
    {
      kind: 'sponsored',
      state: 'active',
      updated_at: '',
      parties: 2,
      offerer: { listed: true, name: 'Goji Analytics' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'partnership',
      state: 'active',
      updated_at: '',
      parties: 3,
      offerer: { listed: true, name: 'Ledger App' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'link_exchange',
      state: 'awaiting',
      updated_at: '',
      parties: 2,
      offerer: { listed: false, role: 'agent', host: 'agents.example' },
      acceptor: { listed: true, name: 'Parcel Ops' },
    },
    {
      kind: 'affiliate',
      state: 'completed',
      updated_at: '',
      parties: 2,
      offerer: { listed: true, name: 'Acme Deploy' },
      acceptor: { listed: false, role: 'publisher' },
    },
  ]
}

function PartyInline({ face, roles }: { face: Face; roles: Record<string, string> }) {
  const { text, named } = partyText(face, roles)
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      {named ? (
        <CheckCircle2 aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-pub" />
      ) : (
        <Bot aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-t2" />
      )}
      <span className={`truncate ${named ? 'font-medium text-t1' : 'text-t2'}`}>{text}</span>
    </span>
  )
}

function DealRow({
  deal,
  roles,
  lang,
  sandbox = false,
}: {
  deal: WallDeal
  roles: Record<string, string>
  lang: Lang
  sandbox?: boolean
}) {
  const kindLabel = getKindLabel(deal.kind)
  const isActive = ACTIVE.has(deal.state)
  const isReciprocal = RECIPROCAL_KINDS.has((deal.kind || '').toLowerCase())

  return (
    <li
      data-deal-card
      data-deal-sandbox={sandbox ? 'true' : 'false'}
      className="grid gap-2 px-4 py-3.5 transition-colors hover:bg-s2/40 md:grid-cols-[132px_minmax(0,1fr)_180px_110px_100px] md:items-center md:gap-4"
    >
      <div className="flex items-center gap-2">
        <span className="text-xs font-semibold text-t1">{kindLabel}</span>
        {deal.parties > 2 ? (
          <span className="inline-flex items-center gap-1 font-mono text-[11px] text-t2">
            <Users aria-hidden="true" className="h-3 w-3" />
            {deal.parties}
          </span>
        ) : null}
      </div>

      <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm">
        <PartyInline face={deal.offerer} roles={roles} />
        <span className="text-t2" aria-hidden="true">
          {isReciprocal ? <ArrowLeftRight className="h-3.5 w-3.5" /> : <ArrowRight className="h-3.5 w-3.5" />}
        </span>
        <PartyInline face={deal.acceptor} roles={roles} />
      </div>

      <div className="inline-flex items-center gap-1.5 font-mono text-[11px] text-t2">
        <ShieldCheck aria-hidden="true" className={`h-3.5 w-3.5 shrink-0 ${isActive ? 'text-pub' : 'text-maker'}`} />
        <span>{isActive ? 'ORP verified' : 'Pending gate'}</span>
      </div>

      <div>
        <span className="inline-flex items-center gap-1.5 font-mono text-[11px] text-t1">
          <span
            aria-hidden="true"
            className={`h-1.5 w-1.5 rounded-full ${isActive ? 'bg-pub' : 'bg-maker'}`}
          />
          {deal.state}
        </span>
      </div>

      <div className="font-mono text-[11px] text-t2 md:text-right">
        {sandbox ? 'Sample deal' : ago(deal.updated_at, lang)}
      </div>
    </li>
  )
}

function OfferCell({ offer, joinLabel }: { offer: WallOffer; joinLabel: string }) {
  const commission = commissionText(offer)
  return (
    <li
      data-offer-card
      className="flex min-w-0 flex-col justify-between gap-2.5 border-r border-b border-line bg-s1 p-5 transition-colors hover:bg-s2/40"
    >
      <div>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            {offer.primary_category ? (
              <p className="text-xs text-t2">{offer.primary_category}</p>
            ) : null}
            <h4 className="mt-0.5 truncate font-display text-base font-bold text-t1">{offer.product_name}</h4>
          </div>
          {commission ? (
            <span className="shrink-0 font-mono text-xs font-semibold text-maker-brand">{commission}</span>
          ) : null}
        </div>
        <p className="mt-2 text-sm leading-relaxed text-t2">{offer.product_tagline}</p>
      </div>
      <a
        className="mt-2 inline-flex min-h-11 items-center gap-1 text-sm font-semibold text-t1 underline decoration-line-strong underline-offset-4 hover:decoration-t1"
        href={offer.website_url}
      >
        {joinLabel}
        <ArrowUpRight aria-hidden="true" className="h-4 w-4" />
      </a>
    </li>
  )
}

export function NetworkWall() {
  const { t, lang } = useLocale()
  const source = useSource()
  const copy = t.network
  const isFixtureMode = wantsDealsFixtures()
  const [deals, setDeals] = useState<WallDeal[] | 'loading' | 'error'>(() =>
    isFixtureMode ? (dealsFixtureData().deals as WallDeal[]) : 'loading'
  )
  const [offers, setOffers] = useState<WallOffer[]>(() =>
    isFixtureMode ? (dealsFixtureData().offers as WallOffer[]) : []
  )
  const [viewMode, setViewMode] = useState<'auto' | 'live' | 'sandbox'>('auto')
  const [sandboxDeals] = useState<WallDeal[]>(() => buildSandboxDeals())

  useEffect(() => {
    if (isFixtureMode) return
    let gone = false
    listRecentDeals()
      .then((rows) => {
        if (!gone) setDeals(rows as WallDeal[])
      })
      .catch(() => {
        if (!gone) setDeals('error')
      })
    listPublicOffers()
      .then((rows) => {
        if (!gone) setOffers(rows as WallOffer[])
      })
      .catch(() => {})
    return () => {
      gone = true
    }
  }, [isFixtureMode])

  const roles = copy.roles as Record<string, string>
  const liveRows = Array.isArray(deals) ? deals : []
  const liveEmptyOrUnavailable = deals === 'error' || (Array.isArray(deals) && liveRows.length === 0)
  const showingSandbox = !isFixtureMode && (viewMode === 'sandbox' || (viewMode === 'auto' && liveEmptyOrUnavailable))
  const displayRows = showingSandbox ? sandboxDeals : liveRows

  return (
    <section id="network" aria-labelledby="network-h">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="max-w-2xl">
            <h2 id="network-h" className="h-display text-3xl sm:text-4xl">
              {showingSandbox ? 'How deals look on the wire' : copy.heading}
            </h2>
            <p className="mt-3 text-base leading-relaxed text-t2">
              {showingSandbox
                ? 'Sample activity illustrating the parties, states, and ORP verification metadata recorded when agents transact.'
                : copy.lead}
            </p>
          </div>

          {!isFixtureMode ? (
            <div
              role="group"
              aria-label="Network activity source"
              className="inline-flex shrink-0 items-center gap-0.5 self-start rounded-md border border-line bg-s1 p-0.5 md:self-auto"
            >
              <button
                type="button"
                data-wall-mode="live"
                aria-pressed={!showingSandbox}
                onClick={() => setViewMode('live')}
                className={`inline-flex min-h-11 items-center gap-1.5 rounded-[5px] px-3 text-xs font-medium transition-colors ${
                  !showingSandbox ? 'bg-t1 text-s0' : 'text-t2 hover:text-t1'
                }`}
              >
                <Radio aria-hidden="true" className="h-3.5 w-3.5" />
                Live wire
              </button>
              <button
                type="button"
                data-wall-mode="sandbox"
                aria-pressed={showingSandbox}
                onClick={() => setViewMode('sandbox')}
                className={`inline-flex min-h-11 items-center gap-1.5 rounded-[5px] px-3 text-xs font-medium transition-colors ${
                  showingSandbox ? 'bg-s2 text-t1' : 'text-t2 hover:text-t1'
                }`}
              >
                <FlaskConical aria-hidden="true" className="h-3.5 w-3.5" />
                Sample (sandbox)
              </button>
            </div>
          ) : null}
        </div>

        {isFixtureMode ? (
          <p className="mt-4 inline-flex min-h-11 items-center rounded-md border border-line-strong bg-s2 px-3 font-mono text-sm text-t1">
            TEST data
          </p>
        ) : null}

        <div className="mt-8 flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-t1">{copy.dealsLabel}</h3>
          {showingSandbox ? (
            <span
              data-sandbox-banner
              className="inline-flex items-center gap-1.5 rounded-md border border-line bg-s1 px-2.5 py-1 text-xs text-t2"
            >
              <FlaskConical aria-hidden="true" className="h-3.5 w-3.5 text-maker" />
              Sample network activity (sandbox)
            </span>
          ) : liveRows.length > 0 ? (
            <span className="inline-flex items-center gap-1.5 rounded-md border border-pub/30 bg-pub-soft px-2.5 py-1 font-mono text-xs text-pub">
              <Radio aria-hidden="true" className="h-3.5 w-3.5" />
              {liveRows.length} live on the wire
            </span>
          ) : null}
        </div>

        {!showingSandbox && deals === 'loading' ? (
          <p className="mt-4 text-sm text-t2" role="status">{copy.loading}</p>
        ) : null}
        {!showingSandbox && deals === 'error' ? (
          <p className="mt-4 text-sm text-t1" role="alert">{copy.unavailable}</p>
        ) : null}
        {!showingSandbox && Array.isArray(deals) && liveRows.length === 0 ? (
          <p className="mt-4 text-sm text-t1" role="status">{copy.quiet}</p>
        ) : null}

        {displayRows.length > 0 ? (
          <div className="mt-3 overflow-hidden rounded-lg border border-line bg-s1">
            <div
              aria-hidden="true"
              className="hidden border-b border-line bg-s2/50 px-4 py-2 text-xs font-medium text-t2 md:grid md:grid-cols-[132px_minmax(0,1fr)_180px_110px_100px] md:gap-4"
            >
              <span>Kind</span>
              <span>Counterparties</span>
              <span>Verification</span>
              <span>Status</span>
              <span className="text-right">Record</span>
            </div>
            <ul className="divide-y divide-line">
              {displayRows.map((deal, i) => (
                <DealRow key={`${deal.kind}-${deal.updated_at}-${i}`} deal={deal} roles={roles} lang={lang} sandbox={showingSandbox} />
              ))}
            </ul>
          </div>
        ) : null}

        {offers.length > 0 ? (
          <>
            <h3 className="mt-12 text-sm font-semibold text-t1">{copy.offersLabel}</h3>
            <ul className="mt-3 grid grid-cols-1 overflow-hidden rounded-lg border-t border-l border-line sm:grid-cols-2 lg:grid-cols-3">
              {offers.slice(0, 6).map((offer) => (
                <OfferCell key={offer.offer_id} offer={offer} joinLabel={copy.joinLabel} />
              ))}
            </ul>
          </>
        ) : null}

        <p className="mt-8 flex flex-wrap gap-5 text-sm">
          <a
            className="inline-flex min-h-11 items-center font-semibold underline decoration-line-strong underline-offset-4 hover:decoration-t1"
            href={withSource('/makers/', source)}
          >
            {copy.directoryLink}
          </a>
          <a
            className="inline-flex min-h-11 items-center font-semibold underline decoration-line-strong underline-offset-4 hover:decoration-t1"
            href={withSource('/agents', source)}
          >
            {copy.agentsLink}
          </a>
        </p>
      </div>
    </section>
  )
}
