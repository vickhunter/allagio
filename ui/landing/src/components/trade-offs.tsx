import { Check, X } from "lucide-react"
import { motion } from "motion/react"

import { reducedMotion } from "@/lib/utils"

const withOrp = [
  "Your agent negotiates and executes the deal — the decide step still waits for a person.",
  "One wire for affiliate offers, sponsorships, link exchanges and agent partnerships.",
  "Signed envelopes and a protocol receipt per deal — GET /orp/proof/:id returns it.",
  "No platform lock-in — ORP is an open wire; Apache-2.0 is planned.",
  "Money moves on external rails — Allagio records the deal, it does not pay out.",
]

const withoutOrp = [
  "A person reads every DM and email thread by hand.",
  "Terms scatter across inboxes — nothing portable to audit.",
  "Attribution lives in spreadsheets, reconciled by hand.",
  "Each new counterparty means a bespoke integration.",
  "No shared receipts — a dispute falls back to screenshots.",
]

export function TradeOffs() {
  const reveal = reducedMotion()
    ? {}
    : {
        initial: { opacity: 0, y: 20 },
        whileInView: { opacity: 1, y: 0 },
        viewport: { once: true, margin: "-60px" },
      }
  return (
    <section id="trade-offs" aria-labelledby="trade-offs-h">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-24">
        <div className="mx-auto max-w-5xl text-center">
          <p className="eyebrow">Honest trade-offs</p>
          <h2
            id="trade-offs-h"
            className="h-display mt-3 text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
          >
            With ORP, or by hand.
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-lg leading-relaxed text-t2">
            Same deals, different wiring. Allagio runs ORP in test mode today —
            no real money moves.
          </p>
        </div>
        <div className="mx-auto mt-12 grid max-w-4xl gap-6 sm:grid-cols-2">
          <motion.article
            {...reveal}
            transition={{ duration: 0.5, ease: "easeOut" }}
            className="relative overflow-hidden rounded-2xl border border-pub/40 bg-s1 p-6 shadow-sm transition-shadow duration-300 hover:shadow-md"
          >
            <div
              className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-pub/70 to-transparent"
              aria-hidden="true"
            />
            <div className="flex flex-wrap items-center gap-2.5">
              <h3 className="text-xl font-semibold tracking-tight text-t1">
                With Allagio (ORP)
              </h3>
              <span className="rounded-full border border-pub/30 bg-pub-soft px-2 py-0.5 font-mono text-[11px] uppercase tracking-wide text-pub">
                test mode
              </span>
            </div>
            <ul className="mt-5 grid gap-3.5">
              {withOrp.map((item) => (
                <li
                  key={item}
                  className="flex gap-3 text-[15px] leading-relaxed text-t1"
                >
                  <Check
                    aria-hidden="true"
                    className="mt-1 h-4 w-4 shrink-0 text-pub"
                  />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </motion.article>
          <motion.article
            {...reveal}
            transition={{ duration: 0.5, delay: 0.1, ease: "easeOut" }}
            className="rounded-2xl border border-line bg-s2/50 p-6 transition-shadow duration-300 hover:shadow-md"
          >
            <h3 className="text-xl font-semibold tracking-tight text-t1">
              Without it, by hand
            </h3>
            <ul className="mt-5 grid gap-3.5">
              {withoutOrp.map((item) => (
                <li
                  key={item}
                  className="flex gap-3 text-[15px] leading-relaxed text-t2"
                >
                  <X
                    aria-hidden="true"
                    className="mt-1 h-4 w-4 shrink-0 text-maker"
                  />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </motion.article>
        </div>
      </div>
    </section>
  )
}
