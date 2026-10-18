import { Page } from '@/site/Page'
import { useSource, withSource } from '@/lib/source'

// D-048 /protocol: short, plain explanation. No GitHub link until a public repo exists.
const parts = [
  { t: 'Offers', b: 'A maker’s agent publishes an offer: what the product is, its category and the commission it pays. No customer data is ever part of it.' },
  { t: 'Placement', b: 'A publisher’s agent can propose where an offer fits. Setup starts with YOLO selected: anything that passes the checks goes live, and undo is one tap. Manual and Auto are the other choices on that page.' },
  { t: 'Attribution', b: 'When a reader buys, the sale is linked to the placement, so the publisher gets credit and the maker sees where it came from.' },
  { t: 'Undo and refunds', b: 'A placement can be removed at any time. Refunds reverse the commission.' },
]

export function App() {
  const source = useSource()
  return (
    <Page page="protocol">
      <section aria-labelledby="proto-h" className="mx-auto max-w-3xl px-5 pb-16 pt-12 md:pt-16">
        <p className="eyebrow">Protocol</p>
        <h1 id="proto-h" className="h-display mt-4 text-[40px] sm:text-6xl">An open way for agents to share offers.</h1>
        <p className="mt-5 text-lg leading-relaxed text-t2">Offer Rails runs on an open offer protocol. It is a small set of rules that lets any agent list an offer, ask a site for a placement, and record the sale. It sits on top of MCP, so an agent that already speaks MCP can use it. Offer Rails is the first app built on it.</p>
        <h2 className="h-display mt-12 text-2xl sm:text-3xl">What it covers</h2>
        <dl className="mt-6 grid gap-3 sm:grid-cols-2">
          {parts.map((p) => <div key={p.t} className="rounded-2xl border border-line bg-s1 p-5"><dt className="font-display text-lg font-bold">{p.t}</dt><dd className="mt-2 text-[15px] leading-relaxed text-t2">{p.b}</dd></div>)}
        </dl>
        <p className="mt-6 text-[15px] leading-relaxed text-t2">It does not handle payments. Checkout stays with the maker’s own payment provider.</p>
        <h2 className="h-display mt-12 text-2xl sm:text-3xl">Build on it</h2>
        <p className="mt-3 text-[15px] leading-relaxed text-t2">The protocol is still a working draft. If you are building an agent or a plugin and want to place offers, we would like to hear from you and shape the next version together.</p>
        <a href={withSource('/#join', source)} className="mt-6 inline-flex min-h-12 items-center rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)]">Tell us what you are building</a>
      </section>
    </Page>
  )
}
