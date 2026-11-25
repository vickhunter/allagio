import {
  ArrowRight,
  ArrowUpRight,
  BadgePercent,
  Check,
  Code2,
  EyeOff,
  FileSignature,
  Hand,
  Megaphone,
  PauseCircle,
  Radio,
  Repeat2,
  ShieldCheck,
  Users,
  X,
  Zap,
} from 'lucide-react'
import { Page } from '@/site/Page'
import { Hero } from '@/components/hero-2'
import { DealSimulator } from '@/components/DealSimulator'
import { NetworkWall } from '@/components/NetworkWall'
import { PostPreview } from '@/components/PostPreview'
import { FAQ } from '@/components/faq-1'
import { CallToAction } from '@/components/call-to-action-1'
import { useOfferConfig, useSource, withSource } from '@/lib/source'
import { useLocale } from '@/i18n'
import { cn } from '@/lib/utils'

const templateIcons: Record<string, typeof BadgePercent> = {
  affiliate: BadgePercent,
  sponsored: Megaphone,
  swap: Repeat2,
  revshare: Users,
  slot: Radio,
  custom: Code2,
}

const modeIcons = {
  off: PauseCircle,
  ask: Hand,
  auto: Zap,
} as const

const checkIcons = [ShieldCheck, FileSignature, EyeOff, Check]

export function App() {
  const source = useSource()
  const { plugin } = useOfferConfig()
  const { t } = useLocale()

  return (
    <Page page="home">
      {/* Band 1: Asymmetric Developer-Precision Hero */}
      <Hero source={source} />

      {/* Band 2: Obsidian Contrast Band — Interactive ORP Deal Simulator */}
      <section
        id="showcase"
        aria-labelledby="showcase-h"
        className="band-dark relative scroll-mt-20 border-t border-line"
      >
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <div className="max-w-2xl">
            <h2
              id="showcase-h"
              className="h-display text-3xl font-bold tracking-tighter text-balance text-t1 sm:text-4xl"
            >
              {t.showcaseBand.heading}
            </h2>
            <p className="mt-3 text-base leading-relaxed text-t2">
              {t.showcaseBand.lead}
            </p>
          </div>
          <div id="simulator-slot" className="mt-8 scroll-mt-24">
            <DealSimulator />
          </div>
        </div>
      </section>

      {/* Band 3: Light Architectural Band — Deal Templates (1px hairline grid) */}
      <section id="templates" aria-labelledby="templates-h" className="band-light border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <div className="flex flex-col justify-between gap-6 md:flex-row md:items-end">
            <div className="max-w-2xl">
              <h2
                id="templates-h"
                className="h-display text-3xl font-bold tracking-tighter text-balance text-t1 sm:text-4xl"
              >
                {t.templates.heading}
              </h2>
              <p className="mt-3 text-base leading-relaxed text-t2">
                {t.templates.lead}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-4 text-sm font-semibold">
              <a
                href={withSource('/use-cases/', source)}
                data-track="templates_all"
                className="inline-flex min-h-11 items-center gap-1.5 text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
              >
                {t.templates.allLink}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </a>
              <a
                href={withSource('/protocol/', source)}
                data-track="templates_protocol"
                className="inline-flex min-h-11 items-center gap-1.5 text-t2 underline decoration-line underline-offset-4 transition-colors hover:text-t1 hover:decoration-t1"
              >
                {t.templates.protocolLink}
              </a>
            </div>
          </div>

          <div className="mt-10 grid overflow-hidden rounded-lg border-t border-l border-line sm:grid-cols-2 lg:grid-cols-3">
            {t.templates.items.map((item) => {
              const Icon = templateIcons[item.id] ?? Code2
              return (
                <a
                  key={item.id}
                  href={withSource(item.href, source)}
                  data-track="template_card"
                  data-track-to={item.id}
                  className="group relative flex flex-col justify-between border-r border-b border-line bg-s1 p-6 text-left transition-colors duration-150 hover:bg-s0"
                >
                  <div>
                    <div className="flex items-center justify-between gap-3">
                      <Icon className="h-4 w-4 text-maker-brand" aria-hidden="true" />
                      <span className="font-mono text-[11px] font-medium text-t2">
                        {item.tag}
                      </span>
                    </div>
                    <h3 className="mt-4 font-display text-lg font-bold tracking-tight text-t1">
                      {item.name}
                    </h3>
                    <p className="mt-2 text-sm leading-relaxed text-t2">
                      {item.body}
                    </p>
                  </div>
                  <div className="mt-5 flex items-center justify-between border-t border-line pt-3">
                    <span className="font-mono text-xs font-medium text-t1">{item.terms}</span>
                    <ArrowUpRight
                      className="h-4 w-4 text-t2 transition-transform duration-150 group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-t1"
                      aria-hidden="true"
                    />
                  </div>
                </a>
              )
            })}
          </div>
        </div>
      </section>

      {/* Band 4: Obsidian Contrast Band — Live Proof (NetworkWall + PostPreview) */}
      <div id="live-proof" className="band-dark relative border-t border-line">
        <div className="relative">
          <NetworkWall />
          <section id="post-preview" aria-labelledby="preview-h" className="mx-auto max-w-6xl scroll-mt-24 px-5 pb-16 pt-2 md:pb-20">
            <div className="flex items-center gap-4">
              <span className="h-px flex-1 bg-line" aria-hidden="true" />
              <span id="preview-h" className="text-xs font-semibold text-t2">
                {t.previewSection.tryLabel}
              </span>
              <span className="h-px flex-1 bg-line" aria-hidden="true" />
            </div>
            <div className="mx-auto mt-8 max-w-3xl">
              <PostPreview />
            </div>
          </section>
        </div>
      </div>

      {/* Band 5: Light Band — Human Approval & Safety */}
      <section id="limits" aria-labelledby="limits-h" className="band-light border-t border-line">
        <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
          <div className="max-w-2xl">
            <h2
              id="limits-h"
              className="h-display text-3xl font-bold tracking-tighter text-balance text-t1 sm:text-4xl"
            >
              {t.safety.heading}
            </h2>
            <p className="mt-3 text-base leading-relaxed text-t2">
              {t.safety.lead}
            </p>
          </div>

          {/* 3 Control Modes: Off / Ask / Auto — flat hairline-divided strip */}
          <div
            aria-label="Approval modes: Off, Ask, Auto"
            className="mt-8 grid overflow-hidden rounded-lg border border-line bg-s1 divide-y divide-line md:grid-cols-3 md:divide-x md:divide-y-0"
          >
            {t.safety.modes.map((mode) => {
              const Icon = modeIcons[mode.id]
              const isAsk = mode.id === 'ask'
              return (
                <div
                  key={mode.id}
                  className={cn(
                    'flex flex-col p-6',
                    isAsk ? 'bg-s0' : 'bg-s1'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="inline-flex items-center gap-2 font-display text-lg font-bold tracking-tight text-t1">
                      <Icon className={cn('h-4 w-4', isAsk ? 'text-maker-brand' : 'text-t2')} aria-hidden="true" />
                      {mode.name}
                    </span>
                    <span
                      className={cn(
                        'rounded-md border px-2 py-0.5 text-[11px] font-medium',
                        isAsk
                          ? 'border-maker-brand/30 bg-maker-soft text-maker'
                          : 'border-line bg-s2 text-t2'
                      )}
                    >
                      {mode.badge}
                    </span>
                  </div>
                  <p className="mt-2.5 text-sm leading-relaxed text-t2">
                    {mode.body}
                  </p>
                </div>
              )
            })}
          </div>

          {/* Protocol Hard Checks & Safeguards — clean ruled checklist */}
          <div className="mt-12">
            <h3 className="text-sm font-semibold text-t1">
              {t.safety.checksHeading}
            </h3>
            <dl className="mt-3 grid gap-x-10 border-t border-line sm:grid-cols-2">
              {t.safety.checks.map((item, i) => {
                const Icon = checkIcons[i] ?? ShieldCheck
                return (
                  <div key={item.title} className="flex gap-3.5 border-b border-line py-5">
                    <Icon className="mt-0.5 h-4 w-4 shrink-0 text-maker-brand" aria-hidden="true" />
                    <div>
                      <dt className="font-display text-base font-bold tracking-tight text-t1">
                        {item.title}
                      </dt>
                      <dd className="mt-1 text-sm leading-relaxed text-t2">
                        {item.body}
                      </dd>
                    </div>
                  </div>
                )
              })}
            </dl>
          </div>

          {/* Hairline-divided With ORP vs By Hand comparison */}
          <div className="mt-10 grid overflow-hidden rounded-lg border border-line bg-s1 divide-y divide-line md:grid-cols-2 md:divide-x md:divide-y-0">
            <article className="p-6">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="font-display text-base font-bold text-t1">{t.tradeoffs.withTitle}</h3>
                <span className="rounded-md border border-pub/25 bg-pub-soft px-2 py-0.5 text-[11px] font-medium text-pub">
                  {t.tradeoffs.chip}
                </span>
              </div>
              <ul className="mt-4 space-y-2.5">
                {t.tradeoffs.withItems.slice(0, 3).map((item) => (
                  <li key={item} className="flex gap-2.5 text-sm leading-relaxed text-t1">
                    <Check className="mt-0.5 h-4 w-4 shrink-0 text-pub" aria-hidden="true" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </article>
            <article className="bg-s0/70 p-6">
              <h3 className="font-display text-base font-bold text-t1">{t.tradeoffs.withoutTitle}</h3>
              <ul className="mt-4 space-y-2.5">
                {t.tradeoffs.withoutItems.slice(0, 3).map((item) => (
                  <li key={item} className="flex gap-2.5 text-sm leading-relaxed text-t2">
                    <X className="mt-0.5 h-4 w-4 shrink-0 text-maker" aria-hidden="true" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </article>
          </div>

          <p className="mt-8">
            <a
              href={withSource('/you/', source)}
              data-track="safety_limits"
              className="inline-flex min-h-11 items-center gap-1.5 text-sm font-semibold text-t1 underline decoration-line-strong underline-offset-4 transition-colors hover:decoration-t1"
            >
              {t.safety.limitsLink}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </a>
          </p>
        </div>
      </section>

      {/* Band 6: Neutral Surface — FAQ + Final CTA */}
      <div className="band-warm border-t border-line">
        <FAQ />
      </div>

      <div className="band-light border-t border-line">
        <CallToAction source={source} />
      </div>

      {plugin ? (
        <div className="band-light border-t border-line">
          <div className="mx-auto max-w-3xl px-5 py-10 text-center">
            <a
              className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline"
              href="/downloads/offer-rails.zip"
              data-track="zip_download"
            >
              {t.plugin.download}
            </a>
          </div>
        </div>
      ) : null}
    </Page>
  )
}
