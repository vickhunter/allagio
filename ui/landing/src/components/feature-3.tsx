import { ArrowRight } from "lucide-react"

import { Card, CardContent } from "@/components/ui/card"
import { withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

export function FeatureSection({ source }: { source: string }) {
  const { t } = useLocale()
  const stepLinks = [null, null, { href: "#approval-rules", label: t.how.more }, { href: withSource("/publishers/", source), label: t.hero.ctaPublisher }]
  return (
    <section id="how" aria-labelledby="how-h">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-24">
        <div className="mx-auto max-w-5xl text-center">
          <h2
            id="how-h"
            className="h-display mt-3 text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
          >
            {t.how.heading}
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-lg leading-relaxed text-t2">
            {t.how.lead}
          </p>
        </div>
        <ol className="mx-auto mt-12 grid max-w-5xl gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {t.how.steps.map((step, i) => {
            const link = stepLinks[i]
            return (
              <li key={step.title}>
                <Card className="h-full border-line bg-card shadow-none">
                  <CardContent className="flex h-full flex-col p-6 text-left">
                    <div aria-hidden="true" className="font-mono text-sm tracking-tight text-maker">
                      {i + 1}
                    </div>
                    <h3 className="mt-3 text-xl font-semibold tracking-tight text-t1">
                      {step.title}
                    </h3>
                    <p className="mt-2 flex-1 text-sm leading-relaxed text-t2">
                      {step.description}
                    </p>
                    {link ? (
                      <a
                        href={link.href}
                        className="mt-4 inline-flex min-h-11 items-center gap-1 font-semibold text-t1 underline underline-offset-4 hover:no-underline"
                      >
                        {link.label}
                        <ArrowRight className="size-4 shrink-0" aria-hidden="true" />
                      </a>
                    ) : null}
                  </CardContent>
                </Card>
              </li>
            )
          })}
        </ol>
      </div>
    </section>
  )
}
