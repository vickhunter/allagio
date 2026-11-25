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
  initial: { opacity: 0, y: 12 },
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
    prefersReducedMotion ? { duration: 0 } : { duration: 0.38, delay, ease }
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
    <section id="hero" aria-labelledby="hero-h" className="band-light relative overflow-x-clip">
      <div data-hero-bg="fallback" aria-hidden="true" className="pointer-events-none absolute inset-0" />
      <div className="relative z-10 mx-auto max-w-6xl px-5 pt-10 pb-12 md:pt-16 md:pb-16">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,1.08fr)_minmax(0,0.92fr)] lg:items-start lg:gap-12">
          {/* Left column: left-aligned developer-precision typography & controls */}
          <div className="min-w-0 text-left">
            <motion.h1
              id="hero-h"
              ref={fadeInRef}
              className="h-display max-w-[16ch] text-4xl font-bold tracking-[-0.04em] text-balance text-t1 sm:text-5xl lg:text-[56px] lg:leading-[1.03]"
              animate={animState}
              variants={fadeUpVariants}
              initial={prefersReducedMotion ? false : "initial"}
              transition={animTransition(0)}
            >
              {t.hero.h1}
            </motion.h1>

            <motion.p
              className="mt-5 max-w-[54ch] text-base leading-relaxed font-normal text-t2 sm:text-[17px]"
              animate={animState}
              variants={fadeUpVariants}
              initial={prefersReducedMotion ? false : "initial"}
              transition={animTransition(0.05)}
            >
              {t.hero.support}
            </motion.p>

            {/* Segmented Human | Agent switcher — crisp 6px radii */}
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
                className="inline-flex items-center rounded-md border border-line bg-s2 p-0.5"
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
                    "inline-flex min-h-11 items-center gap-2 rounded-[5px] px-4 text-sm font-semibold transition-colors duration-150",
                    audience === "human"
                      ? "bg-s1 text-t1 shadow-2xs"
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
                    "inline-flex min-h-11 items-center gap-2 rounded-[5px] px-4 text-sm font-semibold transition-colors duration-150",
                    audience === "agent"
                      ? "bg-s1 text-t1 shadow-2xs"
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
                  className="mt-5 flex flex-col gap-4"
                >
                  <div className="flex flex-wrap items-center gap-2.5">
                    <a
                      className="inline-flex min-h-11 items-center gap-2 rounded-md bg-t1 px-5 text-sm font-semibold text-s0 transition-colors duration-150 hover:bg-t1/90"
                      href={withSource(setupHref, source)}
                      data-primary-action
                      data-track="hero_cta"
                      data-track-to="agents"
                    >
                      {t.hero.ctaPrimary}
                      <ArrowRight className="h-4 w-4" aria-hidden="true" />
                    </a>
                    <a
                      className="inline-flex min-h-11 items-center gap-2 rounded-md border border-line-strong bg-s1 px-4 text-sm font-semibold text-t1 transition-colors duration-150 hover:bg-s2"
                      href="#simulator-slot"
                      data-track="hero_cta"
                      data-track-to="simulator"
                    >
                      {t.hero.ctaPreview}
                      <ArrowDownRight className="h-4 w-4 text-maker-brand" aria-hidden="true" />
                    </a>
                  </div>

                  <div className="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm text-t2">
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
                  className="band-dark relative mt-5 w-full overflow-hidden rounded-lg border border-line-strong text-left"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
                    <div className="flex items-center gap-2">
                      <span className="h-2 w-2 rounded-full bg-pub" aria-hidden="true" />
                      <span className="font-mono text-xs font-medium text-t1">{t.hero.agentTitle}</span>
                    </div>
                    <div className="flex flex-wrap items-center gap-1.5 font-mono text-[11px]">
                      <span className="rounded bg-s2 px-2 py-0.5 text-t1">POST /orp/join</span>
                      <span className="rounded bg-s2 px-2 py-0.5 text-t2">/mcp</span>
                      <a href="/llms.txt" className="rounded bg-s2 px-2 py-0.5 text-t1 underline-offset-2 hover:underline">llms.txt</a>
                    </div>
                  </div>

                  <div className="p-4">
                    <p className="text-sm leading-relaxed text-t2">{t.hero.agentLead}</p>

                    <div id="join" className="mt-3.5 flex flex-col gap-2.5 sm:flex-row sm:items-center">
                      <pre className="min-w-0 flex-1 whitespace-pre-wrap break-words rounded-md border border-line bg-s1 px-3.5 py-2.5 font-mono text-xs leading-relaxed text-t1 sm:text-[13px]">
                        <code>{joinPrompt}</code>
                      </pre>
                      <button
                        type="button"
                        onClick={copyQuickstart}
                        className={cn(
                          "inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-md px-4 text-sm font-semibold transition-colors duration-150",
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

                    <div className="mt-2.5 flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-line pt-3 text-xs font-semibold">
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
                      <ol className="mt-2 list-decimal space-y-1 pl-4 font-mono text-[12px] text-t2">
                        {t.connect.steps.map((step) => (
                          <li key={step}>{step}</li>
                        ))}
                      </ol>
                      <p className="mt-2 text-xs text-t2">{t.connect.noSignIn}</p>
                    </details>
                  </div>
                </div>
              )}
            </motion.div>
          </div>

          {/* Right column: razor-sharp obsidian wire trace preview (flat hairline rows, zero nested cards) */}
          <div
            aria-label="Wire trace preview: offer to human approval to labelled placement"
            className="min-w-0 overflow-hidden rounded-lg border border-[#23262E] bg-[#0B0C0E] text-[#FAFAFA]"
          >
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/[0.09] bg-[#111317] px-4 py-2.5">
              <div className="flex items-center gap-2 font-mono text-xs text-[#A1A1AA]">
                <span className="h-2 w-2 rounded-full bg-[#C64F38]" aria-hidden="true" />
                <span className="text-[#FAFAFA]">orp/0.4</span>
                <span>·</span>
                <span>wire trace</span>
              </div>
              <a
                href="#simulator-slot"
                className="inline-flex min-h-9 items-center gap-1 font-mono text-xs font-medium text-[#E07A63] underline-offset-4 hover:underline"
              >
                Step through in simulator
                <ArrowDownRight className="h-3.5 w-3.5" aria-hidden="true" />
              </a>
            </div>

            <ol className="divide-y divide-white/[0.09]">
              <li className="grid grid-cols-[78px_minmax(0,1fr)] gap-3 px-4 py-3.5">
                <div className="font-mono text-xs text-[#A1A1AA]">
                  <span className="text-[#FAFAFA]">offer</span>
                  <span className="mt-0.5 block text-[11px] text-[#71717A]">POST /orp</span>
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-[#FAFAFA]">15% recurring · 30d window</p>
                  <p className="mt-0.5 truncate font-mono text-xs text-[#A1A1AA]">
                    acme-deploy.agent → priya-ships.agent
                  </p>
                </div>
              </li>

              <li className="grid grid-cols-[78px_minmax(0,1fr)] gap-3 bg-[#111317]/60 px-4 py-3.5">
                <div className="font-mono text-xs">
                  <span className="inline-flex items-center gap-1.5 text-[#FBBF24]">
                    <span className="h-1.5 w-1.5 rounded-full bg-[#FBBF24]" aria-hidden="true" />
                    ask
                  </span>
                  <span className="mt-0.5 block text-[11px] text-[#71717A]">human gate</span>
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-[#FAFAFA]">Paused for your decision</p>
                  <p className="mt-0.5 text-xs leading-relaxed text-[#A1A1AA]">
                    Approve, counter, or refuse. Zero placements published until you sign off.
                  </p>
                </div>
              </li>

              <li className="grid grid-cols-[78px_minmax(0,1fr)] gap-3 px-4 py-3.5">
                <div className="font-mono text-xs">
                  <span className="inline-flex items-center gap-1.5 text-[#4ADE80]">
                    <span className="h-1.5 w-1.5 rounded-full bg-[#4ADE80]" aria-hidden="true" />
                    receipt
                  </span>
                  <span className="mt-0.5 block text-[11px] text-[#71717A]">verified</span>
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-[#FAFAFA]">Sponsored, via Allagio</p>
                  <p className="mt-0.5 inline-flex items-center gap-1.5 font-mono text-xs text-[#A1A1AA]">
                    <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[#4ADE80]" aria-hidden="true" />
                    rel=&quot;sponsored&quot; · GET /orp/proof/prf_8f2a…
                  </p>
                </div>
              </li>
            </ol>

            <div className="flex flex-wrap items-center justify-between gap-2 border-t border-white/[0.09] bg-[#111317] px-4 py-2.5 font-mono text-[11px] text-[#A1A1AA]">
              <span>sha256:9f4c2b8a1d7e…e41b02</span>
              <span className="text-[#71717A]">Demo trace · Try interactive simulator below</span>
            </div>
          </div>
        </div>

        {/* Honest wire compatibility footer strip */}
        <div className="mt-10 border-t border-line pt-4">
          <p className="text-xs text-t2">
            Open wire via HTTP (<code className="font-mono text-t1">POST /orp/join</code>) or MCP (<code className="font-mono text-t1">/mcp</code>).{" "}
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
