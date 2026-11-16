import { BadgeCheck, EyeOff, Fingerprint, Link2, Signature, SlidersHorizontal, type LucideIcon } from "lucide-react"
import { motion } from "motion/react"

import { reducedMotion } from "@/lib/utils"
import { withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

const cardIcons: LucideIcon[] = [
  Signature,
  Link2,
  Fingerprint,
  EyeOff,
  BadgeCheck,
  SlidersHorizontal,
]

export function BuiltTrust({ source }: { source: string }) {
  const { t } = useLocale()
  const reveal = reducedMotion()
    ? {}
    : {
        initial: { opacity: 0, y: 20 },
        whileInView: { opacity: 1, y: 0 },
        viewport: { once: true, margin: "-60px" },
      }
  return (
    <section id="trust" aria-labelledby="trust-h">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-24">
        <div className="mx-auto max-w-5xl text-center">
          <p className="eyebrow">{t.trusted.eyebrow}</p>
          <h2
            id="trust-h"
            className="h-display mt-3 text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
          >
            {t.trusted.heading}
          </h2>
        </div>
        <div className="mx-auto mt-12 grid max-w-5xl gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {t.trusted.cards.map((card, i) => {
            const Icon = cardIcons[i] ?? BadgeCheck
            return (
              <motion.article
                key={card.title}
                {...reveal}
                transition={{ duration: 0.5, delay: i * 0.06, ease: "easeOut" }}
                className="rounded-2xl border border-line bg-s1 p-6 shadow-sm transition-shadow duration-300 hover:shadow-md"
              >
                <Icon aria-hidden="true" className="h-6 w-6 text-pub" />
                <h3 className="mt-4 text-lg font-semibold tracking-tight text-t1">
                  {card.title}
                </h3>
                <p className="mt-2 text-[15px] leading-relaxed text-t2">
                  {card.body}
                </p>
              </motion.article>
            )
          })}
        </div>
        <p className="mx-auto mt-10 max-w-3xl text-center text-[15px] leading-relaxed text-t2">
          <a
            className="font-semibold text-t1 underline underline-offset-4 hover:no-underline"
            href={withSource("/protocol/", source)}
          >
            {t.trusted.linePreLink}
          </a>
          {t.trusted.lineMiddle}
          <a
            className="font-semibold text-t1 underline underline-offset-4 hover:no-underline"
            href="/openapi.json"
          >
            {t.trusted.lineLink2}
          </a>
          {t.trusted.linePost}
        </p>
      </div>
    </section>
  )
}
