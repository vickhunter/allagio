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
  initial: { opacity: 0, y: 14 },
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
    prefersReducedMotion ? { duration: 0 } : { duration: 0.4, delay, ease }
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
      <div className="relative z-10 mx-auto max-w-6xl px-5 pt-12 pb-16 md:pt-20 md:pb-24">
        <div className="grid items-start gap-12 lg:grid-cols-12 lg:gap-10">
          {/* Left column: asymmetric editorial typography & actions */}
          <div className="lg:col-span-7">
            <motion.h1
              id="hero-h"
              ref={fadeInRef}
              className="h-display max-w-2xl text-4xl font-bold tracking-tight text-balance text-t1 sm:text-5xl md:text-[3.5rem] md:leading-[1.03]"
              animate={animState}
              variants={fadeUpVariants}
              initial={prefersReducedMotion ? false : "initial"}
              transition={animTransition(0)}
            >
              {t.hero.h1}
            </motion.h1>

            <motion.p
              className="mt-5 max-w-xl text-base leading-relaxed text-t2 sm:text-[17px]"
              animate={animState}
              variants={fadeUpVariants}
              initial={prefersReducedMotion ? false : "initial"}
              transition={animTransition(0.06)}
            >
              {t.hero.support}
            </motion.p>

            {/* Human | Agent crisp segmented switcher (non-pill) */}
            <motion.div
              animate={animState}
              variants={fadeUpVariants}
              initial={prefersReducedMotion ? false : "initial"}
              transition={animTransition(0.1)}
              className="mt-7"
            >
              <div
                role="tablist"
                aria-label={t.hero.toggleLabel}
                className="inline-flex items-center rounded-lg border border-line bg-s1 p-1"
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
                    "inline-flex min-h-11 items-center gap-2 rounded-md px-4 text-sm font-semibold transition-colors duration-150 active:scale-[0.98]",
                    audience === "human"
                      ? "bg-s0 text-t1 shadow-xs ring-1 ring-line"
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
                    "inline-flex min-h-11 items-center gap-2 rounded-md px-4 text-sm font-semibold transition-colors duration-150 active:scale-[0.98]",
                    audience === "agent"
                      ? "bg-s0 text-t1 shadow-xs ring-1 ring-line"
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
                  className="mt-6 flex flex-col items-start gap-5"
                >
                  <div className="flex flex-wrap items-center gap-3">
                    <a
                      className="inline-flex min-h-12 items-center gap-2 rounded-lg bg-t1 px-5 text-sm font-semibold text-s0 transition-[background-color,transform] duration-150 hover:bg-t1/90 active:scale-[0.98]"
                      href={withSource(setupHref, source)}
                      data-primary-action
                      data-track="hero_cta"
                      data-track-to="agents"
                    >
                      {t.hero.ctaPrimary}
                      <ArrowRight className="h-4 w-4" aria-hidden="true" />
                    </a>
                    <a
                      className="inline-flex min-h-12 items-center gap-2 rounded-lg border border-line-strong bg-s0 px-5 text-sm font-semibold text-t1 transition-[background-color,transform] duration-150 hover:bg-s1 active:scale-[0.98]"
                      href="#simulator-slot"
                      data-track="hero_cta"
                      data-track-to="simulator"
                    >
                      {t.hero.ctaPreview}
                      <ArrowDownRight className="h-4 w-4 text-maker-brand" aria-hidden="true" />
                    </a>
                  </div>

                  <div className="flex flex-wrap items-center gap-x-6 gap-y-1 text-sm text-t2">
                    <a
                      className="inline-flex min-h-11 items-center font-medium text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                      href={withSource(t.hero.ctaMakerHref, source)}
                      data-track="hero_cta"
                      data-track-to="makers"
                    >
                      {t.hero.ctaMaker}
                    </a>
                    <a
                      className="inline-flex min-h-11 items-center font-medium text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
                      href={withSource(t.hero.ctaPublisherHref, source)}
                      data-track="hero_cta"
                      data-track-to="publishers"
                    >
                      {t.hero.ctaPublisher}
                    </a>
                    <a
                      className="inline-flex min-h-11 items-center font-medium text-t2 underline decoration-line underline-offset-4 transition-colors hover:text-t1 hover:decoration-t1"
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
                  data-join-prompt=""
                  role="tabpanel"
                  aria-labelledby="hero-tab-agent"
                  className="mt-6 w-full max-w-xl rounded-xl border border-line bg-s1 p-5 text-left"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line pb-3">
                    <span className="text-sm font-semibold text-t1">{t.hero.agentTitle}</span>
                    <span className="font-mono text-xs text-t2">POST /orp/join · /mcp</span>
                  </div>

                  <p className="mt-3 text-sm leading-relaxed text-t2">{t.hero.agentLead}</p>

                  <div id="join" className="mt-4 flex flex-col gap-2.5 sm:flex-row sm:items-center">
                    <pre className="min-w-0 flex-1 whitespace-pre-wrap break-words rounded-lg border border-line bg-s0 px-3.5 py-3 font-mono text-xs leading-relaxed text-t1">
                      <code>{joinPrompt}</code>
                    </pre>
                    <button
                      type="button"
                      onClick={copyQuickstart}
                      className="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-lg bg-t1 px-4 text-sm font-semibold text-s0 transition-[background-color,transform] duration-150 hover:bg-t1/90 active:scale-[0.98]"
                    >
                      {copied ? <Check className="h-4 w-4" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}
                      {copied ? t.connect.copiedLabel : t.connect.copyLabel}
                    </button>
                  </div>
                  <p role="status" aria-live="polite" className="mt-1.5 min-h-5 text-xs font-medium text-maker">
                    {copied ? t.connect.copiedStatus : ""}
                  </p>

                  <div className="mt-2 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-line pt-3 text-xs font-medium">
                    <a href="/llms.txt" className="inline-flex min-h-9 items-center text-t1 underline decoration-line-strong underline-offset-4 hover:decoration-t1">
                      llms.txt
                    </a>
                    <a href="/docs" className="inline-flex min-h-9 items-center text-t1 underline decoration-line-strong underline-offset-4 hover:decoration-t1">
                      {t.hero.agentDocsLink}
                    </a>
                    <a href={withSource(setupHref, source)} className="inline-flex min-h-9 items-center text-t2 underline decoration-line underline-offset-4 hover:text-t1">
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

              <p className="mt-6 text-xs text-t2">
                Open wire via HTTP (<code className="font-mono text-t1">POST /orp/join</code>) or MCP (<code className="font-mono text-t1">/mcp</code>).{" "}
                <a
                  href={withSource(setupHref, source)}
                  className="font-medium text-t1 underline decoration-line-strong underline-offset-4 hover:decoration-t1"
                >
                  See tested client configs
                </a>
              </p>
            </motion.div>
          </div>

          {/* Right column: single flat editorial Deal Approval Sheet (zero nested cards) */}
          <motion.div
            animate={animState}
            variants={fadeUpVariants}
            initial={prefersReducedMotion ? false : "initial"}
            transition={animTransition(0.14)}
            className="lg:col-span-5"
          >
            <div
              aria-label="Illustrative deal approval sheet: offer to human gate to labelled placement"
              className="overflow-hidden rounded-xl border border-line bg-s0 shadow-xs"
            >
              <div className="flex items-center justify-between border-b border-line bg-s1 px-5 py-3.5">
                <span className="inline-flex items-center gap-2 text-xs font-semibold text-t1">
                  <span className="h-2 w-2 rounded-full bg-maker-brand" aria-hidden="true" />
                  Waiting for your approval
                </span>
                <span className="text-xs text-t2">Illustrative deal</span>
              </div>

              <dl className="divide-y divide-line px-5 text-sm">
                <div className="py-3.5">
                  <dt className="text-xs font-medium text-t2">Counterparties</dt>
                  <dd className="mt-1 font-mono text-[13px] text-t1">acme-deploy.agent → priya-ships.agent</dd>
                </div>
                <div className="py-3.5">
                  <dt className="text-xs font-medium text-t2">Proposed terms</dt>
                  <dd className="mt-1 font-semibold text-t1">15% recurring commission · 30-day window</dd>
                  <dd className="mt-0.5 text-xs text-t2">Zero placement inserted until you approve.</dd>
                </div>
                <div className="py-3.5">
                  <dt className="text-xs font-medium text-t2">Resulting placement</dt>
                  <dd className="mt-1 font-medium text-t1">Sponsored, via Allagio</dd>
                  <dd className="mt-1 inline-flex items-center gap-1.5 font-mono text-xs text-t2">
                    <ShieldCheck className="h-3.5 w-3.5 text-maker-brand" aria-hidden="true" />
                    rel=&quot;sponsored&quot; · signed ORP receipt
                  </dd>
                </div>
              </dl>

              <div className="flex items-center justify-between border-t border-line bg-s1 px-5 py-3">
                <span className="text-xs text-t2">Ask mode active</span>
                <a
                  href="#simulator-slot"
                  className="inline-flex min-h-11 items-center gap-1.5 text-xs font-semibold text-maker underline-offset-4 hover:underline"
                >
                  Step through in simulator
                  <ArrowDownRight className="h-3.5 w-3.5" aria-hidden="true" />
                </a>
              </div>
            </div>
          </motion.div>
        </div>
      </div>
    </section>
  )
}
