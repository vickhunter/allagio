import { useEffect, useState } from 'react'
import { ArrowLeftRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { useLocale, type Lang } from '@/i18n'
import { useSource, withSource } from '@/lib/source'
import { commissionText, dealsFixtureData, listPublicOffers, listRecentDeals, partyText, wantsDealsFixtures, type Face, type WallDeal, type WallOffer } from '@/lib/network-wall.mjs'

const ACTIVE = new Set(['active', 'completed', 'executed'])

function stateVariant(state: string): 'maker' | 'neutral' {
  return ACTIVE.has(state) ? 'maker' : 'neutral'
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

function Party({ face, roles }: { face: Face; roles: Record<string, string> }) {
  const { text, named } = partyText(face, roles)
  return <span className={`min-w-0 truncate ${named ? 'font-semibold' : 'text-t2'}`}>{text}</span>
}

function DealCard({ deal, roles, lang }: { deal: WallDeal; roles: Record<string, string>; lang: Lang }) {
  return (
    <Card className="flex flex-col gap-3 p-5" data-deal-card>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="pub">{deal.kind}</Badge>
        <Badge variant={stateVariant(deal.state)}>{deal.state}</Badge>
        {deal.parties > 2 ? <Badge variant="neutral">{deal.parties} parties</Badge> : null}
      </div>
      <div className="flex items-center gap-2 text-[15px]">
        <Party face={deal.offerer} roles={roles} />
        <ArrowLeftRight aria-hidden="true" className="h-4 w-4 shrink-0 text-t2" />
        <Party face={deal.acceptor} roles={roles} />
      </div>
      <p className="text-sm text-t2">{ago(deal.updated_at, lang)}</p>
    </Card>
  )
}

function OfferCard({ offer, joinLabel }: { offer: WallOffer; joinLabel: string }) {
  const commission = commissionText(offer)
  return (
    <Card className="flex flex-col gap-2 p-5" data-offer-card>
      <div className="flex items-start justify-between gap-3">
        <h3 className="min-w-0 truncate font-display text-lg font-semibold">{offer.product_name}</h3>
        {commission ? <Badge variant="maker">{commission}</Badge> : null}
      </div>
      <p className="min-w-0 text-sm leading-relaxed text-t2">{offer.product_tagline}</p>
      <a className="mt-auto inline-flex min-h-11 items-center text-sm font-semibold underline underline-offset-4" href={offer.website_url}>{joinLabel}</a>
    </Card>
  )
}

export function NetworkWall() {
  const { t, lang } = useLocale()
  const source = useSource()
  const copy = t.network
  const [deals, setDeals] = useState<WallDeal[] | 'loading' | 'error'>('loading')
  const [offers, setOffers] = useState<WallOffer[]>([])

  useEffect(() => {
    if (wantsDealsFixtures()) {
      const fixture = dealsFixtureData()
      setDeals(fixture.deals as WallDeal[])
      setOffers(fixture.offers as WallOffer[])
      return
    }
    let gone = false
    listRecentDeals()
      .then((rows) => { if (!gone) setDeals(rows as WallDeal[]) })
      .catch(() => { if (!gone) setDeals('error') })
    listPublicOffers()
      .then((rows) => { if (!gone) setOffers(rows as WallOffer[]) })
      .catch(() => {})
    return () => { gone = true }
  }, [])

  const roles = copy.roles as Record<string, string>
  const rows = Array.isArray(deals) ? deals : []
  return (
    <section id="network" aria-labelledby="network-h" className="border-t border-line bg-s2/40">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <p className="kicker">{copy.kicker}</p>
        <h2 id="network-h" className="h-display mt-3 text-3xl sm:text-4xl">{copy.heading}</h2>
        <p className="mt-4 max-w-2xl text-lg leading-relaxed text-t2">{copy.lead}</p>
        {wantsDealsFixtures() ? <p className="mt-4 inline-flex min-h-11 items-center rounded-md border border-line-strong bg-s2 px-3 font-mono text-sm text-t1">TEST data</p> : null}

        <h3 className="mt-10 text-sm font-semibold uppercase tracking-[0.12em] text-t2">{copy.dealsLabel}</h3>
        {deals === 'loading' ? <p className="mt-4 text-t2" role="status">{copy.loading}</p> : null}
        {deals === 'error' ? <p className="mt-4 text-t1" role="alert">{copy.unavailable}</p> : null}
        {Array.isArray(deals) && rows.length === 0 ? <p className="mt-4 text-t1" role="status">{copy.quiet}</p> : null}
        {rows.length > 0 ? (
          <ul className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {rows.map((deal, i) => <li key={i}><DealCard deal={deal} roles={roles} lang={lang} /></li>)}
          </ul>
        ) : null}

        {offers.length > 0 ? (
          <>
            <h3 className="mt-10 text-sm font-semibold uppercase tracking-[0.12em] text-t2">{copy.offersLabel}</h3>
            <ul className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {offers.slice(0, 6).map((offer) => <li key={offer.offer_id}><OfferCard offer={offer} joinLabel={copy.joinLabel} /></li>)}
            </ul>
          </>
        ) : null}

        <p className="mt-8 flex flex-wrap gap-4 text-sm">
          <a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4" href={withSource('/makers/', source)}>{copy.directoryLink}</a>
          <a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4" href={withSource('/agents', source)}>{copy.agentsLink}</a>
        </p>
      </div>
    </section>
  )
}
