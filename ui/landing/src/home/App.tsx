import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { JoinForm } from '@/components/JoinForm'
import { example, faq } from '@/content'
import { Vision } from './Vision'
import { PostPreview } from '@/components/PostPreview'
import { useSource, withSource } from '@/lib/source'
import { inWriteOffer } from '@/vision.mjs'

// Home. The first screen is the in-write offer, the agent line, and a link to limits.
export function App() {
  const source = useSource()
  return (
    <Page page="home">
      <section aria-labelledby="hero-h" className="bg-s0" data-hero-bg="fallback">
        <div className="mx-auto max-w-3xl px-5 pb-8 pt-8">
          <h1 id="hero-h" className="h-display max-w-3xl text-[1.65rem] leading-[1.15] sm:text-4xl">{inWriteOffer}</h1>
          <div className="mt-6 max-w-xl">
            <JoinForm />
          </div>
          <p className="mt-2">
            <a className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline" href={withSource('/you/', source)}>Your limits</a>
          </p>
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
    </Page>
  )
}
