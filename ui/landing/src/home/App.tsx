import { Page } from '@/site/Page'
import { Vision } from './Vision'
import { ClientStrip } from '@/components/client-strip'
import { BentoCard, BentoGrid } from '@/components/ui/bento-grid'
import { BorderBeam } from '@/components/ui/border-beam'
import { RetroGrid } from '@/components/ui/retro-grid'
import { CheckCircle2, Compass, FileSignature, Handshake, KeyRound, LayoutTemplate, Receipt, Calculator, XCircle, Zap } from 'lucide-react'
import { PostPreview } from '@/components/PostPreview'
import { ConnectionCard, Hero } from '@/components/hero-2'
import { Why } from './Why'
import { FeatureSection } from '@/components/feature-3'
import { FAQ } from '@/components/faq-1'
import { TradeOffs } from '@/components/trade-offs'
import { CallToAction } from '@/components/call-to-action-1'
import { useOfferConfig, useSource, withSource } from '@/lib/source'
import { NumberTicker } from '@/components/ui/number-ticker'
import { example } from '@/content'
import { useLocale } from '@/i18n'

const linkClass = 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline'

const stepIcons = [KeyRound, Handshake, Compass, FileSignature, CheckCircle2, Zap, Receipt]

const cardWashes = [
  'bg-[radial-gradient(120%_120%_at_0%_0%,rgba(26,110,66,0.10),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_100%_0%,rgba(160,110,20,0.10),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_0%_100%,rgba(60,80,140,0.09),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_100%_100%,rgba(26,110,66,0.08),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_0%_0%,rgba(160,110,20,0.09),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_100%_0%,rgba(60,80,140,0.10),transparent_60%)]',
  'bg-[radial-gradient(120%_120%_at_100%_100%,rgba(26,110,66,0.10),transparent_60%)]',
]

function Rows({ items }: { items: string[] }) {
  return (
    <ul className="feature-rows">
      {items.map((item) => <li key={item}>{item}</li>)}
    </ul>
  )
}

export function App() {
  const source = useSource()
  const { plugin } = useOfferConfig()
  const { t } = useLocale()
  return (
    <Page page="home">
      <Hero source={source} />
      <Why />
      <ClientStrip />

      <div className="border-t border-line bg-s2/40">
        <FeatureSection source={source} />
        <div className="container mx-auto px-4 pb-16 md:px-8 md:pb-20">
          <ConnectionCard />
        </div>
      </div>

      <section id="rails" aria-labelledby="rails-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <div className="max-w-2xl">
            <p className="kicker">{t.rails.kicker}</p>
            <h2 id="rails-h">{t.rails.heading}</h2>
            <p>{t.rails.intro}</p>
            <p className="mt-4 text-[15px] leading-relaxed text-t2">{t.rails.autonomy}</p>
            <p><a className={linkClass} href={withSource('/protocol/', source)}>{t.rails.protocolLink}</a></p>
          </div>
          <figure className="mt-10">
            <figcaption className="sr-only">{t.rails.figcaption}</figcaption>
            <BentoGrid className="auto-rows-[13rem] grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
              {t.rails.steps.map((step, i) => (
                <BentoCard
                  key={step.name}
                  name={step.name}
                  description={step.body}
                  Icon={stepIcons[i]}
                  href={withSource('/reference', source)}
                  cta={t.rails.cta}
                  className={
                    i === 0
                      ? 'bg-s1 col-span-1 sm:col-span-2 lg:col-span-2'
                      : i === t.rails.steps.length - 1
                        ? 'bg-s1 col-span-1 sm:col-span-2 lg:col-span-2'
                        : 'bg-s1 col-span-1'
                  }
                  background={
                    <>
                      <div className={`absolute inset-0 ${cardWashes[i]}`} />
                      {i === 0 ? <BorderBeam size={140} duration={7} borderWidth={1.5} /> : null}
                    </>
                  }
                />
              ))}
            </BentoGrid>
          </figure>
        </div>
      </section>

      <section id="makers" aria-labelledby="makers-h" className="split border-t border-line bg-s2/50">
        <div>
          <p className="kicker">{t.makers.kicker}</p>
          <h2 id="makers-h">{t.makers.heading}</h2>
          <p>{t.makers.lead}</p>
          <Rows items={t.makers.rows} />
          <p><a className={linkClass} href={withSource('/makers/', source)}>{t.makers.link}</a></p>
        </div>
        <figure className="piece group relative overflow-hidden transition-[transform,box-shadow] duration-300 hover:-translate-y-0.5 hover:shadow-lg">
          <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-maker/60 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
          <figcaption className="flex items-center gap-2"><Calculator aria-hidden="true" className="h-4 w-4 text-maker" />{t.makers.workedCaption}</figcaption>
          <p>{t.makers.exampleProduct}</p>
          <p>{t.makers.workedTerms}</p>
          <p className="mt-2 flex items-baseline gap-2">
            <NumberTicker
              value={example.publisherCents / 100}
              decimalPlaces={2}
              prefix="€"
              className="text-3xl font-semibold tracking-tight text-maker"
            />
            <span className="text-sm text-t2">{t.makers.workedTicker}</span>
          </p>
          <p>{t.makers.workedBody}</p>
        </figure>
      </section>

      <section id="publishers" aria-labelledby="publishers-h" className="split border-t border-line">
        <div>
          <p className="kicker">{t.publishers.kicker}</p>
          <h2 id="publishers-h">{t.publishers.heading}</h2>
          <p>{t.publishers.lead}</p>
          <Rows items={t.publishers.rows} />
          <p><a className={linkClass} href={withSource('/publishers/', source)}>{t.publishers.link}</a></p>
        </div>
        <figure className="piece group relative overflow-hidden transition-[transform,box-shadow] duration-300 hover:-translate-y-0.5 hover:shadow-lg">
          <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-pub/60 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
          <figcaption className="flex items-center gap-2"><LayoutTemplate aria-hidden="true" className="h-4 w-4 text-pub" />{t.publishers.placementCaption}</figcaption>
          <p>{t.publishers.placementBody}</p>
          <p className="piece-note">{t.publishers.placementNote}</p>
        </figure>
      </section>

      <section id="compare" aria-labelledby="compare-h" className="compare border-t border-line bg-s1">
        <h2 id="compare-h" className="sr-only">{t.compare.heading}</h2>
        <article className="relative border border-line">
          <h3 className="flex items-center gap-2.5"><XCircle aria-hidden="true" className="h-6 w-6 text-t2" />{t.compare.beforeTitle}</h3>
          <p>{t.compare.beforeBody}</p>
        </article>
        <article className="relative overflow-hidden border border-pub/30 bg-s1 shadow-md">
          <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-pub/70 to-transparent" />
          <h3 className="flex items-center gap-2.5"><CheckCircle2 aria-hidden="true" className="h-6 w-6 text-pub" />{t.compare.afterTitle}</h3>
          <p>{t.compare.afterBody}</p>
        </article>
      </section>

      <div className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-12 md:py-16">
          <Vision />
        </div>
      </div>

      <section id="post-preview" aria-labelledby="preview-h" className="relative overflow-hidden border-t border-line bg-s2/50">
        <RetroGrid aria-hidden="true" className="opacity-100" opacity={0.3} lightLineColor="rgb(63 92 53)" />
        <div className="relative mx-auto max-w-3xl px-5 py-16 md:py-20">
          <p className="kicker">{t.previewSection.kicker}</p>
          <h2 id="preview-h" className="h-display mt-3 text-3xl">{t.previewSection.heading}</h2>
          <p className="mt-4 text-lg leading-relaxed text-t2">{t.previewSection.lead}</p>
          <div className="mt-8">
            <PostPreview />
          </div>
        </div>
      </section>

      <div className="border-t border-line">
        <TradeOffs />
      </div>

      <div className="border-t border-line bg-s1">
        <FAQ />
      </div>

      <div className="border-t border-line">
        <CallToAction source={source} />
      </div>

      {plugin ? (
        <div className="border-t border-line">
          <div className="mx-auto max-w-3xl px-5 py-10 text-center">
            <a className={linkClass} href="/downloads/offer-rails.zip" data-track="zip_download">{t.plugin.download}</a>
          </div>
        </div>
      ) : null}
    </Page>
  )
}
