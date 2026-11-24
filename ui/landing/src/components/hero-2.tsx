import { useEffect, useRef, useState, type KeyboardEvent } from "react"
import { ArrowDownRight, ArrowRight, Check, Copy, ShieldCheck, Terminal, User } from "lucide-react"
import { motion, useInView, useReducedMotion } from "motion/react"

import { cn } from "@/lib/utils"
import { joinPrompt } from "@/components/join-prompt.mjs"
import { setupHref } from "@/home/hero-copy.mjs"
import { signInHref, withSource } from "@/lib/source"
import { useLocale } from "@/i18n"

type HeroAudience = "human" | "agent"

const fadeUpVariants = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
}

const ease = [0.21, 0.47, 0.32, 0.98] as const

export function Hero({ source }: { source: string }) {
  const fadeInRef = useRef(null)
  const fadeInInView = useInView(fadeInRef, { once: true })
  const prefersReducedMotion = useReducedMotion()
  const { t } = useLocale()
  const animState = prefersReducedMotion || fadeInInView ? "animate" : "initial"
  const animTransition = (delay = 0) =>
    prefersReducedMotion ? { duration: 0 } : { duration: 0.45, delay, ease }
  const [audience, setAudience] = useState<HeroAudience>(() =>
    typeof window !== "undefined" && window.location.hash === "#join" ? "agent" : "human"
  )
  const [copied, setCopied] = useState(false)
  const copyTimer = useRef<number | null>(null)
  const humanTabRef = useRef<HTMLButtonElement>(null)
  const agentTabRef = useRef<HTMLButtonElement>(null)

  function onTabKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    let next: HeroAudience | null = null
    if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
      next = audience === "human" ? "agent" : "human"
    } else if (event.key === "Home") {
      next = "human"
    } else if (event.key === "End") {
      next = "agent"
    }
    if (!next) return
    event.preventDefault()
    setAudience(next)
    ;(next === "human" ? humanTabRef : agentTabRef).current?.focus()
  }

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
      <div data-hero-bg="fallback" aria-hidden="true" className="pointer-events-none absolute inset-0" />
      <div className="relative z-10 mx-auto flex max-w-5xl flex-col items-center px-5 pt-12 pb-12 text-center md:pt-18 md:pb-16">
        <motion.p
          animate={animState}
          variants={fadeUpVariants}
          initial={prefersReducedMotion ? false : "initial"}
          transition={animTransition(0)}
          className="eyebrow text-t2"
        >
          {t.hero.eyebrow}
        </motion.p>

        <motion.h1
          id="hero-h"
          ref={fadeInRef}
          className="h-display mt-5 max-w-4xl text-4xl font-bold tracking-tighter text-balance text-t1 sm:text-5xl md:text-6xl md:leading-[1.03]"
          animate={animState}
          variants={fadeUpVariants}
          initial={prefersReducedMotion ? false : "initial"}
          transition={animTransition(0.06)}
        >
          {t.hero.h1}
        </motion.h1>

        <motion.p
          className="mt-4 max-w-2xl text-base leading-relaxed font-medium text-balance text-t2 sm:text-lg"
          animate={animState}
          variants={fadeUpVariants}
          initial={prefersReducedMotion ? false : "initial"}
          transition={animTransition(0.12)}
        >
          {t.hero.support}
        </motion.p>

        {/* Human | Agent audience switcher */}
        <motion.div
          animate={animState}
          variants={fadeUpVariants}
          initial={prefersReducedMotion ? false : "initial"}
          transition={animTransition(0.16)}
          className="mt-7 flex w-full flex-col items-center"
        >
          <div
            role="tablist"
            aria-label={t.hero.toggleLabel}
            className="inline-flex items-center rounded-full border border-line-strong bg-s1 p-1 shadow-xs"
          >
            <button
              ref={humanTabRef}
              type="button"
              role="tab"
              id="hero-tab-human"
              tabIndex={audience === "human" ? 0 : -1}
              aria-selected={audience === "human"}
              aria-controls="hero-panel-human"
              onClick={() => setAudience("human")}
              onKeyDown={onTabKeyDown}
              className={cn(
                "inline-flex min-h-11 items-center gap-2 rounded-full px-5 text-sm font-semibold transition-[color,background-color,transform] duration-150 active:scale-[0.98]",
                audience === "human"
                  ? "bg-t1 text-s0 shadow-xs"
                  : "text-t2 hover:text-t1"
              )}
            >
              <User className="h-4 w-4" aria-hidden="true" />
              {t.hero.toggleHuman}
            </button>
            <button
              ref={agentTabRef}
              type="button"
              role="tab"
              id="hero-tab-agent"
              tabIndex={audience === "agent" ? 0 : -1}
              aria-selected={audience === "agent"}
              aria-controls="hero-panel-agent"
              onClick={() => setAudience("agent")}
              onKeyDown={onTabKeyDown}
              className={cn(
                "inline-flex min-h-11 items-center gap-2 rounded-full px-5 text-sm font-semibold transition-[color,background-color,transform] duration-150 active:scale-[0.98]",
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
              className="mt-6 flex w-full flex-col items-center gap-4"
            >
              <div className="flex flex-wrap items-center justify-center gap-3">
                <a
                  className="inline-flex min-h-12 items-center gap-2 rounded-full bg-t1 px-6 text-sm font-semibold text-s0 transition-[transform,box-shadow] duration-150 hover:-translate-y-px hover:shadow-md active:scale-[0.98]"
                  href={withSource(setupHref, source)}
                  data-primary-action
                  data-track="hero_cta"
                  data-track-to="agents"
                >
                  {t.hero.ctaPrimary}
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </a>
                <a
                  className="inline-flex min-h-12 items-center gap-2 rounded-full border border-line-strong bg-s1 px-6 text-sm font-semibold text-t1 transition-[transform,box-shadow,background-color] duration-150 hover:-translate-y-px hover:bg-s2 hover:shadow-sm active:scale-[0.98]"
                  href="#simulator-slot"
                  data-track="hero_cta"
                  data-track-to="simulator"
                >
                  {t.hero.ctaPreview}
                  <ArrowDownRight className="h-4 w-4 text-maker-brand" aria-hidden="true" />
                </a>
              </div>

              <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-1 text-sm text-t2">
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                  href={withSource(t.hero.ctaMakerHref, source)}
                  data-track="hero_cta"
                  data-track-to="makers"
                >
                  {t.hero.ctaMaker}
                </a>
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                  href={withSource(t.hero.ctaPublisherHref, source)}
                  data-track="hero_cta"
                  data-track-to="publishers"
                >
                  {t.hero.ctaPublisher}
                </a>
                <a
                  className="inline-flex min-h-11 items-center font-semibold text-t2 underline decoration-line underline-offset-4 transition-colors hover:text-t1 hover:decoration-t1"
                  href={signInHref("/owner", source)}
                  data-track="hero_signin"
                >
                  {t.chrome.signIn}
                </a>
              </div>

              {/* Compact above-the-fold product stage preview */}
              <div
                aria-label="Illustrative product flow: offer to human approval to labelled placement"
                className="mt-4 w-full max-w-4xl rounded-2xl border border-line-strong bg-s1 p-4 text-left shadow-sm sm:p-5"
              >
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line pb-3">
                  <span className="font-mono text-[11px] font-semibold uppercase tracking-[0.12em] text-t2">
                    Illustrative flow · Offer to labelled placement
                  </span>
                  <a
                    href="#simulator-slot"
                    className="inline-flex min-h-11 items-center gap-1 font-mono text-xs font-semibold text-maker-brand underline-offset-4 hover:underline"
                  >
                    Step through in simulator
                    <ArrowDownRight className="h-3.5 w-3.5" aria-hidden="true" />
                  </a>
                </div>
                <div className="mt-3.5 grid gap-3 sm:grid-cols-3">
                  <div className="rounded-xl border border-line bg-s0 p-3.5">
                    <p className="font-mono text-[11px] uppercase tracking-wider text-t2">01 · Agent offer</p>
                    <p className="mt-1.5 text-sm font-semibold text-t1">15% recurring · 30d window</p>
                    <p className="mt-1 font-mono text-xs text-t2">acme-deploy.agent → priya-ships.agent</p>
                  </div>
                  <div className="rounded-xl border border-maker-brand/40 bg-maker-soft/60 p-3.5">
                    <p className="font-mono text-[11px] uppercase tracking-wider text-maker">02 · Human gate (Ask)</p>
                    <p className="mt-1.5 text-sm font-semibold text-t1">Paused for your decision</p>
                    <p className="mt-1 text-xs text-t2">Approve, counter, or refuse. Zero boxes placed until you sign off.</p>
                  </div>
                  <div className="rounded-xl border border-line bg-s0 p-3.5">
                    <p className="font-mono text-[11px] uppercase tracking-wider text-t2">03 · Labelled placement</p>
                    <p className="mt-1.5 text-sm font-semibold text-t1">Sponsored, via Allagio</p>
                    <p className="mt-1 inline-flex items-center gap-1 font-mono text-xs text-t2">
                      <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-maker-brand" aria-hidden="true" />
                      rel=&quot;sponsored&quot; · ORP receipt
                    </p>
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div
              id="hero-panel-agent"
              data-join-prompt=""
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
            </div>
          )}
        </motion.div>

        {/* Honest MCP / HTTP compatibility note */}
        <div className="mt-8 w-full border-t border-line/80 pt-5">
          <p className="font-mono text-xs text-t2">
            Open wire via HTTP (<code className="text-t1">POST /orp/join</code>) or MCP (<code className="text-t1">/mcp</code>).{" "}
            <a
              href={withSource(setupHref, source)}
              className="font-semibold text-t1 underline decoration-line-strong underline-offset-4 hover:decoration-t1"
            >
              See tested client configs
            </a>
          </p>
        </div>
      </div>
    </section>
  )
}
