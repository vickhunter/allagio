import { ArrowRight } from "lucide-react"

import { Card, CardContent } from "@/components/ui/card"
import { withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

const stepLinks = [null, null, "/you/", "/publishers/"]

export function FeatureSection({ source }: { source: string }) {
  const { t } = useLocale()
  return (
    <section id="how" aria-labelledby="how-h">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-24">
        <div className="mx-auto max-w-5xl text-center">
          <p className="eyebrow">{t.how.eyebrow}</p>
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
        <div className="mx-auto mt-12 grid max-w-5xl gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {t.how.steps.map((step, i) => (
            <Card
              key={step.title}
              className="border-line bg-card shadow-none transition-[transform,box-shadow,border-color] duration-300 hover:-translate-y-1 hover:border-line-strong hover:shadow-md"
            >
              <CardContent className="flex h-full flex-col p-6 text-left">
                <div className="font-mono text-sm tracking-tight text-setup">
                  {String(i + 1).padStart(2, "0")}
                </div>
                <h3 className="mt-3 text-xl font-semibold tracking-tight text-t1">
                  {step.title}
                </h3>
                <p className="mt-2 flex-1 text-sm leading-relaxed text-t2">
                  {step.description}
                </p>
                {stepLinks[i] ? (
                  <a
                    href={withSource(stepLinks[i]!, source)}
                    className="mt-4 inline-flex min-h-11 items-center gap-1 font-semibold text-moss underline underline-offset-4 hover:no-underline"
                  >
                    {t.how.more}
                    <ArrowRight className="size-4" aria-hidden="true" />
                  </a>
                ) : null}
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}
