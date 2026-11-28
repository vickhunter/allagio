import { ChevronRight } from "lucide-react"

import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"
import { ShimmerButton } from "@/components/ui/shimmer-button"
import { signInHref, withSource } from "@/lib/source"
import { useLocale } from "@/i18n"
import { setupHref } from "@/home/hero-copy.mjs"

export function CallToAction({ source }: { source: string }) {
  const { t } = useLocale()
  const signIn = signInHref("/owner", source)
  return (
    <section id="cta" aria-labelledby="cta-h">
      <div className="container mx-auto flex w-full flex-col items-center justify-center p-4 md:p-10">
        <div className="relative flex w-full flex-col items-center justify-center overflow-hidden rounded-3xl border border-line bg-s1 p-10">
          <div className="z-10 flex flex-col items-center text-center">
            <h2
              id="cta-h"
              className="h-display mt-3 max-w-xl text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-4xl"
            >
              {t.cta.heading}
            </h2>
            <p className="mt-3 max-w-xl text-base text-balance text-t2">
              {t.cta.lead}
            </p>
            <div className="mt-5 flex flex-col items-center gap-3">
              <ShimmerButton href={withSource(setupHref, source)} className="text-base font-medium" data-track="secondary_click" data-track-to="agents">
                {t.chrome.setupCta}
              </ShimmerButton>
              <div className="flex flex-wrap items-center justify-center gap-4">
                <a
                  href={signIn}
                  data-track="join_submit"
                  data-track-cta="Sign in with Google"
                  className={cn(
                    buttonVariants({ size: "lg", variant: "outline" }),
                    "group rounded-full px-6"
                  )}
                >
                  {t.cta.signIn}
                  <ChevronRight
                    className="ml-1 size-4 transition-all duration-300 ease-out group-hover:translate-x-1"
                    aria-hidden="true"
                  />
                </a>
                <a
                  href="#join"
                  className="inline-flex min-h-11 items-center font-semibold text-t2 underline underline-offset-4 hover:no-underline"
                >
                  {t.cta.primary}
                </a>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
