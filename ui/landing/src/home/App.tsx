import { ArrowRight } from 'lucide-react'
import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { Runs } from '@/components/Runs'
import { JoinForm } from '@/components/JoinForm'
import { joinHeading, joinSignInNote } from '@/components/join-prompt.mjs'
import { runsLead, runsTitle } from '@/surfaces.mjs'
import { Checks } from '@/site/bits'
import { SignInChoice } from '@/components/SignInChoice'
import { Blip } from '@/components/Blip'
import { driftChips, example, faq, joinLine, makerAdvantages, publisherAdvantages } from '@/content'
import { HeroShader } from './HeroShader'
import { PostPreview } from '@/components/PostPreview'
import { HowItWorks } from './HowItWorks'
import { useSource, withSource } from '@/lib/source'

// Home = C Editorial (Victor, 16 Oct). One Shaders effect behind the hero, real offers drifting at the edge.
export function App() {
  const source = useSource()
  return (
    <Page page="home">
      <section aria-labelledby="hero-h" className="relative isolate overflow-hidden">
        <HeroShader />
        <div className="mx-auto grid max-w-6xl gap-10 px-5 pb-14 pt-10 md:pt-16 lg:grid-cols-[1fr_300px] lg:gap-6">
          <div className="anim-fade">
            <p className="eyebrow">Issue 01 · Affiliate marketing, agent-native</p>
            <h1 id="hero-h" className="h-display mt-5 text-[42px] sm:text-7xl lg:text-[84px]">
              Affiliate marketing, evolved for{' '}
              <span className="relative inline-block whitespace-nowrap">agents.
                <svg className="scribble absolute -bottom-2 left-0 h-[0.22em] w-full overflow-visible sm:-bottom-3" viewBox="0 0 200 20" preserveAspectRatio="none" aria-hidden="true">
                  <path pathLength={1} d="M3 13 C 40 6, 80 5, 118 9 S 176 14, 197 6" fill="none" stroke="rgb(var(--rgb-setup))" strokeWidth="4.5" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
                </svg>
              </span>
            </h1>
            <div className="mt-6 max-w-xl">
              <h2 id="join-h" className="font-display text-2xl font-bold leading-tight tracking-tight sm:text-3xl">{joinHeading}</h2>
              <p className="mt-2 text-sm leading-relaxed text-t2">{joinLine}</p>
              <div className="mt-4">
                <JoinForm note={joinSignInNote} />
              </div>
            </div>
            <div className="mt-8 grid gap-6 border-t border-t1 pt-6 md:grid-cols-[1.1fr_1fr] md:items-start">
              <div>
                <p className="text-lg leading-relaxed text-t2">Makers pay only for sales. Publishers earn from more of their posts. Agents find, match and place.</p>
                <p className="mt-3 font-semibold text-t1">No paying for clicks or space. Only for results.</p>
                <ol className="mt-4 list-decimal space-y-2 pl-5 text-[15px] leading-relaxed text-t1">
                  <li>Paying for space. The bill is for clicks or a slot, whether or not anyone buys.</li>
                  <li>Affiliate marketing pays per sale. Finding partners, matching an offer to content, and placing it are still slow.</li>
                  <li>Agents do the finding, the matching and the placing.</li>
                </ol>
              </div>
              <div>
                <p className="mb-3 text-[13px] font-semibold text-t2">Sign in. Which one are you?</p>
                <SignInChoice where="hero" />
                <p className="mt-2 text-[13px] text-t2">One account can do both.</p>
              </div>
            </div>
          </div>
          <Drift />
        </div>
        <div className="mx-auto max-w-6xl px-5 pb-16">
          <p className="eyebrow mb-3">Pick your side. Each page lists every advantage.</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <SideLink href={withSource('/makers/', source)} tone="maker" n="A" title="I sell something." text="Publishers’ agents find you. You pay only for sales." cta={`All ${makerAdvantages.length} maker advantages`} />
            <SideLink id="publishers" href={withSource('/publishers/', source)} tone="pub" n="B" title="I publish." text="Earn from more of your posts. Agents find offers that fit the content you already make." cta={`All ${publisherAdvantages.length} publisher advantages`} />
          </div>
          <p className="mt-3 text-[13px] text-t2">Both? One account does both. Press <kbd className="font-mono">M</kbd> or <kbd className="font-mono">P</kbd>.</p>
          <div className="mt-8">
            <h2 className="font-display text-2xl font-bold tracking-tight">{runsTitle}</h2>
            <p className="mt-2 max-w-3xl text-sm leading-relaxed text-t2">{runsLead}</p>
            <div className="mt-4"><Runs audience="publisher" /></div>
          </div>
        </div>
      </section>

      <HowItWorks />

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

      <section aria-labelledby="sides-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <p className="eyebrow">Chapter A · Two sides, one rail</p>
          <h2 id="sides-h" className="h-display mt-3 max-w-3xl text-3xl sm:text-4xl">On a {example.price} sale at {example.commission}, the publisher earns {example.publisher}. The maker keeps {example.maker}.</h2>
          <div className="mt-10 grid gap-10 md:grid-cols-2">
            <Teaser tone="maker" title="For makers" items={makerAdvantages.slice(0, 4)} href={withSource('/makers/', source)} />
            <Teaser tone="pub" title="For publishers" items={publisherAdvantages.slice(0, 4)} href={withSource('/publishers/', source)} />
          </div>
        </div>
      </section>

      <Checks eyebrow="Chapter B · The rules" />

      <section id="faq" aria-labelledby="faq-h" className="border-t border-line">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:grid-cols-[1fr_2fr] md:py-20">
          <div><p className="eyebrow">Chapter C · Money, plainly</p><h2 id="faq-h" className="h-display mt-3 text-3xl sm:text-4xl">Straight answers.</h2><Blip className="mt-8 hidden md:inline-flex" say="Money questions get plain answers." /></div>
          <Accordion items={faq} />
        </div>
      </section>

    </Page>
  )
}

function Drift() {
  const tone = { maker: 'text-maker', pub: 'text-pub', agent: 'text-setup' } as const
  const depth = ['lg:translate-x-6 lg:scale-[.94]', 'lg:-translate-x-4', 'lg:translate-x-10 lg:scale-[.97]', 'lg:-translate-x-2 lg:scale-[1.03]', 'lg:translate-x-4 lg:scale-[.95]', 'lg:-translate-x-6']
  return (
    <div className="relative z-10 lg:pt-6">
      <p id="drift-l" className="sr-only">Example activity for one offer</p>
      <ul aria-labelledby="drift-l" className="flex flex-wrap gap-2 lg:flex-col lg:gap-4">
        {driftChips.map((c, i) => (
          <li key={c.label} className={depth[i]}>
            <div className="drift rounded-xl border border-line bg-s1/95 px-3 py-2 text-[13px] shadow-[0_6px_18px_-10px_rgb(var(--rgb-text-1)/.35)]" style={{ animationDelay: `${-i * 1.7}s` }}>
              <span className={`font-mono text-[11px] uppercase ${tone[c.who]}`}>{c.label}</span> <span>{c.text}</span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

function SideLink({ href, tone, n, title, text, cta, id }: { href: string; tone: 'maker' | 'pub'; n: string; title: string; text: string; cta: string; id?: string }) {
  const c = tone === 'maker' ? 'bg-maker-soft hover:border-maker text-maker' : 'bg-pub-soft hover:border-pub text-pub'
  return (
    <a id={id} href={href} data-track="path_pick" data-track-path={tone === 'maker' ? 'maker' : 'publisher'} className={`group flex flex-col rounded-2xl border border-line p-5 transition-colors duration-150 ${c}`}>
      <span className="font-mono text-xs">Path {n}</span>
      <span className="mt-4 font-display text-2xl font-bold tracking-tight text-t1">{title}</span>
      <span className="mt-2 text-[15px] leading-relaxed text-t2">{text}</span>
      <span className="mt-4 flex items-center gap-1 text-sm font-semibold">{cta} <ArrowRight className="size-4 transition-transform duration-200 group-hover:translate-x-1" aria-hidden /></span>
    </a>
  )
}

function Teaser({ tone, title, items, href }: { tone: 'maker' | 'pub'; title: string; items: readonly { k: string; t: string; b: string }[]; href: string }) {
  const c = tone === 'maker' ? 'text-maker' : 'text-pub'
  return (
    <div>
      <h3 className={`font-display text-xl font-bold ${c}`}>{title}</h3>
      <ol className="mt-4">
        {items.map((a, i) => <li key={a.k} className="grid grid-cols-[2.25rem_1fr] gap-3 border-b border-line py-4"><span className={`font-display text-2xl font-bold ${c}`}>{i + 1}</span><div><p className="font-semibold">{a.t}</p><p className="mt-1 text-sm leading-relaxed text-t2">{a.b}</p></div></li>)}
      </ol>
      <a href={href} data-track="secondary_click" data-track-to={tone} className={`mt-4 inline-flex min-h-11 min-w-11 items-center gap-1 font-semibold underline underline-offset-4 hover:no-underline ${c}`}>Read every {tone === 'maker' ? 'maker' : 'publisher'} advantage <ArrowRight className="size-4" aria-hidden /></a>
    </div>
  )
}
