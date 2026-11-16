import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import {
  AtSign,
  Camera,
  Check,
  FileText,
  Handshake,
  Mail,
  Mic,
  Music2,
  Pause,
  Pin,
  Play,
  Radio,
  SquarePlay,
} from 'lucide-react'

import { cn } from '@/lib/utils'
import { BorderBeam } from '@/components/ui/border-beam'
import { PlatformPostFrame } from '@/components/platform-frames'
import { useLocale } from '@/i18n'
import type { ShowcaseId, ShowcasePlatform } from '@/i18n/messages'

const tabIcons: Record<ShowcaseId, typeof AtSign> = {
  x: AtSign,
  pinterest: Pin,
  instagram: Camera,
  tiktok: Music2,
  youtube: SquarePlay,
  newsletter: Mail,
  blog: FileText,
  podcast: Mic,
}

function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(
    () => typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const handleChange = () => setReduced(query.matches)
    query.addEventListener('change', handleChange)
    return () => query.removeEventListener('change', handleChange)
  }, [])
  return reduced
}

function DealPanel({ platform }: { platform: ShowcasePlatform }) {
  const { t } = useLocale()
  const reduced = usePrefersReducedMotion()
  const steps = [platform.offerStep, platform.counterStep, t.showcase.agreedStep, t.showcase.approvedStep, t.showcase.liveStep]
  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-s1 p-5">
      <div className="grid gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-wider text-t2">{t.showcase.partnershipLabel}</p>
          <p className="mt-0.5 text-sm font-semibold text-t1">{platform.partnership}</p>
        </div>
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-wider text-t2">{t.showcase.termsLabel}</p>
          <p className="mt-0.5 text-sm font-semibold text-t1">{platform.terms}</p>
        </div>
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-wider text-t2">{t.showcase.partiesLabel}</p>
          <ul className="mt-0.5 flex flex-wrap gap-1.5">
            {platform.parties.map((party) => (
              <li key={party} className="rounded-full border border-line bg-s0 px-2.5 py-0.5 text-xs font-medium text-t1">{party}</li>
            ))}
          </ul>
        </div>
      </div>
      <div className="mt-4 border-t border-line pt-4">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-t2">{t.showcase.timelineLabel}</p>
        <ol className="mt-3 grid gap-0">
          {steps.map((step, i) => {
            const last = i === steps.length - 1
            const terminal = i >= steps.length - 3
            return (
              <motion.li
                key={step}
                initial={reduced ? false : { opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.3, delay: reduced ? 0 : i * 0.55 }}
                className="relative flex gap-3 pb-4 last:pb-0"
              >
                <span className="relative flex flex-col items-center" aria-hidden="true">
                  <span className={cn(
                    'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[10px]',
                    last ? 'border-pub bg-pub text-s0' : terminal ? 'border-pub/60 bg-pub-soft text-pub' : 'border-line bg-s0 text-t2'
                  )}>
                    {terminal ? <Check className="h-3 w-3" /> : i + 1}
                  </span>
                  {!last && <span className="mt-0.5 w-px flex-1 bg-line" />}
                </span>
                <span className={cn('pt-0.5 text-[13px] leading-snug', last ? 'font-semibold text-t1' : 'text-t2')}>
                  {i === steps.length - 1 ? <span className="inline-flex items-center gap-1.5"><Radio className="h-3.5 w-3.5 text-pub" aria-hidden="true" />{step}</span> : step}
                </span>
              </motion.li>
            )
          })}
        </ol>
      </div>
    </div>
  )
}

export function PlatformShowcase() {
  const { t } = useLocale()
  const reduced = usePrefersReducedMotion()
  const platforms = t.showcase.platforms
  const [active, setActive] = useState(0)
  const [paused, setPaused] = useState(false)
  const [manualPaused, setManualPaused] = useState(false)
  const platform = platforms[active]
  const panelRef = useRef<HTMLDivElement>(null)
  const [panelMinH, setPanelMinH] = useState(0)

  useEffect(() => {
    if (reduced || paused || manualPaused) return
    const timer = window.setInterval(() => setActive((i) => (i + 1) % platforms.length), 5000)
    return () => window.clearInterval(timer)
  }, [reduced, paused, manualPaused, platforms.length])

  // Reserve the tallest panel height seen so tab switches don't move the page.
  useEffect(() => {
    const el = panelRef.current
    if (!el) return
    const observer = new ResizeObserver(() => {
      setPanelMinH((height) => Math.max(height, el.offsetHeight))
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const last = platforms.length - 1
    let next: number | null = null
    if (event.key === 'ArrowRight') next = active === last ? 0 : active + 1
    else if (event.key === 'ArrowLeft') next = active === 0 ? last : active - 1
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = last
    if (next === null) return
    event.preventDefault()
    setActive(next)
    document.getElementById(`showcase-tab-${platforms[next].id}`)?.focus()
  }

  return (
    <div onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocus={() => setPaused(true)} onBlur={() => setPaused(false)}>
      <div
        role="tablist"
        aria-label={t.showcase.tabsLabel}
        onKeyDown={handleKeyDown}
        className="flex items-center gap-2 overflow-x-auto pb-1 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {platforms.map((item, i) => {
          const Icon = tabIcons[item.id]
          const selected = i === active
          return (
            <button
              key={item.id}
              id={`showcase-tab-${item.id}`}
              role="tab"
              aria-selected={selected}
              aria-controls="showcase-panel"
              tabIndex={selected ? 0 : -1}
              onClick={() => setActive(i)}
              className={cn(
                'inline-flex min-h-10 shrink-0 items-center gap-1.5 rounded-full border px-3.5 text-[13px] font-semibold transition-colors',
                selected ? 'border-t1 bg-t1 text-s0' : 'border-line bg-s1 text-t2 hover:text-t1'
              )}
            >
              <Icon className="h-3.5 w-3.5" aria-hidden="true" />
              {item.name}
            </button>
          )
        })}
        <button
          type="button"
          aria-pressed={manualPaused}
          aria-label={manualPaused ? t.showcase.resumeLabel : t.showcase.pauseLabel}
          title={manualPaused ? t.showcase.resumeLabel : t.showcase.pauseLabel}
          onClick={() => setManualPaused((value) => !value)}
          className="inline-flex min-h-10 min-w-10 shrink-0 items-center justify-center rounded-full border border-line bg-s1 text-t2 transition-colors hover:text-t1"
        >
          {manualPaused ? <Play className="h-3.5 w-3.5" aria-hidden="true" /> : <Pause className="h-3.5 w-3.5" aria-hidden="true" />}
        </button>
      </div>
      <div id="showcase-panel" role="tabpanel" aria-labelledby={`showcase-tab-${platform.id}`} className="mt-5" style={panelMinH ? { minHeight: panelMinH } : undefined} ref={panelRef}>
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={platform.id}
            initial={reduced ? false : { opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={reduced ? undefined : { opacity: 0, y: -6 }}
            transition={{ duration: 0.3 }}
            className="grid gap-5 lg:grid-cols-[minmax(0,1.1fr)_minmax(280px,0.9fr)]"
          >
            <div className="relative flex flex-col justify-center overflow-hidden rounded-2xl border border-line bg-s1 p-4 shadow-sm [background-image:radial-gradient(circle,rgb(var(--rgb-line))_1px,transparent_1px)] [background-size:18px_18px] sm:p-6">
              <div className="w-full rounded-2xl">
                <PlatformPostFrame platform={platform.id} mock={platform.mock} tag={t.showcase.exampleTag} />
              </div>
              <BorderBeam size={140} duration={8} borderWidth={1.5} />
            </div>
            <DealPanel platform={platform} />
          </motion.div>
        </AnimatePresence>
      </div>
      <p className="mt-5 flex items-start gap-2 text-[13px] leading-relaxed text-t2">
        <Handshake className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        {t.showcase.footnote}
      </p>
    </div>
  )
}
