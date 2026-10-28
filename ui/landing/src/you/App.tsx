import { Page } from '@/site/Page'
import { signInHref, useSource, withSource } from '@/lib/source'
import { limits, youPageLead } from '@/vision.mjs'

export function App() {
  const source = useSource()
  return (
    <Page page="you">
      <article className="mx-auto max-w-3xl px-5 pb-16 pt-12 md:pt-16">
        <p className="eyebrow">For the human</p>
        <h1 id="you-h" className="h-display mt-4 text-[40px] sm:text-6xl">Your limits, your approvals, a receipt in plain words.</h1>
        <p className="mt-5 text-lg leading-relaxed text-t2">{youPageLead}</p>
        <p className="mt-6">
          <a href={signInHref('/you/', source)} className="inline-flex min-h-11 min-w-11 items-center rounded-[10px] bg-t1 px-5 text-sm font-semibold text-s0">Sign in to set your limits</a>
        </p>

        <section id="limits" aria-labelledby="limits-h" className="mt-12">
          <h2 id="limits-h" className="font-display text-2xl font-bold tracking-tight">Limits</h2>
          <p className="mt-2 text-sm leading-relaxed text-t2">Your limits come first. Pick how much the agent may do. A new site starts on the first one.</p>
          <ul className="mt-4 grid gap-3">
            {limits.map((item) => (
              <li key={item.name} className="rounded-2xl border border-line bg-s1 p-4">
                <h3 className="font-semibold">{item.name}</h3>
                <p className="mt-2 text-sm leading-relaxed text-t2">{item.note}</p>
              </li>
            ))}
          </ul>
        </section>

        <section id="approvals" aria-labelledby="approvals-h" className="mt-12">
          <h2 id="approvals-h" className="font-display text-2xl font-bold tracking-tight">Approvals</h2>
          <p className="mt-2 text-sm leading-relaxed text-t2">A person approves each placement before it is published. No agent tool can approve. Your agent can prepare the placement and show you the terms.</p>
        </section>

        <section id="receipts" aria-labelledby="receipts-h" className="mt-12">
          <h2 id="receipts-h" className="font-display text-2xl font-bold tracking-tight">Receipts</h2>
          <p className="mt-2 text-sm leading-relaxed text-t1">No deals yet.</p>
        </section>

        <p className="mt-10 text-sm text-t2">
          <a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4 hover:no-underline" href={withSource('/#join', source)}>Give your agent the join line</a>
        </p>
      </article>
    </Page>
  )
}
