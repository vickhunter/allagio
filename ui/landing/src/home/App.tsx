import { Page } from '@/site/Page'
import { example } from '@/content'
import { Vision } from './Vision'
import { ClientStrip } from '@/components/client-strip'
import { BentoCard, BentoGrid } from '@/components/ui/bento-grid'
import { BorderBeam } from '@/components/ui/border-beam'
import { CheckCircle2, Compass, FileSignature, Handshake, KeyRound, Receipt, Zap } from 'lucide-react'
import { PostPreview } from '@/components/PostPreview'
import { Hero } from '@/components/hero-2'
import { FeatureSection } from '@/components/feature-3'
import { FAQ } from '@/components/faq-1'
import { CallToAction } from '@/components/call-to-action-1'
import { useOfferConfig, useSource, withSource } from '@/lib/source'
import { comingLine } from '@/home/hero-copy.mjs'
import { makerBenefit, publisherBenefit, railsAutonomy, railsHeading, railsIntro, railsKicker, railsSteps } from '@/home/desk-copy.mjs'

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
  return (
    <Page page="home">
      <Hero source={source} />
      <ClientStrip />

      <div className="border-t border-line">
        <FeatureSection source={source} />
      </div>

      <section id="rails" aria-labelledby="rails-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <div className="max-w-2xl">
            <p className="kicker">{railsKicker}</p>
            <h2 id="rails-h">{railsHeading}</h2>
            <p>{railsIntro}</p>
            <p className="mt-4 text-[15px] leading-relaxed text-t2">{railsAutonomy}</p>
            <p><a className={linkClass} href={withSource('/protocol/', source)}>The protocol</a></p>
          </div>
          <figure className="mt-10">
            <figcaption className="sr-only">The deal, end to end</figcaption>
            <BentoGrid className="auto-rows-[13rem] grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
              {railsSteps.map((step, i) => {
                const [name, ...rest] = step.split(' — ')
                return (
                  <BentoCard
                    key={name}
                    name={name}
                    description={rest.join(' — ')}
                    Icon={stepIcons[i]}
                    href={withSource('/reference', source)}
                    cta="Reference"
                    className={
                      i === 0
                        ? 'bg-s1 col-span-1 sm:col-span-2 lg:col-span-2'
                        : i === railsSteps.length - 1
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
                )
              })}
            </BentoGrid>
          </figure>
        </div>
      </section>

      <section id="makers" aria-labelledby="makers-h" className="split border-t border-line">
        <div>
          <p className="kicker">Makers</p>
          <h2 id="makers-h">{makerBenefit}</h2>
          <p>Agents put your product in content that fits. The worked sale is an example, not a result from the picture.</p>
          <Rows items={['A commission is a share of a sale.', 'A new site waits for a person before publish.', `${example.product} · ${example.price} · ${example.commission} commission.`]} />
          <p><a className={linkClass} href={withSource('/makers/', source)}>Makers</a></p>
        </div>
        <figure className="piece">
          <figcaption>Worked sale</figcaption>
          <p>{example.product}</p>
          <p>{example.price} · {example.commission} commission</p>
          <p>On a {example.price} sale at {example.commission}, the publisher earns {example.publisher}. A {example.fee} fee is paid by the maker, so the maker keeps {example.maker}.</p>
        </figure>
      </section>

      <section id="publishers" aria-labelledby="publishers-h" className="split border-t border-line">
        <div>
          <p className="kicker">Publishers</p>
          <h2 id="publishers-h">{publisherBenefit}</h2>
          <p>Agents find offers that fit the content you already make. You pick each step. Some posts get no offer.</p>
          <Rows items={['One labelled box, marked sponsored.', comingLine, 'Undo stops clicks at the link right away.']} />
          <p><a className={linkClass} href={withSource('/publishers/', source)}>Publishers</a></p>
        </div>
        <figure className="piece">
          <figcaption>Placement</figcaption>
          <p>After the matching part of the post.</p>
          <p className="piece-note">Illustrative. Not a live placement on your site.</p>
        </figure>
      </section>

      <section id="compare" aria-labelledby="compare-h" className="compare border-t border-line">
        <h2 id="compare-h" className="sr-only">With and without a labelled offer</h2>
        <article>
          <h3>A post, before an offer</h3>
          <p>Nothing is inserted. Some posts get no offer.</p>
        </article>
        <article>
          <h3>A labelled placement you can refuse</h3>
          <p>One box, marked sponsored. Prepare waits until you approve.</p>
        </article>
      </section>

      <div className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-12 md:py-16">
          <Vision />
        </div>
      </div>

      <section id="post-preview" aria-labelledby="preview-h" className="border-t border-line">
        <div className="mx-auto max-w-3xl px-5 py-16 md:py-20">
          <p className="kicker">On your post</p>
          <h2 id="preview-h" className="h-display mt-3 text-3xl">See it on a real post.</h2>
          <p className="mt-4 text-lg leading-relaxed text-t2">Paste a post URL. See where an offer would sit.</p>
          <div className="mt-8">
            <PostPreview />
          </div>
        </div>
      </section>

      <div className="border-t border-line">
        <FAQ />
      </div>

      <div className="border-t border-line">
        <CallToAction source={source} />
      </div>

      {plugin ? (
        <div className="border-t border-line">
          <div className="mx-auto max-w-3xl px-5 py-10 text-center">
            <a className={linkClass} href="/downloads/offer-rails.zip" data-track="zip_download">Download WordPress plugin</a>
          </div>
        </div>
      ) : null}
    </Page>
  )
}
