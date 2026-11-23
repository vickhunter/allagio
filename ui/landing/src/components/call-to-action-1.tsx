import { ArrowRight, ChevronRight } from "lucide-react"

import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"
import { signInHref, withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

export function CallToAction({ source }: { source: string }) {
  const { t } = useLocale()
  const signIn = signInHref("/owner", source)

  return (
    <section id="cta" aria-labelledby="cta-h">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-24">
        <div className="flex flex-col items-center rounded-2xl border border-line bg-s1 px-6 py-12 text-center md:px-12 md:py-16">
          <p className="eyebrow text-t2">{t.cta.eyebrow}</p>
          <h2
            id="cta-h"
            className="h-display mt-3 max-w-xl text-3xl font-bold tracking-tighter text-balance text-t1 sm:text-4xl"
          >
            {t.cta.heading}
          </h2>
          <p className="mt-3 max-w-xl text-base leading-relaxed text-balance text-t2">
            {t.cta.lead}
          </p>
          <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
            <a
              href={withSource("/agents", source)}
              data-track="cta_setup"
              className="inline-flex min-h-12 items-center gap-2 rounded-full bg-t1 px-6 text-sm font-semibold text-s0 transition-opacity duration-150 hover:opacity-90"
            >
              {t.chrome.setupCta}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </a>
            <a
              href={signIn}
              data-track="join_submit"
              data-track-cta="Sign in with Google"
              className={cn(
                buttonVariants({ size: "lg", variant: "outline" }),
                "group rounded-full border-line-strong bg-s0 px-6 text-t1 hover:bg-s2"
              )}
            >
              {t.cta.signIn}
              <ChevronRight
                className="ml-1 size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5"
                aria-hidden="true"
              />
            </a>
            <a
              href="/docs"
              data-track="secondary_click"
              data-track-to="docs"
              className="inline-flex min-h-11 items-center px-3 text-sm font-semibold text-t2 underline decoration-line-strong underline-offset-4 transition-colors hover:text-t1 hover:decoration-t1"
            >
              {t.chrome.navDocs}
            </a>
          </div>
        </div>
      </div>
    </section>
  )
}
