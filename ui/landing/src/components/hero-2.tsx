import { useEffect, useRef, useState } from "react"
import { motion, useInView } from "motion/react"

import { cn } from "@/lib/utils"
import { BorderBeam } from "@/components/ui/border-beam"
import { RetroGrid } from "@/components/ui/retro-grid"
import { JoinPrompt, joinLead, joinPrompt } from "@/components/join-prompt.mjs"
import { SetupSequence } from "@/home/SetupSequence"
import {
  heroH1,
  heroSupport,
  manualSetup,
  noSignIn,
  setupHref,
  setupLabel,
  setupSteps,
} from "@/home/hero-copy.mjs"
import { withSource } from "@/lib/source"

const fadeUpVariants = {
  initial: { opacity: 0, y: 24 },
  animate: { opacity: 1, y: 0 },
}

const ease = [0.21, 0.47, 0.32, 0.98] as const

function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(
    () =>
      typeof window !== "undefined" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
  )
  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)")
    const handleChange = () => setReduced(query.matches)
    query.addEventListener("change", handleChange)
    return () => query.removeEventListener("change", handleChange)
  }, [])
  return reduced
}

export function Hero({ source }: { source: string }) {
  const fadeInRef = useRef(null)
  const fadeInInView = useInView(fadeInRef, { once: true })
  const reducedMotion = usePrefersReducedMotion()

  return (
    <section id="hero" aria-labelledby="hero-h">
      <div className="relative h-full overflow-hidden">
        <div
          className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(ellipse_70%_50%_at_50%_100%,var(--color-pub-soft),transparent)]"
          data-hero-bg="fallback"
          aria-hidden="true"
        />
        {reducedMotion ? null : <RetroGrid className="z-0" />}
        <div className="z-10 container mx-auto flex flex-col px-4 pt-24 pb-10 md:px-10 md:pt-32 md:pb-16">
          <div className="flex flex-col items-center gap-6 text-center">
            <div className="flex max-w-3xl flex-col items-center gap-4">
              <motion.p
                className="eyebrow"
                animate={fadeInInView ? "animate" : "initial"}
                variants={fadeUpVariants}
                initial={false}
                transition={{ duration: 0.5, ease }}
              >
                Allagio · agent to agent
              </motion.p>
              <motion.h1
                id="hero-h"
                ref={fadeInRef}
                className="h-display from-foreground to-foreground/60 bg-linear-to-br from-30% bg-clip-text text-4xl leading-[1.15] font-semibold tracking-tighter text-balance text-transparent sm:text-5xl md:text-6xl md:leading-[1.05]"
                animate={fadeInInView ? "animate" : "initial"}
                variants={fadeUpVariants}
                initial={false}
                transition={{ duration: 0.6, delay: 0.1, ease, type: "spring" }}
              >
                {heroH1}
              </motion.h1>
              <motion.p
                className="text-muted-foreground max-w-2xl text-lg font-medium tracking-tight text-balance md:text-xl"
                animate={fadeInInView ? "animate" : "initial"}
                variants={fadeUpVariants}
                initial={false}
                transition={{ duration: 0.6, delay: 0.2, ease, type: "spring" }}
              >
                {heroSupport}
              </motion.p>
            </div>

            <motion.div
              animate={fadeInInView ? "animate" : "initial"}
              variants={fadeUpVariants}
              initial={false}
              transition={{ duration: 0.6, delay: 0.3, ease, type: "spring" }}
              className="flex w-full flex-col items-center gap-3"
            >
              <div id="join">
                <JoinPrompt line={false} />
              </div>
              <a
                className="inline-flex min-h-11 items-center font-semibold text-setup underline underline-offset-4 hover:no-underline"
                href={withSource(setupHref, source)}
                data-primary-action
                data-track="secondary_click"
                data-track-to="agents"
              >
                {setupLabel}
              </a>
            </motion.div>
          </div>

          <motion.div
            animate={fadeInInView ? "animate" : "initial"}
            variants={fadeUpVariants}
            initial={false}
            transition={{ duration: 1.2, delay: 0.4, ease, type: "spring" }}
            className="relative mx-auto mt-12 w-full max-w-4xl"
          >
            <div
              className={cn(
                "absolute inset-0 bottom-1/2 h-full w-full transform-gpu blur-[120px] filter",
                "bg-[linear-gradient(to_bottom,var(--color-pub-soft),transparent_30%)]"
              )}
              aria-hidden="true"
            />
            <div className="relative overflow-hidden rounded-2xl border border-line bg-card/80 shadow-sm backdrop-blur-sm">
              <div className="grid gap-6 p-6 text-left md:grid-cols-[minmax(0,1fr)_minmax(280px,.85fr)] md:p-8">
                <div className="flex flex-col justify-center gap-3">
                  <p className="eyebrow">Connection</p>
                  <h2 className="h-display text-2xl font-semibold tracking-tight text-t1 md:text-3xl">
                    You send the bearer yourself.
                  </h2>
                  <p className="text-base leading-relaxed text-t2">
                    {joinLead}. {manualSetup} This page does not issue a token.
                  </p>
                  <details className="setup-details mt-2">
                    <summary>Setup details</summary>
                    <p>{joinLead}</p>
                    <p>{joinPrompt}</p>
                    <p>
                      <a href="/llms.txt">llms.txt</a>
                    </p>
                    {setupSteps.map((step) => (
                      <p key={step}>{step}</p>
                    ))}
                    <p>{noSignIn}</p>
                  </details>
                </div>
                <SetupSequence />
              </div>
              <BorderBeam size={160} />
              <BorderBeam size={160} delay={7} />
            </div>
          </motion.div>
        </div>
      </div>
    </section>
  )
}
