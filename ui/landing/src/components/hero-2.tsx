import { useEffect, useRef, useState } from "react"
import { ArrowDownRight, ArrowRight, Check, Copy, Terminal, User } from "lucide-react"
import { motion, useInView } from "motion/react"

import { cn } from "@/lib/utils"
import { BorderBeam } from "@/components/ui/border-beam"
import { RetroGrid } from "@/components/ui/retro-grid"
import { clientMarks, joinPrompt } from "@/components/join-prompt.mjs"
import { setupHref } from "@/home/hero-copy.mjs"
import { signInHref, withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

type HeroAudience = "human" | "agent"

const fadeUpVariants = {
  initial: { opacity: 0, y: 20 },
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
  const { t } = useLocale()
  const [audience, setAudience] = useState<HeroAudience>(() =>
    typeof window !== "undefined" && window.location.hash === "#join" ? "agent" : "human"
  )
  const [copied, setCopied] = useState(false)
  const copyTimer = useRef<number | null>(null)

  useEffect(() => {
    const syncHash = () => {
      if (window.location.hash === "#join") {
        setAudience("agent")
        window.requestAnimationFrame(() => {
          document.getElementById("join")?.scrollIntoView({ block: "center" })
        })
      }
    }
    syncHash()
    window.addEventListener("hashchange", syncHash)
    return () => {
      window.removeEventListener("hashchange", syncHash)
      if (copyTimer.current !== null) window.clearTimeout(copyTimer.current)
    }
  }, [])

  async function copyQuickstart() {
    try {
      await navigator.clipboard.writeText(joinPrompt)
      setCopied(true)
      if (copyTimer.current !== null) window.clearTimeout(copyTimer.current)
      copyTimer.current = window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  return (
    <section id="hero" aria-labelledby="hero-h" className="band-light relative overflow-hidden">
      <div
        className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(ellipse_75%_55%_at_50%_0%,rgb(var(--rgb-pub-soft)/0.85),transparent_70%)]"
        data-hero-bg="fallback"
        aria-hidden="true"
      />
      {reducedMotion ? null : <RetroGrid className="z-0 opacity-60" />}

      <div className="relative z-10 mx-auto flex max-w-5xl flex-col items-center px-5 pt-16 pb-14 text-center md:pt-24 md:pb-20">
        <motion.div
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.45, ease }}
          className="inline-flex items-center gap-2 rounded-full border border-line bg-s1 px-3.5 py-1.5 shadow-2xs"
        >
          <span className="h-2 w-2 rounded-full bg-pub" aria-hidden="true" />
          <span className="eyebrow text-t1">{t.hero.eyebrow}</span>
        </motion.div>

        <motion.h1
          id="hero-h"
          ref={fadeInRef}
          className="h-display mt-6 max-w-4xl text-4xl font-bold tracking-tighter text-balance text-t1 sm:text-5xl md:text-6xl md:leading-[1.03]"
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.55, delay: 0.08, ease }}
        >
          {t.hero.h1}
        </motion.h1>

        <motion.p
          className="mt-5 max-w-2xl text-base leading-relaxed font-medium text-balance text-t2 sm:text-lg md:text-xl"
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.55, delay: 0.14, ease }}
        >
          {t.hero.support}
        </motion.p>

        {/* Human | Agent audience switcher */}
        <motion.div
          animate={fadeInInView ? "animate" : "initial"}
          variants={fadeUpVariants}
          initial={false}
          transition={{ duration: 0.55, delay: 0.2, ease }}
          className="mt-8 flex w-full flex-col items-center"
        >
          <div
            role="tablist"
            aria-label={t.hero.toggleLabel}
            className="inline-flex items-center rounded-full border border-line-strong bg-s1 p-1 shadow-xs"
          >
            <button
              type="button"
              role="tab"
              id="hero-tab-human"
              aria-selected={audience === "human"}
              aria-controls="hero-panel-human"
              onClick={() => setAudience("human")}
              className={cn(
                "inline-flex min-h-10 items-center gap-2 rounded-full px-5 text-sm font-semibold transition-colors duration-150",
                audience === "human"
                  ? "bg-t1 text-s0 shadow-xs"
                  : "text-t2 hover:text-t1"
              )}
            >
              <User className="h-4 w-4" aria-hidden="true" />
              {t.hero.toggleHuman}
            </button>
            <button
              type="button"
              role="tab"
              id="hero-tab-agent"
              aria-selected={audience === "agent"}
              aria-controls="hero-panel-agent"
              onClick={() => setAudience("agent")}
              className={cn(
                "inline-flex min-h-10 items-center gap-2 rounded-full px-5 text-sm font-semibold transition-colors duration-150",
                audience === "agent"
                  ? "bg-t1 text-s0 shadow-xs"
                  : "text-t2 hover:text-t1"
              )}
            >
              <Terminal className="h-4 w-4" aria-hidden="true" />
              {t.hero.toggleAgent}
            </button>
          </div>

          {audience === "human" ? (
            <div
              id="hero-panel-human"
              role="tabpanel"
              aria-labelledby="hero-tab-human"
              className="mt-7 flex w-full flex-col items-center gap-4"
            >
              <div className="flex flex-wrap items-center justify-center gap-3">
                <a
                  className="inline-flex min-h-12 items-center gap-2 rounded-full bg-t1 px-6 text-sm font-semibold text-s0 transition-[transform,box-shadow] duration-150 hover:-translate-y-px hover:shadow-md"
                  href={withSource(setupHref, source)}
                  data-primary-action
                  data-track="hero_cta"
                  data-track-to="agents"
                >
                  {t.hero.ctaPrimary}
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </a>
                <a
                  className="inline-flex min-h-12 items-center gap-2 rounded-full border border-line-strong bg-s1 px-6 text-sm font-semibold text-t1 transition-[transform,box-shadow,background-color] duration-150 hover:-translate-y-px hover:bg-s2 hover:shadow-sm"
                  href="#simulator-slot"
                  data-track="hero_cta"
                  data-track-to="simulator"
                >
                  {t.hero.ctaPreview}
                  <ArrowDownRight className="h-4 w-4 text-pub" aria-hidden="true" />
                </a>
              </div>

              <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-1 text-sm text-t2">
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                  href={withSource(t.hero.ctaMakerHref, source)}
                  data-track="hero_cta"
                  data-track-to="makers"
                >
                  {t.hero.ctaMaker}
                </a>
                <span aria-hidden="true" className="text-line-strong">·</span>
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                  href={withSource(t.hero.ctaPublisherHref, source)}
                  data-track="hero_cta"
                  data-track-to="publishers"
                >
                  {t.hero.ctaPublisher}
                </a>
                <span aria-hidden="true" className="text-line-strong">·</span>
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t2 underline decoration-line underline-offset-4 transition-colors hover:text-t1 hover:decoration-t1"
                  href={signInHref("/owner", source)}
                  data-track="hero_signin"
                >
                  {t.chrome.signIn}
                </a>
              </div>
            </div>
          ) : (
            <div
              id="hero-panel-agent"
              role="tabpanel"
              aria-labelledby="hero-tab-agent"
              className="band-dark relative mt-7 w-full max-w-[46rem] overflow-hidden rounded-2xl border border-line-strong p-5 text-left shadow-xl sm:p-6"
            >
              <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line pb-3">
                <div className="flex items-center gap-2">
                  <span className="h-2.5 w-2.5 rounded-full bg-pub/80" aria-hidden="true" />
                  <span className="font-mono text-xs font-medium text-t1">{t.hero.agentTitle}</span>
                </div>
                <div className="flex flex-wrap items-center gap-1.5 font-mono text-[11px]">
                  <span className="rounded-md bg-pub-soft px-2 py-0.5 text-pub">POST /orp/join</span>
                  <span className="rounded-md bg-s2 px-2 py-0.5 text-t2">/mcp</span>
                  <a href="/llms.txt" className="rounded-md bg-s2 px-2 py-0.5 text-t1 underline-offset-2 hover:underline">llms.txt</a>
                </div>
              </div>

              <p className="mt-3 text-sm leading-relaxed text-t2">{t.hero.agentLead}</p>

              <div id="join" className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center">
                <pre className="min-w-0 flex-1 whitespace-pre-wrap break-words rounded-xl border border-line bg-s1 px-3.5 py-3 font-mono text-xs leading-relaxed text-t1 sm:text-[13px]">
                  <code>{joinPrompt}</code>
                </pre>
                <button
                  type="button"
                  onClick={copyQuickstart}
                  className={cn(
                    "inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-xl px-4 text-sm font-semibold transition-colors duration-150",
                    copied
                      ? "bg-pub text-s0"
                      : "bg-t1 text-s0 hover:bg-t1/90"
                  )}
                >
                  {copied ? <Check className="h-4 w-4" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}
                  {copied ? t.connect.copiedLabel : t.connect.copyLabel}
                </button>
              </div>
              <p role="status" aria-live="polite" className="mt-1.5 min-h-5 font-mono text-xs text-pub">
                {copied ? t.connect.copiedStatus : ""}
              </p>

              <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-line pt-3 text-xs font-semibold">
                <a href="/llms.txt" className="inline-flex min-h-9 items-center text-pub underline underline-offset-4 hover:no-underline">
                  llms.txt
                </a>
                <a href="/docs" className="inline-flex min-h-9 items-center text-t1 underline underline-offset-4 hover:no-underline">
                  {t.hero.agentDocsLink}
                </a>
                <a href={withSource(setupHref, source)} className="inline-flex min-h-9 items-center text-t2 underline underline-offset-4 hover:text-t1 hover:no-underline">
                  {t.hero.agentGuideLink}
                </a>
              </div>

              <details className="mt-2 border-t border-line pt-2 text-xs text-t2">
                <summary className="inline-flex min-h-10 cursor-pointer items-center font-semibold text-t1">
                  {t.connect.summary}
                </summary>
                <ol className="mt-2 space-y-1 pl-4 font-mono text-[12px] text-t2 list-decimal">
                  {t.connect.steps.map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>
                <p className="mt-2 text-xs text-t2">{t.connect.noSignIn}</p>
              </details>

              {!reducedMotion && <BorderBeam size={160} duration={9} borderWidth={1.5} />}
            </div>
          )}
        </motion.div>

        {/* Compact agent-client trust strip */}
        <div className="mt-12 w-full border-t border-line/80 pt-8">
          <p className="font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-t2">
            {t.clients.heading}
          </p>
          <ul className="mt-4 flex flex-wrap items-center justify-center gap-2">
            {clientMarks.map((mark) => (
              <li
                key={mark.name}
                className="inline-flex items-center gap-1.5 rounded-full border border-line bg-s1 px-3 py-1.5 text-xs font-semibold text-t1"
              >
                <svg viewBox="0 0 24 24" width={13} height={13} role="img" aria-label={mark.name} className="shrink-0 opacity-75">
                  <path d={mark.d} fill="currentColor" />
                </svg>
                {mark.name}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  )
}
