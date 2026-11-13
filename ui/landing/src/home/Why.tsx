import { BadgePercent, Compass, ShieldCheck } from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'
import { whyCards, whyHeading, whyTrust } from '@/home/why-copy.mjs'

const icons = [Compass, BadgePercent, ShieldCheck]

export function Why() {
  return (
    <section id="why" aria-labelledby="why-h" className="border-t border-line bg-s1">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-20">
        <div className="mx-auto max-w-5xl text-center">
          <h2
            id="why-h"
            className="h-display text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
          >
            {whyHeading}
          </h2>
        </div>
        <div className="mx-auto mt-12 grid max-w-5xl gap-6 sm:grid-cols-3">
          {whyCards.map((card, i) => {
            const Icon = icons[i]
            return (
              <Card
                key={card.title}
                className="border-line bg-card shadow-none transition-[transform,box-shadow,border-color] duration-300 hover:-translate-y-1 hover:border-line-strong hover:shadow-md"
              >
                <CardContent className="flex h-full flex-col p-6 text-left">
                  <span className="inline-flex size-10 items-center justify-center rounded-xl bg-s2 text-setup">
                    <Icon className="size-5" aria-hidden="true" />
                  </span>
                  <h3 className="mt-4 text-xl font-semibold tracking-tight text-t1">
                    {card.title}
                  </h3>
                  <p className="mt-2 flex-1 text-sm leading-relaxed text-t2">
                    {card.body}
                  </p>
                </CardContent>
              </Card>
            )
          })}
        </div>
        <p className="mx-auto mt-10 max-w-2xl text-center text-sm leading-relaxed text-t2">
          {whyTrust}
        </p>
      </div>
    </section>
  )
}
