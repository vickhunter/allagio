import { useEffect, useState } from 'react'
import { Undo2 } from 'lucide-react'
import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { Checks, JoinSection, PrimaryCta } from '@/site/bits'
import { Blip } from '@/components/Blip'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { example, faq, publisherAdvantages, surfaces } from '@/content'
import { reducedMotion } from '@/lib/utils'
import { track } from '@/lib/track'

const stations = [
  { k: 'Listed', d: `${example.product} · ${example.price} · ${example.commission}` },
  { k: 'Matched', d: 'Journaling for beginners · relevance 71' },
  { k: 'Checked', d: '9 of 9 hard checks passed' },
  { k: 'Placed', d: 'After the intro · sponsored link' },
  { k: 'Earned', d: `You earn ${example.publisher}` },
] as const

// /publishers: built from B (live-product hero). Accent: publisher olive. No shader here (rule 37: home only).
export function App() {
  const [featured, ...rest] = publisherAdvantages
  return (
    <Page page="publishers">
      <section aria-labelledby="hero-h" className="relative isolate overflow-hidden border-b border-line">
        <div aria-hidden className="absolute inset-0 -z-10 bg-[radial-gradient(60%_70%_at_85%_20%,rgb(var(--rgb-pub-soft))_0%,transparent_70%),radial-gradient(50%_60%_at_10%_90%,rgb(var(--rgb-surface-2))_0%,transparent_70%)]" />
        <div className="mx-auto grid max-w-6xl items-center gap-10 px-5 pb-14 pt-10 md:pt-16 lg:grid-cols-[1fr_1.1fr]">
          <div className="anim-fade">
            <Badge variant="pub">For publishers · {publisherAdvantages.length} advantages</Badge>
            <h1 id="hero-h" className="h-display mt-5 text-[42px] sm:text-6xl">The right offer,<br className="hidden sm:block" /> travelling to your page.</h1>
            <p className="mt-5 max-w-lg text-lg leading-relaxed text-t2">A blog, an X account, a shop: the space you already have, earning per sale. Every step lands in one digest you can undo.</p>
            <div className="mt-7"><PrimaryCta label="Start as a publisher" where="publishers_hero" role="publisher" /></div>
          </div>
          <LivePanel />
        </div>
      </section>

      <section aria-labelledby="adv-h" className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <p className="eyebrow">Every publisher advantage</p>
        <h2 id="adv-h" className="h-display mt-3 max-w-3xl text-3xl sm:text-4xl">{featured.t}. {featured.b}</h2>
        <ol className="mt-10 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {rest.map((a, i) => (
            <li key={a.k} className="rounded-2xl border border-line bg-s1 p-5 transition-colors duration-150 hover:border-pub">
              <span className="font-mono text-xs text-pub">{String(i + 2).padStart(2, '0')}</span>
              <h3 className="mt-1 font-display text-lg font-semibold">{a.t}</h3>
              <p className="mt-2 text-sm leading-relaxed text-t2">{a.b}</p>
            </li>
          ))}
        </ol>
      </section>

      <section id="how" aria-labelledby="how-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <p className="eyebrow">Where it runs</p>
          <h2 id="how-h" className="h-display mt-3 text-3xl sm:text-4xl">Start with WordPress. X and Shopify next.</h2>
          <div className="mt-10 grid gap-3 sm:grid-cols-3">{surfaces.map((s) => <div key={s.name} className="rounded-2xl border border-dashed border-line-strong p-5"><p className="font-mono text-xs text-t2">{s.status}</p><h3 className="mt-1 font-display text-lg font-semibold">{s.name}</h3><p className="mt-1 text-sm text-t2">{s.note}</p></div>)}</div>
        </div>
      </section>

      <Checks eyebrow="What protects your readers" />

      <section id="faq" aria-labelledby="faq-h" className="border-t border-line">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:grid-cols-[1fr_2fr] md:py-20">
          <div><p className="eyebrow">Publishers ask</p><h2 id="faq-h" className="h-display mt-3 text-3xl sm:text-4xl">Before you ask.</h2><Blip className="mt-8 hidden md:inline-flex" say="Undo is one tap. I checked." /></div>
          <Accordion tone="pub" items={[faq[3], faq[1], faq[2], faq[4], faq[0]]} />
        </div>
      </section>
      <JoinSection role="publisher" title="Get on the first train." sub="Pilot: 3 paid products and 3 independent audiences, WordPress first." />
    </Page>
  )
}

function LivePanel() {
  const [step, setStep] = useState(stations.length - 1)
  const [paused, setPaused] = useState(false)
  useEffect(() => {
    if (reducedMotion() || paused) return
    const t = setInterval(() => setStep((s) => (s + 1) % stations.length), 1800)
    return () => clearInterval(t)
  }, [paused])
  const pct = (step / (stations.length - 1)) * 100
  return (
    <Card className="anim-fade relative bg-s1/95 [animation-delay:150ms]">
      <div className="flex items-center justify-between border-b border-line px-5 py-3">
        <p className="text-sm font-semibold">Digest · journalwithme.blog</p>
        <button type="button" onClick={() => { setPaused((p) => !p); track('demo_pause') }} className="inline-flex min-h-11 min-w-11 items-center rounded-md px-2 font-mono text-[11px] uppercase text-t2 transition-colors duration-150 hover:bg-s2" aria-pressed={paused}>{paused ? 'Play' : 'Pause'} demo</button>
      </div>
      <div className="px-5 pb-2 pt-6">
        <div className="relative mx-6 h-10" aria-hidden="true">
          <div className="absolute inset-x-0 top-[14px] h-px bg-line-strong" /><div className="absolute inset-x-0 top-[22px] h-px bg-line-strong" />
          <div className="absolute left-0 top-[18px] h-0.5 w-full origin-left bg-pub transition-transform duration-700 ease-out" style={{ transform: `scaleX(${pct / 100})` }} />
          {stations.map((s, n) => <span key={s.k} className={`absolute top-[12px] size-3 -translate-x-1/2 rounded-full border-2 transition-colors duration-150 ${n <= step ? 'border-pub bg-pub' : 'border-line-strong bg-s1'}`} style={{ left: `${(n / (stations.length - 1)) * 100}%` }} />)}
          <span className="absolute top-0 grid h-9 w-12 -translate-x-1/2 place-items-center rounded-md bg-maker-brand font-mono text-[11px] font-medium text-white transition-[left] duration-700 ease-out" style={{ left: `${pct}%` }}>10%</span>
        </div>
        <ol className="-mx-2 mt-3 grid grid-cols-5 text-center font-mono text-[10px] uppercase text-t2 sm:text-[11px]">{stations.map((s, n) => <li key={s.k} className={n === step ? 'font-medium text-t1' : ''}>{s.k}</li>)}</ol>
      </div>
      <ul className="space-y-2 p-5 pt-3">
        {stations.map((s, n) => n <= step ? (
          <li key={s.k} className={`flex items-center justify-between gap-3 rounded-xl border border-line px-4 py-3 text-sm ${n === step ? 'anim-fade bg-s0' : 'bg-s1'}`}>
            <span><strong className="font-semibold">{s.k}</strong> <span className="text-t2">· {s.d}</span></span>
            {s.k === 'Placed' && <span className="hidden shrink-0 items-center gap-1 text-[12px] text-t2 sm:flex"><Undo2 className="size-3.5" aria-hidden />Undo</span>}
          </li>
        ) : (
          <li key={s.k} aria-hidden="true" className="flex items-center gap-3 rounded-xl border border-dashed border-line px-4 py-3 text-sm"><span className="h-3 w-16 rounded bg-s2" /><span className="h-3 flex-1 rounded bg-s2/70" /></li>
        ))}
      </ul>
      <p className="border-t border-line px-5 py-3 text-[12px] text-t2">Example data. Approved by policy: “Approve anything that passes the checks”.</p>
    </Card>
  )
}
