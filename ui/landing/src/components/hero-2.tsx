import { Suspense, lazy, useEffect, useRef, useState } from "react"
import { motion, useInView } from "motion/react"

import { BorderBeam } from "@/components/ui/border-beam"
import { ShimmerButton } from "@/components/ui/shimmer-button"
import { JoinPrompt, joinPrompt } from "@/components/join-prompt.mjs"
import { example } from "@/content"
import { SetupSequence } from "@/home/SetupSequence"
import { setupHref } from "@/home/hero-copy.mjs"
import { withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

const fadeUpVariants = {
  initial: { opacity: 0, y: 24 },
  animate: { opacity: 1, y: 0 },
}

const LazyDither = lazy(() => import("@/components/ui/dither").then((module) => ({ default: module.Dither })))

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
  const { t } = useLocale()
  const steps = t.cta.feed.slice(0, 4)
  const split = [example.publisher, example.maker, example.fee]

  return (
    <section id="hero" aria-labelledby="hero-h" className="relative overflow-hidden">
      <div
        className="pointer-events-none absolute inset-0 z-0 bg-s0"
        data-hero-bg="fallback"
        aria-hidden="true"
      />
      <div className="relative z-10 container mx-auto flex flex-col items-center px-4 pt-10 pb-8 text-center md:px-10 md:pt-24 md:pb-10" ref={fadeInRef}>
        <motion.p
          className="eyebrow"
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.5, ease }}
        >
          {t.hero.eyebrow}
        </motion.p>
        <motion.h1
          id="hero-h"
          className="h-display editorial-title mt-5 max-w-4xl text-5xl leading-[.98] text-balance text-t1 sm:text-6xl md:text-8xl"
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.6, delay: 0.1, ease, type: "spring" }}
        >
          {t.hero.h1}
        </motion.h1>
        <motion.p
          className="mt-5 max-w-[36rem] text-base leading-relaxed text-pretty text-t2 sm:text-lg"
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.6, delay: 0.2, ease, type: "spring" }}
        >
          {t.hero.support}
        </motion.p>
        <motion.div
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.6, delay: 0.3, ease, type: "spring" }}
          className="mt-7 flex flex-col items-center gap-4"
        >
          <div className="flex flex-wrap items-center justify-center gap-3">
            <ShimmerButton
              className="min-h-11 text-sm font-semibold text-s0"
              background="rgb(35 39 32)"
              href={withSource(setupHref, source)}
              data-primary-action
              data-track="hero_cta"
              data-track-to="agents"
            >
              {t.chrome.setupCta}
            </ShimmerButton>
            <a
              className="inline-flex min-h-11 items-center rounded-full border border-line bg-s0/85 px-6 text-sm font-semibold text-t1 backdrop-blur transition-[transform,box-shadow] duration-150 hover:-translate-y-px hover:shadow-md"
              href="#how"
              data-track="hero_cta"
              data-track-to="how"
            >
              {t.hero.secondary}
            </a>
          </div>
          <p className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-sm text-t2">
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <a className="inline-flex min-h-11 items-center font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource(t.hero.ctaMakerHref, source)} data-track="hero_audience" data-track-to="makers">
                {t.hero.ctaMaker}
              </a>
              <span aria-hidden="true">·</span>
              <a className="inline-flex min-h-11 items-center font-semibold text-t1 underline underline-offset-4 hover:no-underline" href={withSource(t.hero.ctaPublisherHref, source)} data-track="hero_audience" data-track-to="publishers">
                {t.hero.ctaPublisher}
              </a>
            </span>
          </p>
        </motion.div>
      </div>

      <div className="relative z-10 container mx-auto px-4 pb-12 md:px-10 md:pb-20">
        <motion.aside
          aria-labelledby="hero-example-h"
          className="relative mx-auto max-w-4xl overflow-hidden rounded-2xl border border-line bg-s1"
          initial={reducedMotion ? false : { opacity: 0, y: 24 }}
          animate={fadeInInView ? { opacity: 1, y: 0 } : undefined}
          transition={{ duration: 0.6, delay: 0.25, ease }}
        >
          {reducedMotion ? null : <Suspense fallback={null}>
            <LazyDither
              pattern="orbit"
              colors={["rgb(248 245 238)", "rgb(221 214 202)", "rgb(167 157 141)"]}
              intensity={0.13}
              speed={0.24}
              className="z-0 opacity-35"
            />
          </Suspense>}
          <div className="relative z-10">
            <div className="border-b border-line bg-s0/72 px-5 py-4 text-left">
              <p className="eyebrow">{t.hero.exampleLabel}</p>
              <h2 id="hero-example-h" className="mt-1 text-xl font-semibold tracking-tight text-t1">{t.hero.exampleTitle}</h2>
            </div>
            <ol className="grid gap-0 px-5 py-2 text-left md:grid-cols-2 md:gap-x-8">
              {steps.map((step, i) => (
                <li key={step.text} className="relative flex gap-3 py-3">
                  <span className="relative flex flex-col items-center" aria-hidden="true">
                    <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full border font-mono text-[11px] ${i === 3 ? 'border-pub bg-pub text-s0' : 'border-line bg-s0 text-t2'}`}>{i + 1}</span>
                    {i < steps.length - 1 ? <span className="w-px flex-1 bg-line md:hidden" /> : null}
                  </span>
                  <span className="min-w-0">
                    <span className="block text-sm font-semibold leading-snug text-t1">{step.text}</span>
                    <span className="mt-0.5 block font-mono text-xs text-t2">{step.meta}</span>
                  </span>
                </li>
              ))}
            </ol>
            <div className="grid grid-cols-3 border-t border-line bg-s2/45">
              {split.map((value, i) => (
                <div key={t.hero.splitLabels[i]} className="border-l border-line px-3 py-4 first:border-l-0">
                  <p className="font-mono text-[10px] uppercase tracking-wide text-t2">{t.hero.splitLabels[i]}</p>
                  <p className="mt-1 font-display text-lg font-semibold tracking-tight text-t1 sm:text-xl">{value}</p>
                </div>
              ))}
            </div>
            <p className="border-t border-line px-5 py-3 text-left text-xs leading-relaxed text-t2">{t.hero.exampleFootnote}</p>
          </div>
        </motion.aside>
      </div>
    </section>
  )
}

export function ConnectionCard({ source }: { source: string }) {
  const { t } = useLocale()
  return (
    <div className="relative mx-auto w-full max-w-4xl">
      <div className="relative overflow-hidden rounded-2xl border border-line bg-card">
        <div className="grid gap-6 p-6 text-left md:grid-cols-[minmax(0,1fr)_minmax(280px,.85fr)] md:p-8">
          <div id="join" className="flex flex-col justify-center gap-3">
            <h2 className="h-display text-2xl font-semibold tracking-tight text-t1 md:text-3xl">
              {t.connect.heading}
            </h2>
            <p className="text-base leading-relaxed text-t2">{t.connect.lead}</p>
            <a href={withSource(setupHref, source)} className="inline-flex min-h-11 self-start items-center font-semibold text-t1 underline underline-offset-4 hover:no-underline">
              {t.chrome.setupCta}
            </a>
            <details className="setup-details mt-2">
              <summary>{t.connect.summary}</summary>
              <p>{t.connect.joinLead}</p>
              <p>{joinPrompt}</p>
              <p>
                <a href="/llms.txt">llms.txt</a>
              </p>
              {t.connect.steps.map((step) => (
                <p key={step}>{step}</p>
              ))}
              <p>{t.connect.noSignIn}</p>
            </details>
            <div className="mt-2">
              <JoinPrompt
                line={false}
                copyLabel={t.connect.copyLabel}
                copiedLabel={t.connect.copiedLabel}
                copiedStatus={t.connect.copiedStatus}
              />
            </div>
          </div>
          <SetupSequence />
        </div>
        <BorderBeam size={160} />
      </div>
    </div>
  )
}
