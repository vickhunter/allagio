import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { JoinPrompt, joinLead, joinPrompt } from '@/components/join-prompt.mjs'
import { example, faq } from '@/content'
import { Vision } from './Vision'
import { PostPreview } from '@/components/PostPreview'
import { SetupSequence } from './SetupSequence'
import { Orbit } from './Orbit'
import { signInHref, useOfferConfig, useSource, withSource } from '@/lib/source'
import { comingLine, heroH1, heroSupport, manualSetup, noSignIn, setupHref, setupLabel, setupSteps } from '@/home/hero-copy.mjs'
import { howLead, makerBenefit, publisherBenefit } from '@/home/desk-copy.mjs'

const linkClass = 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline'

const capabilities = ['Manual setup', 'MCP', 'Your limits', 'You approve', 'Labelled placement']

function Rows({ items }: { items: string[] }) {
  return (
    <ul className="feature-rows">
      {items.map((item) => <li key={item}>{item}</li>)}
    </ul>
  )
}

// Home. Scalar-like rhythm: centered orbit, then text beside an original visual.
export function App() {
  const source = useSource()
  const { plugin } = useOfferConfig()
  const signIn = signInHref('/owner', source)
  return (
    <Page page="home">
      <div className="scalar-frame">
        <nav className="scalar-rail" aria-label="On this page">
          <p>On this page</p>
          <a href="#connection">Connection</a>
          <a href="#makers">Makers</a>
          <a href="#publishers">Publishers</a>
          <a href="#how">Boundaries</a>
          <a href="#compare">Compare</a>
        </nav>
        <div className="scalar-main">
          <section aria-labelledby="hero-h" className="orbit-stage" data-hero-bg="fallback">
            <div className="hero-fallback" aria-hidden="true" />
            <div className="orbit-scene">
              <Orbit />
              <div className="orbit-copy">
                <h1 id="hero-h">{heroH1}</h1>
                <p className="orbit-support">{heroSupport}</p>
                <div className="orbit-actions">
                  <a className="orbit-signin" href={signIn}>Sign in</a>
                  <div id="join" className="orbit-join">
                    <JoinPrompt line={false} />
                  </div>
                </div>
              </div>
            </div>
            <p className="orbit-next">
              <a className="orbit-setup" data-primary-action href={withSource(setupHref, source)} data-track="secondary_click" data-track-to="agents">{setupLabel}</a>
            </p>
            <details className="setup-details">
              <summary>Setup details</summary>
              <p>{joinLead}</p>
              <p>{joinPrompt}</p>
              <p><a href="/llms.txt">llms.txt</a></p>
              {setupSteps.map((step) => <p key={step}>{step}</p>)}
              <p>{noSignIn}</p>
            </details>
          </section>

          <ul className="cap-rail" aria-label="What this page can do">
            {capabilities.map((item) => <li key={item}>{item}</li>)}
          </ul>

          <section id="connection" aria-labelledby="connection-h" className="split">
            <div>
              <p className="kicker">Connection</p>
              <h2 id="connection-h">You send the bearer yourself.</h2>
              <p>{manualSetup} This page does not issue a token.</p>
              <Rows items={['The join line is public. Copy does not open a page.', noSignIn]} />
            </div>
            <SetupSequence />
          </section>

          <section id="makers" aria-labelledby="makers-h" className="split">
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

          <section id="publishers" aria-labelledby="publishers-h" className="split">
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

          <section id="how" aria-labelledby="how-h" className="flow">
            <p className="kicker">Boundaries</p>
            <h2 id="how-h">The offer stays inside your limits.</h2>
            <p className="flow-lead">{howLead}</p>
            <ol className="flow-nodes" aria-label="Offer boundaries">
              <li><span>Your agent</span><small>Finds a fit</small></li>
              <li><span>The offer</span><small>Not published yet</small></li>
              <li><span>You approve</span><small>Prepare waits</small></li>
            </ol>
            <p className="flow-note">This picture is not a live connection and it does not move money.</p>
          </section>

          <section id="compare" aria-labelledby="compare-h" className="compare">
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

          <section aria-labelledby="money-h" className="border-t border-line">
            <div className="mx-auto max-w-3xl px-5 py-16 text-center md:py-20">
              <h2 id="money-h" className="h-display text-3xl sm:text-4xl">On a {example.price} sale at {example.commission}, the publisher earns {example.publisher}. A {example.fee} fee is paid by the maker, so the maker keeps {example.maker}.</h2>
            </div>
          </section>

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

          <section id="faq" aria-labelledby="faq-h" className="border-t border-line">
            <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:py-20">
              <div>
                <p className="kicker">Money, plainly</p>
                <h2 id="faq-h" className="h-display mt-3 text-3xl">Straight answers.</h2>
              </div>
              <Accordion items={faq} />
            </div>
          </section>

          <section aria-labelledby="join-h" className="border-t border-line">
            <div className="mx-auto grid max-w-3xl gap-4 px-5 py-16 text-center">
              <h2 id="join-h" className="h-display text-3xl">Give your agent this line to start</h2>
              <p className="text-t2">Copy setup prompt does not create a token. The line stays in the first screen.</p>
              <p><a className={linkClass} href="#join">Back to the join line</a></p>
              <div className="mt-4 flex flex-col items-center gap-1">
                <a href={signIn} data-track="join_submit" data-track-role="unset" data-track-cta="Sign in with Google" className={linkClass}>Sign in with Google</a>
                <a href={withSource('/landing-v9/agents/', source)} data-track="secondary_click" data-track-to="agents" className={linkClass}>MCP and setup</a>
                {plugin ? (
                  <a className={linkClass} href="/downloads/offer-rails.zip" data-track="zip_download">Download WordPress plugin</a>
                ) : (
                  <p className="text-base text-t2">The WordPress plugin download is prepared with the hosted package.</p>
                )}
              </div>
            </div>
          </section>
        </div>
      </div>
    </Page>
  )
}
