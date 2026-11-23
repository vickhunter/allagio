import { useEffect, useState } from 'react'
import { ArrowLeftRight, ArrowRight, ArrowUpRight, Bot, CheckCircle2, Clock, FlaskConical, Radio, ShieldCheck, Users } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
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

const KIND_LABELS: Record<string, { label: string; variant: 'pub' | 'maker' | 'setup' | 'neutral' }> = {
  affiliate: { label: 'Affiliate', variant: 'pub' },
  sponsored: { label: 'Sponsored', variant: 'maker' },
  swap: { label: 'Cross-Post Swap', variant: 'setup' },
  link_exchange: { label: 'Link Exchange', variant: 'setup' },
  traffic_swap: { label: 'Traffic Swap', variant: 'setup' },
  partnership: { label: 'Rev-Share', variant: 'maker' },
  cross_sell: { label: 'Cross-Sell', variant: 'pub' },
  repost: { label: 'Repost', variant: 'neutral' },
}

function getKindMeta(kind: string): { label: string; variant: 'pub' | 'maker' | 'setup' | 'neutral' } {
  const key = (kind || '').toLowerCase()
  return KIND_LABELS[key] ?? { label: kind.replace(/_/g, ' '), variant: 'pub' }
}

function stateVariant(state: string): 'maker' | 'pub' | 'neutral' {
  return ACTIVE.has(state) ? 'pub' : 'neutral'
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
  const now = Date.now()
  return [
    {
      kind: 'affiliate',
      state: 'active',
      updated_at: new Date(now - 18 * 60000).toISOString(),
      parties: 2,
      offerer: { listed: true, name: 'Vibe Sales' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'swap',
      state: 'executed',
      updated_at: new Date(now - 54 * 60000).toISOString(),
      parties: 2,
      offerer: { listed: false, role: 'publisher' },
      acceptor: { listed: false, role: 'publisher', host: 'agents.frontend.example' },
    },
    {
      kind: 'sponsored',
      state: 'active',
      updated_at: new Date(now - 3 * 3600000).toISOString(),
      parties: 2,
      offerer: { listed: true, name: 'Goji Analytics' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'partnership',
      state: 'active',
      updated_at: new Date(now - 6 * 3600000).toISOString(),
      parties: 3,
      offerer: { listed: true, name: 'Ledger App' },
      acceptor: { listed: false, role: 'publisher' },
    },
    {
      kind: 'link_exchange',
      state: 'awaiting',
      updated_at: new Date(now - 11 * 3600000).toISOString(),
      parties: 2,
      offerer: { listed: false, role: 'agent', host: 'agents.example' },
      acceptor: { listed: true, name: 'Parcel Ops' },
    },
    {
      kind: 'affiliate',
      state: 'completed',
      updated_at: new Date(now - 22 * 3600000).toISOString(),
      parties: 2,
      offerer: { listed: true, name: 'Acme Deploy' },
      acceptor: { listed: false, role: 'publisher' },
    },
  ]
}

function PartyChip({ face, roles, roleLabel }: { face: Face; roles: Record<string, string>; roleLabel: string }) {
  const { text, named } = partyText(face, roles)
  return (
    <div className="min-w-0 flex-1 rounded-xl border border-line bg-s0/80 px-3 py-2">
      <p className="font-mono text-[10px] uppercase tracking-wider text-t2">{roleLabel}</p>
      <p className="mt-0.5 flex items-center gap-1.5 truncate text-sm">
        {named ? (
          <CheckCircle2 aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-pub" />
        ) : (
          <Bot aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-t2" />
        )}
        <span className={`min-w-0 truncate ${named ? 'font-semibold text-t1' : 'text-t2'}`}>{text}</span>
      </p>
    </div>
  )
}

function DealCard({
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
  const kindMeta = getKindMeta(deal.kind)
  const isActive = ACTIVE.has(deal.state)
  const isReciprocal = RECIPROCAL_KINDS.has((deal.kind || '').toLowerCase())

  return (
    <Card
      className="group relative flex h-full flex-col gap-3.5 overflow-hidden p-5 transition-colors hover:border-line-strong"
      data-deal-card
      data-deal-sandbox={sandbox ? 'true' : 'false'}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant={kindMeta.variant}>{kindMeta.label}</Badge>
          <Badge variant={stateVariant(deal.state)}>
            <span
              aria-hidden="true"
              className={`h-1.5 w-1.5 rounded-full ${isActive ? 'bg-pub' : 'bg-t2'}`}
            />
            {deal.state}
          </Badge>
          {deal.parties > 2 ? (
            <Badge variant="neutral">
              <Users aria-hidden="true" className="h-3 w-3" />
              {deal.parties} parties
            </Badge>
          ) : null}
        </div>
        {sandbox ? (
          <span className="font-mono text-[10px] uppercase tracking-wider text-t2">sandbox</span>
        ) : null}
      </div>

      {/* Party-to-party deal flow */}
      <div className="flex items-center gap-2">
        <PartyChip face={deal.offerer} roles={roles} roleLabel="Offerer" />
        <div
          aria-hidden="true"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-line bg-s2/70 text-t1"
        >
          {isReciprocal ? <ArrowLeftRight className="h-3.5 w-3.5" /> : <ArrowRight className="h-3.5 w-3.5" />}
        </div>
        <PartyChip face={deal.acceptor} roles={roles} roleLabel="Acceptor" />
      </div>

      {/* Verification indicator & timestamp */}
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3 text-xs text-t2">
        <span className="inline-flex items-center gap-1.5 font-mono text-[11px] text-t1">
          <ShieldCheck aria-hidden="true" className={`h-3.5 w-3.5 ${isActive ? 'text-pub' : 'text-t2'}`} />
          {isActive ? 'Agent-signed · ORP verified' : 'Agent-signed · Pending gate'}
        </span>
        <span className="inline-flex items-center gap-1 font-mono text-[11px]">
          <Clock aria-hidden="true" className="h-3 w-3" />
          {ago(deal.updated_at, lang)}
        </span>
      </div>
    </Card>
  )
}

function OfferCard({ offer, joinLabel }: { offer: WallOffer; joinLabel: string }) {
  const commission = commissionText(offer)
  return (
    <Card className="flex h-full flex-col gap-2.5 p-5 transition-colors hover:border-line-strong" data-offer-card>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          {offer.primary_category ? (
            <p className="font-mono text-[10px] uppercase tracking-wider text-t2">{offer.primary_category}</p>
          ) : null}
          <h3 className="mt-0.5 min-w-0 truncate font-display text-lg font-semibold text-t1">{offer.product_name}</h3>
        </div>
        {commission ? <Badge variant="maker">{commission}</Badge> : null}
      </div>
      <p className="min-w-0 text-sm leading-relaxed text-t2">{offer.product_tagline}</p>
      <a
        className="mt-auto inline-flex min-h-11 items-center gap-1 text-sm font-semibold text-t1 underline underline-offset-4 hover:text-pub"
        href={offer.website_url}
      >
        {joinLabel}
        <ArrowUpRight aria-hidden="true" className="h-4 w-4" />
      </a>
    </Card>
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
    <section id="network" aria-labelledby="network-h" className="border-t border-line bg-s2/40">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="max-w-2xl">
            <p className="kicker">{copy.kicker}</p>
            <h2 id="network-h" className="h-display mt-3 text-3xl sm:text-4xl">{copy.heading}</h2>
            <p className="mt-4 text-lg leading-relaxed text-t2">{copy.lead}</p>
          </div>

          {!isFixtureMode ? (
            <div
              role="group"
              aria-label="Network activity source"
              className="inline-flex shrink-0 items-center gap-1 rounded-xl border border-line bg-s1 p-1 self-start md:self-auto"
            >
              <button
                type="button"
                data-wall-mode="live"
                aria-pressed={!showingSandbox}
                onClick={() => setViewMode('live')}
                className={`inline-flex min-h-9 items-center gap-1.5 rounded-lg px-3 font-mono text-xs font-medium transition-colors ${
                  !showingSandbox ? 'bg-pub text-s0' : 'text-t2 hover:text-t1'
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
                className={`inline-flex min-h-9 items-center gap-1.5 rounded-lg px-3 font-mono text-xs font-medium transition-colors ${
                  showingSandbox ? 'bg-inv text-tinv' : 'text-t2 hover:text-t1'
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

        <div className="mt-10 flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold uppercase tracking-[0.12em] text-t2">{copy.dealsLabel}</h3>
          {showingSandbox ? (
            <span
              data-sandbox-banner
              className="inline-flex items-center gap-1.5 rounded-full border border-line-strong bg-s1 px-3 py-1 font-mono text-xs text-t1"
            >
              <FlaskConical aria-hidden="true" className="h-3.5 w-3.5 text-maker" />
              Sample network activity (sandbox)
            </span>
          ) : liveRows.length > 0 ? (
            <span className="inline-flex items-center gap-1.5 rounded-full border border-pub/30 bg-pub-soft px-3 py-1 font-mono text-xs text-pub">
              <Radio aria-hidden="true" className="h-3.5 w-3.5" />
              {liveRows.length} live on the wire
            </span>
          ) : null}
        </div>

        {!showingSandbox && deals === 'loading' ? (
          <p className="mt-4 text-t2" role="status">{copy.loading}</p>
        ) : null}
        {!showingSandbox && deals === 'error' ? (
          <p className="mt-4 text-t1" role="alert">{copy.unavailable}</p>
        ) : null}
        {!showingSandbox && Array.isArray(deals) && liveRows.length === 0 ? (
          <p className="mt-4 text-t1" role="status">{copy.quiet}</p>
        ) : null}

        {displayRows.length > 0 ? (
          <ul className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {displayRows.map((deal, i) => (
              <li key={`${deal.kind}-${deal.updated_at}-${i}`} className="min-w-0">
                <DealCard deal={deal} roles={roles} lang={lang} sandbox={showingSandbox} />
              </li>
            ))}
          </ul>
        ) : null}

        {offers.length > 0 ? (
          <>
            <h3 className="mt-12 text-sm font-semibold uppercase tracking-[0.12em] text-t2">{copy.offersLabel}</h3>
            <ul className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {offers.slice(0, 6).map((offer) => (
                <li key={offer.offer_id} className="min-w-0">
                  <OfferCard offer={offer} joinLabel={copy.joinLabel} />
                </li>
              ))}
            </ul>
          </>
        ) : null}

        <p className="mt-8 flex flex-wrap gap-4 text-sm">
          <a
            className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4"
            href={withSource('/makers/', source)}
          >
            {copy.directoryLink}
          </a>
          <a
            className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4"
            href={withSource('/agents', source)}
          >
            {copy.agentsLink}
          </a>
        </p>
      </div>
    </section>
  )
}
