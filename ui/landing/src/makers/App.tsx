import { useEffect, useState } from 'react'
import { Page } from '@/site/Page'
import { Accordion } from '@/site/Accordion'
import { Runs } from '@/components/Runs'
import { Checks, JoinSection } from '@/site/bits'
import { SignInChoice } from '@/components/SignInChoice'
import { Blip } from '@/components/Blip'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { runsLead, runsTitle } from '@/surfaces.mjs'
import { example, faq, feed, joinLine, makerAdvantages } from '@/content'
import { installs } from '@/agents/facts.mjs'
import { reducedMotion } from '@/lib/utils'
import { track } from '@/lib/track'
import { DirectorySection } from '@/directory/Directory'

// /makers: built from A (bento). Accent: maker terracotta. Every cell is one concrete advantage.
export function App() {
  const [featured, ...rest] = makerAdvantages
  const spans = ['lg:col-span-2', 'lg:col-span-2', 'lg:col-span-2', 'lg:col-span-3', 'lg:col-span-3', 'lg:col-span-2', 'lg:col-span-2']
  return (
    <Page page="makers">
      <section aria-labelledby="hero-h" className="mx-auto grid max-w-6xl gap-8 px-5 pb-10 pt-10 md:grid-cols-[1.3fr_1fr] md:items-end md:pt-16">
        <div className="anim-fade">
          <Badge variant="maker">For makers</Badge>
          <h1 id="hero-h" className="h-display mt-4 text-[42px] sm:text-6xl lg:text-7xl">Help publishers find your product</h1>
          <p className="mt-5 max-w-xl text-lg leading-relaxed text-t2">List what you sell and what you’ll pay on a sale. Publishers’ agents can find your offer through Allagio and prepare a sponsored mention where it fits.</p>
          <div className="mt-7"><SignInChoice where="makers_hero" only="maker" /></div>
        </div>
        <Card className="anim-fade p-5 [animation-delay:150ms]">
          <p className="eyebrow">Example sale</p>
          <p className="mt-2 text-sm text-t2">{example.product} · {example.price} · {example.commission} commission</p>
          <dl className="mt-4 grid grid-cols-3 gap-3 font-mono tabular">
            <div><dt className="text-[11px] uppercase text-t2">You keep</dt><dd className="text-2xl font-medium text-maker">{example.maker}</dd></div>
            <div><dt className="text-[11px] uppercase text-t2">Publisher</dt><dd className="text-2xl font-medium text-pub">{example.publisher}</dd></div>
            <div><dt className="text-[11px] uppercase text-t2">Fee</dt><dd className="text-2xl font-medium">{example.fee}</dd></div>
          </dl>
          <p className="mt-3 text-[12px] text-t2">Fee: 10% of the publisher commission, paid by the maker.</p>
        </Card>
      </section>

      <section id="install" aria-labelledby="install-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <h2 id="install-h" className="h-display mt-3 max-w-3xl text-3xl sm:text-4xl">Connect the client you use</h2>
          <p className="mt-4 max-w-2xl text-lg leading-relaxed text-t2">Use the setup command for your MCP client, then ask your agent to list your product.</p>
          <ul className="mt-8 grid gap-3">
            {installs.map((item) => (
              <li key={item.id} className="min-w-0 rounded-2xl border border-line bg-s1 p-5">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="font-display text-lg font-semibold">{item.label}</h3>
                  <p className="text-sm text-t2">{item.tested ? 'Checked' : 'Untested'}</p>
                </div>
                <p className="mt-2 text-sm leading-relaxed text-t2">{item.proof}</p>
                <pre className="mt-3 max-w-full whitespace-pre-wrap break-all rounded-xl border border-line bg-s0 p-4 font-mono text-sm leading-relaxed text-t1">{item.snippet}</pre>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <DirectorySection kind="maker" />

      <section aria-labelledby="adv-h" className="mx-auto max-w-6xl px-5 pb-16">
        <h2 id="adv-h" className="sr-only">Every maker advantage</h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-6">
          <Card className="flex flex-col p-6 sm:col-span-2 lg:col-span-4 lg:row-span-2">
            <span className="font-mono text-xs text-maker">01</span>
            <h3 className="mt-1 font-display text-2xl font-bold tracking-tight">{featured.t}</h3>
            <p className="mt-2 max-w-xl leading-relaxed text-t2">{featured.b}</p>
            <DigestDemo />
          </Card>
          <Card className="flex flex-col justify-between bg-inv p-6 text-tinv lg:col-span-2">
            <p className="eyebrow !text-tinv2">02 · {rest[1].t}</p>
            <div><p className="text-sm text-tinv2">One command per client is in the install list above. Adding a product needs an MCP client. There is no add-product form on this page.</p><p className="mt-2 text-sm text-tinv2">{rest[1].b}</p><a href="#install" className="mt-3 inline-flex min-h-11 items-center font-semibold text-tinv underline underline-offset-4">Install commands</a></div>
          </Card>
          <Adv n={3} a={rest[0]} className="lg:col-span-2" />
          {rest.slice(2).map((a, i) => <Adv key={a.k} n={i + 4} a={a} className={spans[i]} />)}
          <div className="flex items-end justify-center p-6 sm:col-span-2 lg:col-span-2"><Blip say="Publishers’ agents find you." /></div>
        </div>
      </section>

      <section id="how" aria-labelledby="how-h" className="border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <p className="eyebrow">Start in four steps</p>
          <h2 id="how-h" className="h-display mt-3 text-3xl sm:text-4xl">From your repo to someone else’s page.</h2>
          <ol className="mt-10 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {[['Install once', 'Run the command below, pasted as is. Untested: Claude Code and Generic, both MCP over HTTP. Coming: Cursor, VS Code, Windsurf, ChatGPT and Grok.'], ['Your agent scans locally', 'You see the exact listing card before the first upload.'], ['Set the offer', `Commission and control. Example: ${example.product}, ${example.price}, ${example.commission}.`], ['Get found', 'Publishers’ agents match your offer to posts that fit. The digest shows each step you picked.']].map(([h, b], i) => (
              <li key={h} className="rounded-2xl border border-line bg-s1 p-5"><span className="font-display text-3xl font-bold text-maker">{i + 1}</span><h3 className="mt-2 font-display text-lg font-semibold">{h}</h3><p className="mt-1 text-sm leading-relaxed text-t2">{b}</p></li>
            ))}
          </ol>
          <h3 className="mt-12 font-display text-lg font-semibold">{runsTitle}</h3>
          <p className="mt-2 max-w-3xl text-sm leading-relaxed text-t2">{runsLead}</p>
          <div className="mt-4"><Runs audience="maker" notes /></div>
        </div>
      </section>

      <Checks eyebrow="What protects your brand" />

      <section id="faq" aria-labelledby="faq-h" className="border-t border-line">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:grid-cols-[1fr_2fr] md:py-20">
          <div><p className="eyebrow">Makers ask</p><h2 id="faq-h" className="h-display mt-3 text-3xl sm:text-4xl">Straight answers.</h2></div>
          <Accordion tone="maker" items={[faq[1], faq[0], faq[2], faq[4]]} />
        </div>
      </section>
      <JoinSection role="maker" title="Put your product on the rails." sub={joinLine} />
    </Page>
  )
}

function Adv({ n, a, className }: { n: number; a: { t: string; b: string }; className?: string }) {
  return (
    <Card className={`p-6 transition-colors duration-150 hover:border-maker ${className ?? ''}`}>
      <span className="font-mono text-xs text-maker">{String(n).padStart(2, '0')}</span>
      <h3 className="mt-1 font-display text-lg font-semibold">{a.t}</h3>
      <p className="mt-2 text-sm leading-relaxed text-t2">{a.b}</p>
    </Card>
  )
}

// From A: the digest plays once, per view. side_toggle switches what you are looking at.
function DigestDemo() {
  const [side, setSide] = useState<'maker' | 'pub'>('maker')
  const items = side === 'maker' ? feed : feed.filter((f) => ['matched', 'checks', 'placed', 'clicked', 'earned'].includes(f.kind))
  const [n, setN] = useState(items.length)
  useEffect(() => {
    if (reducedMotion()) { setN(items.length); return }
    setN(1)
    const t = setInterval(() => setN((x) => { if (x + 1 >= items.length) clearInterval(t); return Math.min(x + 1, items.length) }), 1400)
    return () => clearInterval(t)
  }, [side, items.length])
  return (
    <div className="mt-6 flex-1 rounded-xl border border-line bg-s0 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2 px-1 pb-3">
        <p className="eyebrow">Example digest</p>
        <div role="group" aria-label="Whose view" className="inline-flex rounded-lg border border-line bg-s2 p-1 text-[13px]">
          {(['maker', 'pub'] as const).map((s) => <button key={s} type="button" aria-pressed={side === s} onClick={() => { setSide(s); track('side_toggle', { side: s }) }} className={`inline-flex min-h-11 min-w-11 items-center rounded-md px-3 font-semibold transition-colors duration-150 ${side === s ? 'bg-s1 text-maker' : 'text-t2'}`}>{s === 'maker' ? 'Your view' : 'Publisher’s view'}</button>)}
        </div>
      </div>
      <ul className="space-y-1.5">
        {items.slice(0, n).map((f) => (
          <li key={f.kind} className="anim-fade flex items-center justify-between gap-2 rounded-lg bg-s1 px-3 py-2 text-[13px]">
            <span className="flex items-center gap-2"><span aria-hidden className={`size-1.5 shrink-0 rounded-full ${f.who === 'maker' ? 'bg-maker' : f.who === 'pub' ? 'bg-pub' : 'bg-setup'}`} />{f.text}</span>
            <span className="shrink-0 font-mono text-[11px] text-t2">{f.meta}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
