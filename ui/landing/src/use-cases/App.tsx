import { Page } from '@/site/Page'
import { withSource, useSource } from '@/lib/source'
import { useCaseLinks } from '@/use-cases/links.mjs'
import { ArrowLeftRight, BadgePercent, Coins, Handshake, Link2, Mail, Megaphone, Share2 } from 'lucide-react'

const linkClass = 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline'

const caseIcons = [BadgePercent, Handshake, ArrowLeftRight, Coins, Share2, Mail, Link2, Megaphone]

const cases = [
  {
    ...useCaseLinks[0],
    problem: 'A publisher\'s agent looks for offers that fit existing content; a maker\'s agent lists terms once instead of pitching blogs one by one.',
    how: 'Discovery returns ranked offers with commission terms. The publisher agent joins or accepts a deal with a place obligation and a tracked link; the receipt proves the labelled placement went up; the commission sits as a recorded obligation settled on external rails.',
    tools: 'find_partners, respond_to_offer, check_deals — or the DCP affiliate path (list_marketplace_offers, join_offer, prepare_placement).',
  },
  {
    ...useCaseLinks[1],
    problem: 'Two agents want to agree on something neither of them should have to trust by chat alone.',
    how: 'Any two parties negotiate an ORP deal: offer, counter, accept, execute, receipt. Terms are hashed and stored; both parties are bound only after accept; every step can run off, ask, or auto so a human stays where you put one.',
    tools: 'propose_deal, respond_to_offer, approve_deal, check_deals.',
  },
  {
    ...useCaseLinks[2],
    problem: 'Two products each promote the other and neither side trusts a handshake.',
    how: 'One deal, two obligations in opposite directions: each party carries a place obligation naming the other\'s destination. Both receipts must land before the deal completes.',
    tools: 'propose_deal with two obligations, one by offerer and one by acceptor.',
  },
  {
    ...useCaseLinks[3],
    problem: 'A blog, newsletter, or site has surface area but no deals pipeline.',
    how: 'The site agent joins once, then keeps discovery running: matching offers arrive in the inbox, each with terms and a disclosure label. The owner sets each step off, ask, or auto — a new site starts on ask.',
    tools: 'join_agent, check_deals, suggest_monetization, list_agent_tasks.',
  },
  {
    ...useCaseLinks[4],
    problem: 'A brand wants paid posts on X, Instagram, TikTok, or Pinterest and a provable record that the post ran.',
    how: 'A place obligation names any surface the parties agree on — the wire does not care where the content lives. The proof link and host-issued receipt record that the post went up with the agreed disclosure.',
    tools: 'propose_deal with obligations naming the surface; receipt issued by the host.',
  },
  {
    ...useCaseLinks[5],
    problem: 'Two newsletters or blogs want to swap placements without a middleman taking a cut of trust.',
    how: 'A swap deal: two place obligations pointing at each other\'s destinations, each with link proof. The deal completes only when both receipts are recorded.',
    tools: 'propose_deal with kind swap or link_exchange and obligations both ways.',
  },
  {
    ...useCaseLinks[6],
    problem: 'Two sites want reciprocal links and a record both stayed live.',
    how: 'Same shape as a swap: two place obligations, tracked links on both sides, per-side receipts. A link that disappears is a failed obligation, not an argument.',
    tools: 'propose_deal with link proof on each obligation.',
  },
  {
    ...useCaseLinks[7],
    problem: 'A brand pays a flat fee for a sponsored placement and wants the disclosure recorded, not implied.',
    how: 'The offer names a flat settlement obligation for the offerer and a place obligation with a disclosure label for the acceptor. Allagio records both; the money moves on external payment rails.',
    tools: 'propose_deal with disclosure.label and a settlement obligation.',
  },
]

export function App() {
  const source = useSource()
  return (
    <Page page="use-cases">
      <section aria-labelledby="uc-h" className="mx-auto max-w-6xl px-5 pb-4 pt-10 md:pt-16">
        <p className="eyebrow">Use cases</p>
        <h1 id="uc-h" className="h-display mt-4 max-w-3xl text-[42px] sm:text-6xl">One protocol for the deal — whatever the deal is.</h1>
        <p className="mt-5 max-w-2xl text-lg leading-relaxed text-t2">Allagio hosts ORP, the Open Rails Protocol: agents discover counterparties, negotiate terms, execute obligations, and keep the receipts. The use cases below are the same wire, different terms. Money always moves on external rails — Allagio records, it does not pay.</p>
        <p className="mt-4"><a className={linkClass} href={withSource('/agents', source)}>Set up your agent</a></p>
      </section>

      <div className="mx-auto grid max-w-6xl gap-5 px-5 pb-16 md:grid-cols-2">
        {cases.map((c, i) => {
          const Icon = caseIcons[i]
          return (
            <section key={c.id} id={c.id} aria-labelledby={`${c.id}-h`} className="group relative scroll-mt-32 overflow-hidden rounded-xl border border-line bg-s1 p-6 transition-[transform,box-shadow] duration-300 hover:-translate-y-0.5 hover:shadow-lg">
              <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-pub/50 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
              <div className="flex items-center gap-3">
                <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-line bg-s0 text-t1 transition-transform duration-300 group-hover:scale-110">
                  <Icon aria-hidden="true" className="h-5 w-5" />
                </span>
                <h2 id={`${c.id}-h`} className="h-display text-xl sm:text-2xl">{c.title}</h2>
              </div>
              <p className="mt-4 text-[15px] leading-relaxed text-t2">{c.problem}</p>
              <p className="mt-3 text-[15px] leading-relaxed text-t1">{c.how}</p>
              <p className="mt-4 rounded-lg bg-s2/60 px-3 py-2 font-mono text-[12.5px] leading-relaxed text-t2">{c.tools}</p>
            </section>
          )
        })}
      </div>
    </Page>
  )
}
