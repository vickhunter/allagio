import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { JoinForm } from '@/components/JoinForm'
import { example, faq, pitch } from '@/content'
import { Vision } from './Vision'
import { PostPreview } from '@/components/PostPreview'
import { DistributionDesk } from './DistributionDesk'
import { signInHref, useOfferConfig, useSource, withSource } from '@/lib/source'
import { inWriteOffer } from '@/vision.mjs'
import { howLead, makerBenefit, publisherBenefit } from '@/home/desk-copy.mjs'

const linkClass = 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline'

// Home. The first screen is the in-write offer, the join line, your limits, and the labelled path.
export function App() {
  const source = useSource()
  const { plugin } = useOfferConfig()
  const signIn = signInHref('/owner', source)
  return (
    <Page page="home">
      <section aria-labelledby="hero-h" className="home-hero" data-hero-bg="fallback">
        <div className="hero-fallback" aria-hidden="true" />
        <div className="hero-grid">
          <div className="hero-title">
            <h1 id="hero-h" className="h-display max-w-[18em] text-[1.35rem] leading-[1.15] text-tinv sm:text-[2rem] sm:leading-[1.12]">{inWriteOffer}</h1>
          </div>
          <DistributionDesk />
          <div className="hero-copy">
            <div className="hero-panel">
              <JoinForm />
            </div>
            <p className="mt-2">
              <a className={`${linkClass} text-tinv`} href={withSource('/you/', source)}>Your limits</a>
            </p>
          </div>
          <p className="hero-pitch">{pitch}</p>
        </div>
      </section>

      <section aria-labelledby="makers-h" className="border-t border-line bg-[rgb(var(--rgb-footer))]">
        <div className="mx-auto grid max-w-6xl items-start gap-8 px-5 py-14 md:grid-cols-2 md:py-16">
          <div>
            <p className="eyebrow">For makers</p>
            <h2 id="makers-h" className="h-display mt-3 text-3xl sm:text-4xl">{makerBenefit}</h2>
            <p className="mt-4 text-lg leading-relaxed text-t2">Agents put your product in content that fits. The worked sale below is an example, not a result from the picture above.</p>
            <p className="mt-4">
              <a className={linkClass} href={withSource('/makers/', source)}>Makers</a>
            </p>
          </div>
          <div className="rounded-2xl border border-line bg-s1 p-5">
            <p className="eyebrow">Worked sale</p>
            <p className="mt-3 text-base leading-relaxed">{example.product} · {example.price} · {example.commission} commission</p>
            <p className="mt-3 text-base leading-relaxed text-t2">On a {example.price} sale at {example.commission}, the publisher earns {example.publisher}. A {example.fee} fee is paid by the maker, so the maker keeps {example.maker}.</p>
          </div>
        </div>
      </section>

      <section id="publishers" aria-labelledby="publishers-h" className="border-t border-line">
        <div className="mx-auto grid max-w-6xl items-start gap-8 px-5 py-14 md:grid-cols-2 md:py-16">
          <div>
            <p className="eyebrow">For publishers</p>
            <h2 id="publishers-h" className="h-display mt-3 text-3xl sm:text-4xl">{publisherBenefit}</h2>
            <p className="mt-4 text-lg leading-relaxed text-t2">Agents find offers that fit the content you already make. You pick each step. Some posts get no offer.</p>
            <p className="mt-4">
              <a className={linkClass} href={withSource('/publishers/', source)}>Publishers</a>
            </p>
          </div>
          <div className="rounded-2xl border border-line bg-s1 p-5">
            <p className="eyebrow">Placement</p>
            <p className="mt-3 text-base leading-relaxed">One labelled box, rel="sponsored", after the matching part of the post. Undo stops clicks at the link right away.</p>
            <p className="mt-3 text-base leading-relaxed text-t2">The picture in the first screen is an illustrative example. It is not a live placement on your site.</p>
          </div>
        </div>
      </section>

      <section id="how" aria-labelledby="how-h" className="border-t border-line bg-[rgb(var(--rgb-footer))]">
        <div className="mx-auto max-w-6xl px-5 py-14 md:py-16">
          <p className="eyebrow">How it works</p>
          <h2 id="how-h" className="h-display mt-3 max-w-3xl text-3xl sm:text-4xl">How it works</h2>
          <p className="mt-4 max-w-3xl text-lg leading-relaxed text-t2">{howLead}</p>
          <ol className="how-rail">
            <li><h3>Find an offer</h3><p>Your agent can find an offer and prepare a placement. A pasted link counts clicks.</p></li>
            <li><h3>Match it to a post</h3><p>An offer needs a relevance score of 25 or higher and must be live. This page does not show a score for the example.</p></li>
            <li><h3>Prepare the placement</h3><p>Prepare does not publish. A new site stores the publish step as ask, so prepare waits for a person.</p></li>
          </ol>
        </div>
      </section>

      <div className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-12 md:py-16">
          <Vision />
        </div>
      </div>

      <section aria-labelledby="money-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <h2 id="money-h" className="h-display max-w-3xl text-3xl sm:text-4xl">On a {example.price} sale at {example.commission}, the publisher earns {example.publisher}. A {example.fee} fee is paid by the maker, so the maker keeps {example.maker}.</h2>
        </div>
      </section>

      <section id="post-preview" aria-labelledby="preview-h" className="border-t border-line">
        <div className="mx-auto max-w-3xl px-5 py-16 md:py-20">
          <p className="eyebrow">On your post</p>
          <h2 id="preview-h" className="h-display mt-3 text-3xl sm:text-4xl">See it on a real post.</h2>
          <p className="mt-4 text-lg leading-relaxed text-t2">Paste a post URL. See where an offer would sit.</p>
          <div className="mt-8">
            <PostPreview />
          </div>
        </div>
      </section>

      <section id="faq" aria-labelledby="faq-h" className="border-t border-line">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:py-20">
          <div>
            <p className="eyebrow">Money, plainly</p>
            <h2 id="faq-h" className="h-display mt-3 text-3xl sm:text-4xl">Straight answers.</h2>
          </div>
          <Accordion items={faq} />
        </div>
      </section>

      <section aria-labelledby="join-h" className="border-t border-line bg-s1">
        <div className="mx-auto grid max-w-6xl gap-6 px-5 py-14 md:py-16">
          <div>
            <h2 id="join-h" className="h-display text-3xl sm:text-4xl">Give your agent this line to start</h2>
            <p className="mt-3 text-lg leading-relaxed text-t2">The join line and the Copy button stay in the first screen.</p>
          </div>
          <div className="flex flex-col items-start gap-1">
            <a href={signIn} data-track="join_submit" data-track-role="unset" data-track-cta="Sign in with Google" className={linkClass}>Sign in with Google</a>
            <a href={withSource('/landing-v9/agents/', source)} data-track="secondary_click" data-track-to="agents" className={linkClass}>MCP and setup</a>
            {plugin ? (
              <a className={linkClass} href="/downloads/offer-rails.zip" data-track="zip_download">Download WordPress plugin</a>
            ) : (
              <p className="text-base text-t2">The WordPress plugin download is prepared with the hosted package.</p>
            )}
            <a className={linkClass} href="#join">Back to the join line</a>
          </div>
        </div>
      </section>
    </Page>
  )
}
