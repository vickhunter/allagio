import { useEffect, useRef, useState } from 'react'
import { ArrowRight, Bot, Package, PanelTop, ShoppingCart, Coins, UserCheck } from 'lucide-react'
import { example } from '@/content'

// D-048 "How it works": bento grid, one tile per step. Motion = gentle scroll reveal (fade + 8px rise, 700ms, staggered)
// plus a slow step highlight that walks the tiles in order (feature-slideshow idea, rebuilt by hand).
// Reduced motion or "Pause motion": everything visible and static, no highlight walk.
const steps = [
  { n: 1, icon: Package, tone: 'maker', title: 'A maker adds a product', body: `They describe what they sell and set a commission, for example ${example.commission} of each sale.` },
  { n: 2, icon: Bot, tone: 'setup', title: 'An agent checks the fit', body: 'It reads the product and the publisher’s posts, finds a post where the product really fits, and runs the checks.' },
  { n: 3, icon: PanelTop, tone: 'pub', title: 'One labelled box in a matching post', body: 'The publisher approves it. Their site then shows one offer box, clearly marked as sponsored, inside that post.' },
  { n: 4, icon: ShoppingCart, tone: 'pub', title: 'A reader buys', body: 'A reader clicks the box and buys the product on the maker’s own site.' },
  { n: 5, icon: Coins, tone: 'maker', title: 'The sale is credited', body: `The sale is linked to that post. The publisher earns the commission. The maker pays only when a sale happens: on ${example.price}, ${example.publisher} goes to the publisher.` },
] as const

const toneCls = { maker: 'text-maker bg-maker-soft', pub: 'text-pub bg-pub-soft', setup: 'text-setup bg-s2' } as const
const span = ['md:col-span-2', 'md:col-span-2', 'md:col-span-2 md:row-span-2', 'md:col-span-2', 'md:col-span-2']
const assetBase = import.meta.env.BASE_URL

export function HowItWorks() {
  const ref = useRef<HTMLElement>(null)
  const reduce = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const [shown, setShown] = useState(reduce || typeof IntersectionObserver === 'undefined')
  const [active, setActive] = useState(-1)
  useEffect(() => {
    const el = ref.current
    if (!el || reduce || !('IntersectionObserver' in window)) return
    let timer = 0
    const io = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) {
        if (timer) { clearInterval(timer); timer = 0; setActive(-1) }
        return
      }
      setShown(true)
      if (!timer) timer = window.setInterval(() => { if (document.documentElement.dataset.motion !== 'paused') setActive((a) => (a + 1) % steps.length) }, 2600)
    }, { threshold: 0.15 })
    io.observe(el)
    return () => { io.disconnect(); clearInterval(timer) }
  }, [reduce])
  return (
    <section ref={ref} id="how" aria-labelledby="how-h" className="hiw border-t border-line" data-shown={shown ? 'y' : 'n'}>
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <p className="eyebrow">How it works</p>
        <h2 id="how-h" className="h-display mt-3 max-w-3xl text-3xl sm:text-4xl">From a product to a paid sale, in five steps.</h2>
        <p className="mt-3 max-w-2xl text-t2">Makers bring products. Publishers bring posts and readers. Offer Rails connects the two and keeps the record.</p>
        <ol className="mt-10 grid gap-3 md:grid-cols-6 md:grid-rows-[auto_auto_auto]">
          {steps.map((s, i) => {
            const Icon = s.icon
            return (
              <li key={s.n} style={{ ['--i' as string]: i }} data-active={active === i ? 'y' : undefined}
                className={`hiw-tile relative flex flex-col rounded-2xl border border-line bg-s1 p-5 ${span[i]} ${i === 2 ? 'md:col-start-5 md:row-start-1' : ''}`}>
                <div className="flex items-center gap-3">
                  <span className={`inline-flex size-10 items-center justify-center rounded-xl ${toneCls[s.tone]}`}><Icon className="size-5" aria-hidden /></span>
                  <span className="font-mono text-xs text-t2">Step {s.n} of {steps.length}</span>
                </div>
                <h3 className="mt-4 font-display text-xl font-bold tracking-tight">{s.title}</h3>
                <p className="mt-2 text-[15px] leading-relaxed text-t2">{s.body}</p>
                {i === 2 && <OfferBoxSketch />}
              </li>
            )
          })}
          <li style={{ ['--i' as string]: 5 }} className="hiw-tile relative flex flex-col gap-5 overflow-hidden rounded-2xl border border-line-strong bg-s0 p-5 md:col-span-6 md:flex-row md:items-center">
            <div className="md:w-[42%]">
              <span className="inline-flex size-10 items-center justify-center rounded-xl bg-s2 text-setup"><UserCheck className="size-5" aria-hidden /></span>
              <h3 className="mt-4 font-display text-xl font-bold tracking-tight">Why an AI agent does the matching</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">An agent can read the whole product page and every post on a site, so finding the right post for the right product happens on its own, not by hand. People stay in charge: nothing goes live until the publisher approves it, and either side can switch it off.</p>
              <a href={`${assetBase}how-it-works/`} className="mt-4 inline-flex min-h-11 items-center gap-1 font-semibold text-setup underline underline-offset-4 hover:no-underline">See the animated railway figure <ArrowRight className="size-4" aria-hidden /></a>
            </div>
            <figure className="md:flex-1">
              <img src={`${assetBase}how-it-works/railway.webp`} width={1200} height={643} loading="lazy" decoding="async" className="h-auto w-full rounded-xl border border-line"
                alt="Model railway diagram: an offer card leaves the maker's dock, passes a gate with three check pins, and lands in a tray that stands for the blog post; two dials show what each side earns." />
              <figcaption className="mt-2 text-[13px] text-t2">The offer rides the rail: checked at the gate, placed in the post, earnings sent back to both sides.</figcaption>
            </figure>
          </li>
        </ol>
      </div>
    </section>
  )
}

// Tiny static picture of "one labelled box in a post" (decorative; the text above says it).
function OfferBoxSketch() {
  return (
    <div aria-hidden="true" className="mt-5 flex flex-1 flex-col justify-end">
      <div className="rounded-xl border border-line bg-s0 p-4">
        <div className="h-2 w-3/4 rounded bg-s2" /><div className="mt-2 h-2 w-full rounded bg-s2" /><div className="mt-2 h-2 w-5/6 rounded bg-s2" />
        <div className="hiw-box mt-4 rounded-lg border-2 border-pub bg-pub-soft p-3">
          <p className="font-mono text-[11px] uppercase text-pub">Sponsored link</p>
          <p className="mt-1 text-sm font-semibold text-t1">{example.product} · {example.price}</p>
        </div>
        <div className="mt-4 h-2 w-2/3 rounded bg-s2" /><div className="mt-2 h-2 w-4/5 rounded bg-s2" />
      </div>
    </div>
  )
}
