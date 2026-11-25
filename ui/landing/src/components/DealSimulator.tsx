import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import {
  ArrowLeftRight,
  Ban,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Code2,
  Copy,
  FileCheck2,
  Hash,
  Play,
  RotateCcw,
  ShieldCheck,
  UserCheck,
  Zap,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  DEAL_TEMPLATES,
  advanceStep,
  applyHumanDecision,
  buildOrpEnvelope,
  createInitialState,
  getActiveArtifact,
  getActiveTerms,
  getTemplateById,
  resetSimulation,
  selectTemplate,
  shortHash,
  toggleJsonDrawer,
  type DealTemplateId,
  type HumanDecision,
  type SimulatorStage,
  type SimulatorState,
} from '@/components/deal-simulator-data'

const STEP_DELAY_MS = 650

function stageReached(state: SimulatorState, target: SimulatorStage): boolean {
  return state.stage === target || state.history.includes(target)
}

export interface DealSimulatorProps {
  initialTemplate?: DealTemplateId
  className?: string
}

export function DealSimulator({ initialTemplate = 'affiliate', className }: DealSimulatorProps) {
  const reduced = Boolean(useReducedMotion())
  const [state, setState] = useState<SimulatorState>(() => createInitialState(initialTemplate))
  const [copiedJson, setCopiedJson] = useState(false)
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({})

  function onTemplateTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, currentIndex: number) {
    let nextIndex: number | null = null
    if (event.key === 'ArrowRight') nextIndex = (currentIndex + 1) % DEAL_TEMPLATES.length
    else if (event.key === 'ArrowLeft') nextIndex = (currentIndex - 1 + DEAL_TEMPLATES.length) % DEAL_TEMPLATES.length
    else if (event.key === 'Home') nextIndex = 0
    else if (event.key === 'End') nextIndex = DEAL_TEMPLATES.length - 1
    if (nextIndex === null) return
    event.preventDefault()
    if (nextIndex === currentIndex) return
    const nextTemplate = DEAL_TEMPLATES[nextIndex]
    handleSelectTemplate(nextTemplate.id)
    tabRefs.current[nextTemplate.id]?.focus()
  }

  const template = useMemo(() => getTemplateById(state.templateId), [state.templateId])
  const terms = useMemo(() => getActiveTerms(template, state.countered), [template, state.countered])
  const artifact = useMemo(() => getActiveArtifact(template, state.countered), [template, state.countered])
  const envelopeJson = useMemo(
    () => JSON.stringify(buildOrpEnvelope(template, state), null, 2),
    [template, state]
  )

  useEffect(() => {
    if (!state.isPlaying) return
    const delay = reduced ? 0 : STEP_DELAY_MS
    const timer = window.setTimeout(() => {
      setState((prev) => advanceStep(prev))
    }, delay)
    return () => window.clearTimeout(timer)
  }, [state.isPlaying, state.stage, reduced])

  useEffect(() => {
    if (!copiedJson) return
    const timer = window.setTimeout(() => setCopiedJson(false), 1800)
    return () => window.clearTimeout(timer)
  }, [copiedJson])

  function handleSelectTemplate(id: DealTemplateId) {
    setState((prev) => selectTemplate(prev, id))
  }

  function handleDecision(decision: HumanDecision) {
    setState((prev) => applyHumanDecision(prev, decision, { immediate: reduced }))
  }

  function handleReplay() {
    setState((prev) => resetSimulation(prev, !reduced))
  }

  function handleReset() {
    setState((prev) => resetSimulation(prev, false))
  }

  function handleCopyJson() {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      navigator.clipboard.writeText(envelopeJson).then(() => setCopiedJson(true)).catch(() => {})
    }
  }

  const isApprovedFlow = state.stage === 'active' || state.stage === 'execute' || state.stage === 'receipt'
  const isReceipt = state.stage === 'receipt'
  const isRefused = state.stage === 'refused'
  const isWaitingHuman = state.stage === 'ask'
  const statusAnnouncement = isReceipt
    ? `${template.label} simulator deal approved and verified in demo with receipt ${template.proofId}.`
    : isRefused
      ? `${template.label} simulator deal refused. Zero placement inserted.`
      : isWaitingHuman
        ? `${template.label} simulator waiting at human approval gate${state.countered ? ' with countered terms' : ''}.`
        : `${template.label} simulator at ${state.stage} stage.`
  const darkFocus = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#FAFAFA]'

  return (
    <section
      aria-label="ORP 0.4 Interactive Deal Simulator"
      data-deal-simulator
      data-template={state.templateId}
      data-stage={state.stage}
      className={cn(
        'relative overflow-hidden rounded-lg border border-white/[0.1] bg-[#111317] text-[#FAFAFA]',
        className
      )}
    >
      <div role="status" aria-live="polite" className="sr-only">
        {statusAnnouncement}
      </div>

      {/* Top workbench bar */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/[0.09] bg-[#0B0C0E] px-4 py-2.5 sm:px-5">
        <div className="flex items-center gap-2.5">
          <span className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.12] bg-[#111317] px-2 py-0.5 font-mono text-[11px] font-medium text-[#FAFAFA]">
            <span className="h-1.5 w-1.5 rounded-full bg-[#C64F38]" aria-hidden="true" />
            Simulator
          </span>
          <span className="font-mono text-xs text-[#A1A1AA]">
            orp/0.4 · {template.dealId}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            onClick={handleReplay}
            className={cn(
              'inline-flex min-h-11 items-center gap-1.5 rounded-md border border-white/[0.1] bg-[#111317] px-3 text-xs font-medium text-[#FAFAFA] transition-colors hover:border-white/[0.2] hover:bg-[#181A20]',
              darkFocus
            )}
          >
            <Play className="h-3.5 w-3.5 text-[#E07A63]" aria-hidden="true" />
            Replay wire
          </button>
          <button
            type="button"
            onClick={handleReset}
            className={cn(
              'inline-flex min-h-11 items-center gap-1.5 rounded-md border border-white/[0.1] bg-[#111317] px-3 text-xs font-medium text-[#A1A1AA] transition-colors hover:border-white/[0.2] hover:text-[#FAFAFA]',
              darkFocus
            )}
          >
            <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
            Reset
          </button>
          <button
            type="button"
            onClick={() => setState((prev) => toggleJsonDrawer(prev))}
            aria-expanded={state.showJson}
            className={cn(
              'inline-flex min-h-11 items-center gap-1.5 rounded-md border px-3 font-mono text-xs font-medium transition-colors',
              darkFocus,
              state.showJson
                ? 'border-[#FAFAFA] bg-[#FAFAFA] text-[#0B0C0E]'
                : 'border-white/[0.1] bg-[#111317] text-[#A1A1AA] hover:border-white/[0.2] hover:text-[#FAFAFA]'
            )}
          >
            <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
            ORP JSON
            {state.showJson ? <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" /> : <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />}
          </button>
        </div>
      </div>

      {/* Underline template tab bar (zero pill tabs) */}
      <div
        role="tablist"
        aria-label="ORP deal templates"
        className="flex flex-wrap gap-1 border-b border-white/[0.09] px-4 sm:px-5"
      >
        {DEAL_TEMPLATES.map((item, idx) => {
          const active = item.id === state.templateId
          return (
            <button
              key={item.id}
              ref={(el) => { tabRefs.current[item.id] = el }}
              type="button"
              role="tab"
              id={`sim-tab-${item.id}`}
              tabIndex={active ? 0 : -1}
              aria-selected={active}
              aria-controls="sim-tabpanel"
              data-template-pill={item.id}
              onClick={() => handleSelectTemplate(item.id)}
              onKeyDown={(event) => onTemplateTabKeyDown(event, idx)}
              className={cn(
                '-mb-px inline-flex min-h-11 items-center gap-2 border-b-2 px-3 text-sm font-medium transition-colors',
                darkFocus,
                active
                  ? 'border-[#C64F38] text-[#FAFAFA]'
                  : 'border-transparent text-[#A1A1AA] hover:text-[#FAFAFA]'
              )}
            >
              <span>{item.label}</span>
            </button>
          )
        })}
      </div>

      {/* Main 2-column hairline-divided workbench */}
      <div
        id="sim-tabpanel"
        role="tabpanel"
        aria-labelledby={`sim-tab-${state.templateId}`}
        className="grid divide-y divide-white/[0.09] lg:grid-cols-[1.12fr_0.88fr] lg:divide-x lg:divide-y-0"
      >
        {/* Left pane: Parties header + flat hairline timeline rows */}
        <div className="flex min-w-0 flex-col">
          {/* Parties bar */}
          <div className="border-b border-white/[0.09] bg-[#0B0C0E]/50 px-5 py-3.5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-[11px] text-[#A1A1AA]">{template.offerer.label}</p>
                <p className="truncate font-mono text-xs font-semibold text-[#FAFAFA]">
                  {template.offerer.handle}
                </p>
                <p className="font-mono text-[11px] text-[#71717A]">{template.offerer.keyFingerprint}</p>
              </div>

              <ArrowLeftRight className="h-4 w-4 shrink-0 text-[#71717A]" aria-hidden="true" />

              <div className="min-w-0 text-right">
                <p className="text-[11px] text-[#A1A1AA]">{template.acceptor.label}</p>
                <p className="truncate font-mono text-xs font-semibold text-[#FAFAFA]">
                  {template.acceptor.handle}
                </p>
                <p className="font-mono text-[11px] text-[#71717A]">{template.acceptor.keyFingerprint}</p>
              </div>
            </div>

            {template.extraParty ? (
              <div className="mt-2.5 flex flex-wrap items-center justify-between gap-2 border-t border-white/[0.09] pt-2 font-mono text-[11px] text-[#A1A1AA]">
                <span>3rd seat · {template.extraParty.label}: {template.extraParty.handle}</span>
                <span className="text-[#71717A]">{template.extraParty.keyFingerprint}</span>
              </div>
            ) : null}
          </div>

          {/* Flat hairline-divided wire steps */}
          <ol className="divide-y divide-white/[0.09]" aria-label="ORP deal execution steps">
            <TimelineRow
              verb="discover"
              reached={stageReached(state, 'discover')}
              active={state.stage === 'discover'}
              reduced={reduced}
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm text-[#FAFAFA]">{template.discoverMatch}</p>
                <span className="font-mono text-xs text-[#A1A1AA] tabular">{template.discoverScore}</span>
              </div>
            </TimelineRow>

            <TimelineRow
              verb="offer"
              reached={stageReached(state, 'offer')}
              active={state.stage === 'offer'}
              reduced={reduced}
            >
              <p className="text-sm font-semibold text-[#FAFAFA]">{terms.headline}</p>
              <p className="mt-0.5 text-xs text-[#A1A1AA]">{terms.windowLabel}</p>
              <div className="mt-2.5 flex flex-wrap items-center gap-3">
                <span
                  data-terms-hash
                  className="inline-flex items-center gap-1.5 font-mono text-[11px] text-[#A1A1AA]"
                  title={terms.termsHash}
                >
                  <Hash className="h-3 w-3 text-[#E07A63]" aria-hidden="true" />
                  {shortHash(terms.termsHash)}
                </span>
                <button
                  type="button"
                  onClick={() => setState((prev) => toggleJsonDrawer(prev))}
                  className={cn(
                    'inline-flex min-h-9 items-center gap-1 font-mono text-xs text-[#E07A63] underline-offset-4 hover:underline',
                    darkFocus
                  )}
                >
                  <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
                  {state.showJson ? 'Hide JSON' : 'Inspect JSON'}
                </button>
              </div>
            </TimelineRow>

            {state.countered ? (
              <TimelineRow
                verb="counter"
                reached
                active={state.stage === 'counter'}
                reduced={reduced}
              >
                <p className="text-sm text-[#FAFAFA]">{template.counterNote}</p>
                <p className="mt-1 font-mono text-[11px] text-[#FBBF24]">
                  terms_hash → {shortHash(template.counteredTerms.termsHash)}
                </p>
              </TimelineRow>
            ) : null}

            {/* Human Approval Gate row */}
            <li
              data-human-gate
              className={cn(
                'grid grid-cols-1 gap-2 px-5 py-4 transition-colors sm:grid-cols-[92px_minmax(0,1fr)] sm:gap-4',
                isWaitingHuman
                  ? 'bg-[#C64F38]/[0.08]'
                  : isRefused
                    ? 'bg-red-500/[0.06]'
                    : isApprovedFlow
                      ? 'bg-[#4ADE80]/[0.04]'
                      : 'opacity-55'
              )}
            >
              <div className="font-mono text-xs">
                <span className="inline-flex items-center gap-1.5 font-semibold text-[#FAFAFA]">
                  <UserCheck
                    className={cn(
                      'h-3.5 w-3.5',
                      isApprovedFlow ? 'text-[#4ADE80]' : isRefused ? 'text-red-400' : 'text-[#FBBF24]'
                    )}
                    aria-hidden="true"
                  />
                  ask
                </span>
                <span className="mt-0.5 block text-[11px] text-[#71717A]">human gate</span>
              </div>

              <div className="min-w-0">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-sm font-semibold text-[#FAFAFA]">Human approval gate</p>
                  {isApprovedFlow ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs text-[#4ADE80]">
                      <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                      Approved
                    </span>
                  ) : isRefused ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs text-red-400">
                      <Ban className="h-3.5 w-3.5" aria-hidden="true" />
                      Refused
                    </span>
                  ) : isWaitingHuman ? (
                    <span className="font-mono text-xs text-[#FBBF24]">Awaiting decision</span>
                  ) : null}
                </div>

                <p className="mt-1 text-xs leading-relaxed text-[#A1A1AA]">
                  {isWaitingHuman
                    ? 'Nothing publishes or moves until you decide. Approve to sign and execute, Counter to negotiate better terms, or Refuse to halt.'
                    : isApprovedFlow
                      ? `Human signed acceptance for ${shortHash(terms.termsHash)}. Agents proceeded to execution.`
                      : isRefused
                        ? 'You refused this proposal. Deal halted cleanly. Zero placement inserted, zero money moved.'
                        : 'Agents pause here for human sign-off before anything touches your site.'}
                </p>

                {isWaitingHuman ? (
                  <div className="mt-3.5 flex flex-wrap gap-2">
                    <button
                      type="button"
                      data-action="approve"
                      onClick={() => handleDecision('approve')}
                      className={cn(
                        'inline-flex min-h-11 items-center gap-1.5 rounded-md bg-[#A43C28] px-4 text-sm font-semibold text-[#FAFAFA] transition-colors hover:bg-[#C64F38]',
                        darkFocus
                      )}
                    >
                      <Check className="h-4 w-4" aria-hidden="true" />
                      Approve Deal
                    </button>
                    <button
                      type="button"
                      data-action="counter"
                      disabled={state.countered}
                      onClick={() => handleDecision('counter')}
                      className={cn(
                        'inline-flex min-h-11 items-center gap-1.5 rounded-md border px-3.5 text-sm font-medium transition-colors',
                        darkFocus,
                        state.countered
                          ? 'cursor-not-allowed border-white/[0.08] bg-transparent text-[#71717A]'
                          : 'border-white/[0.16] bg-[#0B0C0E] text-[#FAFAFA] hover:border-white/[0.3]'
                      )}
                    >
                      <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                      {state.countered ? 'Counter Applied' : 'Counter'}
                    </button>
                    <button
                      type="button"
                      data-action="refuse"
                      onClick={() => handleDecision('refuse')}
                      className={cn(
                        'inline-flex min-h-11 items-center gap-1.5 rounded-md border border-white/[0.12] bg-transparent px-3.5 text-sm font-medium text-[#A1A1AA] transition-colors hover:border-red-400/50 hover:text-red-300',
                        darkFocus
                      )}
                    >
                      <Ban className="h-4 w-4" aria-hidden="true" />
                      Refuse
                    </button>
                  </div>
                ) : null}
              </div>
            </li>

            {/* Post-approval execution & receipt row */}
            {isApprovedFlow ? (
              <motion.li
                data-receipt-panel
                initial={reduced ? false : { opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.2 }}
                className="grid grid-cols-1 gap-2 px-5 py-4 sm:grid-cols-[92px_minmax(0,1fr)] sm:gap-4"
              >
                <div className="font-mono text-xs">
                  <span className={cn('font-semibold', isReceipt ? 'text-[#4ADE80]' : 'text-[#FAFAFA]')}>
                    {isReceipt ? 'receipt' : state.stage}
                  </span>
                  <span className="mt-0.5 block text-[11px] text-[#71717A]">
                    {isReceipt ? 'verified' : 'running'}
                  </span>
                </div>

                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2 font-mono text-[11px] text-[#A1A1AA]">
                    <span className={stageReached(state, 'active') ? 'text-[#4ADE80]' : ''}>active</span>
                    <span aria-hidden="true">→</span>
                    <span className={stageReached(state, 'execute') ? 'text-[#4ADE80]' : ''}>execute</span>
                    <span aria-hidden="true">→</span>
                    <span className={isReceipt ? 'font-semibold text-[#4ADE80]' : ''}>receipt</span>
                  </div>

                  {isReceipt ? (
                    <div className="mt-2.5 space-y-2">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#FAFAFA]">
                          <FileCheck2 className="h-4 w-4 text-[#4ADE80]" aria-hidden="true" />
                          Verified ORP Receipt issued
                        </span>
                        <span
                          data-proof-id
                          className="font-mono text-xs font-semibold text-[#4ADE80]"
                        >
                          {template.proofId}
                        </span>
                      </div>
                      <p className="break-all font-mono text-xs text-[#A1A1AA]">
                        GET /orp/proof/{template.proofId} · hash-chained &amp; signed by both agents
                      </p>
                      <ul className="space-y-1 pt-1 text-xs text-[#D4D4D8]">
                        {template.hardChecks.map((check) => (
                          <li key={check} className="flex items-center gap-2">
                            <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[#4ADE80]" aria-hidden="true" />
                            <span>{check}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : (
                    <p className="mt-2 flex items-center gap-2 text-xs text-[#A1A1AA]">
                      <Zap className="h-3.5 w-3.5 text-[#FBBF24]" aria-hidden="true" />
                      {state.stage === 'active'
                        ? 'Signing hash-chained acceptance envelope…'
                        : 'Verifying hard checks and issuing ORP Receipt…'}
                    </p>
                  )}
                </div>
              </motion.li>
            ) : null}
          </ol>

          {/* Toggleable ORP JSON Drawer (flat hairline top divider) */}
          <AnimatePresence initial={false}>
            {state.showJson ? (
              <motion.div
                data-orp-json-drawer
                initial={reduced ? false : { opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={reduced ? undefined : { opacity: 0, height: 0 }}
                transition={{ duration: 0.2 }}
                className="overflow-hidden border-t border-white/[0.09] bg-[#0B0C0E]"
              >
                <div className="flex items-center justify-between border-b border-white/[0.09] px-5 py-2">
                  <span className="font-mono text-xs text-[#A1A1AA]">
                    ORP 0.4 Wire Payload · {template.dealId}
                  </span>
                  <button
                    type="button"
                    onClick={handleCopyJson}
                    className={cn(
                      'inline-flex min-h-11 items-center gap-1.5 rounded-md border border-white/[0.12] bg-[#111317] px-3 font-mono text-xs text-[#FAFAFA] hover:border-white/[0.24]',
                      darkFocus
                    )}
                  >
                    {copiedJson ? <Check className="h-3.5 w-3.5 text-[#4ADE80]" /> : <Copy className="h-3.5 w-3.5" />}
                    {copiedJson ? 'Copied' : 'Copy JSON'}
                  </button>
                </div>
                <pre className="max-h-72 overflow-auto p-5 font-mono text-xs leading-relaxed text-[#D4D4D8]">
                  <code>{envelopeJson}</code>
                </pre>
              </motion.div>
            ) : null}
          </AnimatePresence>
        </div>

        {/* Right pane: Minimal-chrome Artifact & Receipt preview */}
        <div className="flex min-w-0 flex-col bg-[#0B0C0E]/40">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/[0.09] px-5 py-3.5">
            <span className="text-xs font-semibold text-[#A1A1AA]">Resulting artifact</span>
            {isReceipt ? (
              <span className="inline-flex items-center gap-1.5 rounded-md border border-[#4ADE80]/30 bg-[#4ADE80]/10 px-2 py-0.5 font-mono text-[11px] font-medium text-[#4ADE80]">
                <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
                Simulated · Verified in demo
              </span>
            ) : isRefused ? (
              <span className="inline-flex items-center gap-1.5 rounded-md border border-red-400/30 bg-red-500/10 px-2 py-0.5 font-mono text-[11px] font-medium text-red-300">
                <Ban className="h-3 w-3" aria-hidden="true" />
                Halted · 0 Placements
              </span>
            ) : (
              <span className="inline-flex items-center gap-1.5 rounded-md border border-[#FBBF24]/30 bg-[#FBBF24]/10 px-2 py-0.5 font-mono text-[11px] font-medium text-[#FBBF24]">
                Preview · Uncommitted
              </span>
            )}
          </div>

          {isRefused ? (
            <div data-refused-preview className="flex flex-1 flex-col justify-between">
              <div className="p-5">
                <div className="flex items-center gap-2 text-red-300">
                  <Ban className="h-4 w-4" aria-hidden="true" />
                  <h3 className="font-display text-base font-bold text-[#FAFAFA]">
                    Zero placement. Page untouched.
                  </h3>
                </div>
                <p className="mt-2 text-sm leading-relaxed text-[#A1A1AA]">
                  Because <code className="font-mono text-xs text-[#FAFAFA]">ask</code> mode gated execution, refusing the offer stopped the deal before any link or card reached your site.
                </p>
                <button
                  type="button"
                  onClick={handleReset}
                  className={cn(
                    'mt-5 inline-flex min-h-11 items-center gap-1.5 rounded-md border border-white/[0.14] bg-[#111317] px-3.5 text-xs font-semibold text-[#FAFAFA] hover:border-white/[0.28]',
                    darkFocus
                  )}
                >
                  <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
                  Re-open at human gate
                </button>
              </div>
              <div className="grid grid-cols-3 divide-x divide-white/[0.09] border-t border-white/[0.09] font-mono text-xs">
                <div className="px-4 py-3">
                  <p className="text-[11px] text-[#71717A]">Inserted</p>
                  <p className="mt-0.5 font-semibold text-[#FAFAFA]">0 boxes</p>
                </div>
                <div className="px-4 py-3">
                  <p className="text-[11px] text-[#71717A]">Cash moved</p>
                  <p className="mt-0.5 font-semibold text-[#FAFAFA]">€0</p>
                </div>
                <div className="px-4 py-3">
                  <p className="text-[11px] text-[#71717A]">Status</p>
                  <p className="mt-0.5 font-semibold text-red-300">refused</p>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-1 flex-col justify-between">
              <div className="p-5">
                {/* Single placement preview frame (paper contrast when verified) */}
                <div
                  className={cn(
                    'rounded-lg border p-4 transition-colors',
                    isReceipt
                      ? 'border-white bg-[#FFFFFF] text-[#09090B]'
                      : 'border-white/[0.1] bg-[#111317] text-[#FAFAFA]'
                  )}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                    <span
                      className={cn(
                        'font-mono text-[11px] font-medium',
                        isReceipt ? 'text-[#A43C28]' : 'text-[#E07A63]'
                      )}
                    >
                      {artifact.badge}
                    </span>
                    <span
                      className={cn(
                        'font-mono text-[11px]',
                        isReceipt ? 'text-[#52525B]' : 'text-[#71717A]'
                      )}
                    >
                      {template.dealId}
                    </span>
                  </div>

                  <h3 className="mt-2.5 font-display text-base font-bold leading-snug">
                    {artifact.title}
                  </h3>
                  <p className={cn('mt-0.5 text-xs', isReceipt ? 'text-[#52525B]' : 'text-[#A1A1AA]')}>
                    {artifact.subtitle}
                  </p>
                  <p className={cn('mt-2.5 text-sm leading-relaxed', isReceipt ? 'text-[#27272A]' : 'text-[#D4D4D8]')}>
                    {artifact.body}
                  </p>

                  {artifact.swapSides ? (
                    <div
                      className={cn(
                        'mt-3 divide-y border-t',
                        isReceipt ? 'divide-[#E4E4E7] border-[#E4E4E7]' : 'divide-white/[0.09] border-white/[0.09]'
                      )}
                    >
                      {artifact.swapSides.map((side) => (
                        <div key={side.publisher} className="flex items-center justify-between gap-2 py-2 text-xs">
                          <div className="min-w-0">
                            <p className="font-semibold">{side.publisher} · {side.issue}</p>
                            <p className={isReceipt ? 'text-[#52525B]' : 'text-[#A1A1AA]'}>{side.target}</p>
                          </div>
                          <span className={cn('shrink-0 font-mono text-[11px] font-medium', isReceipt ? 'text-[#166534]' : 'text-[#FBBF24]')}>
                            {isReceipt ? side.verified : 'Queued'}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : null}

                  {artifact.ledgerRows ? (
                    <div
                      className={cn(
                        'mt-3 overflow-x-auto border-t pt-2',
                        isReceipt ? 'border-[#E4E4E7]' : 'border-white/[0.09]'
                      )}
                    >
                      <table className="w-full text-left text-xs">
                        <thead>
                          <tr className={isReceipt ? 'text-[#52525B]' : 'text-[#71717A]'}>
                            <th className="py-1 pr-2 font-mono font-medium">Party</th>
                            <th className="px-2 py-1 font-mono font-medium">Split</th>
                            <th className="py-1 pl-2 text-right font-mono font-medium">Payout</th>
                          </tr>
                        </thead>
                        <tbody className={cn('divide-y', isReceipt ? 'divide-[#E4E4E7]' : 'divide-white/[0.09]')}>
                          {artifact.ledgerRows.map((row) => (
                            <tr key={row.party}>
                              <td className="py-1.5 pr-2 font-medium">
                                {row.party}{' '}
                                <span className={isReceipt ? 'text-[#52525B]' : 'text-[#71717A]'}>({row.role})</span>
                              </td>
                              <td className="px-2 py-1.5 font-mono tabular">{row.share}</td>
                              <td className="py-1.5 pl-2 text-right font-mono font-semibold tabular">{row.payout}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ) : null}

                  <div
                    className={cn(
                      'mt-3 flex flex-wrap items-center justify-between gap-2 border-t pt-2.5 font-mono text-[11px]',
                      isReceipt ? 'border-[#E4E4E7] text-[#52525B]' : 'border-white/[0.09] text-[#71717A]'
                    )}
                  >
                    <span>{artifact.disclosureLabel}</span>
                    <span className="truncate">{artifact.trackedUrl}</span>
                  </div>
                </div>
              </div>

              <div>
                {/* Flat hairline-divided 3-column metrics strip */}
                <div className="grid grid-cols-3 divide-x divide-white/[0.09] border-t border-white/[0.09]">
                  {artifact.metrics.map((m) => (
                    <div key={m.label} className="px-4 py-3">
                      <p className="text-[11px] text-[#71717A]">{m.label}</p>
                      <p className="mt-0.5 font-mono text-xs font-semibold text-[#FAFAFA] tabular sm:text-sm">
                        {m.value}
                      </p>
                    </div>
                  ))}
                </div>

                {/* Flat terms fingerprint footer */}
                <div className="flex flex-wrap items-center justify-between gap-2 border-t border-white/[0.09] px-5 py-3 font-mono text-[11px] text-[#A1A1AA]">
                  <span className="truncate text-[#FAFAFA]">{shortHash(terms.termsHash)}</span>
                  <span className={state.countered ? 'text-[#FBBF24]' : 'text-[#71717A]'}>
                    {state.countered ? 'v2 (countered)' : 'v1 (initial)'}
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  )
}

function TimelineRow({
  verb,
  reached,
  active,
  reduced,
  children,
}: {
  verb: string
  reached: boolean
  active: boolean
  reduced: boolean
  children: ReactNode
}) {
  return (
    <motion.li
      initial={reduced ? false : { opacity: 0, y: 4 }}
      animate={{ opacity: reached ? 1 : 0.45, y: 0 }}
      transition={{ duration: 0.18 }}
      className={cn(
        'grid grid-cols-1 gap-2 px-5 py-4 transition-colors sm:grid-cols-[92px_minmax(0,1fr)] sm:gap-4',
        active ? 'bg-white/[0.03]' : ''
      )}
    >
      <div className="font-mono text-xs">
        <span className={cn('font-semibold', active ? 'text-[#E07A63]' : reached ? 'text-[#FAFAFA]' : 'text-[#71717A]')}>
          {verb}
        </span>
      </div>
      <div className="min-w-0">{children}</div>
    </motion.li>
  )
}
