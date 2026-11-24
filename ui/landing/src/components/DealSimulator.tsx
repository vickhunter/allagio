import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
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

function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(
    () => typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
  useEffect(() => {
    if (typeof window === 'undefined') return
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const onChange = () => setReduced(query.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])
  return reduced
}

const STEP_DELAY_MS = 650

function stageReached(state: SimulatorState, target: SimulatorStage): boolean {
  return state.stage === target || state.history.includes(target)
}

export interface DealSimulatorProps {
  initialTemplate?: DealTemplateId
  className?: string
}

export function DealSimulator({ initialTemplate = 'affiliate', className }: DealSimulatorProps) {
  const reduced = usePrefersReducedMotion()
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
  const darkFocus = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#F5F3EE]'

  return (
    <section
      aria-label="ORP 0.4 Interactive Deal Simulator"
      data-deal-simulator
      data-template={state.templateId}
      data-stage={state.stage}
      className={cn(
        'relative overflow-hidden rounded-2xl border border-[#2E2E2B] bg-[#1B1B19] text-[#F5F3EE]',
        className
      )}
    >
      <div role="status" aria-live="polite" className="sr-only">
        {statusAnnouncement}
      </div>
      <div className="relative p-5 sm:p-7 lg:p-8">
        {/* Compact toolbar */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center rounded-full border border-[#3E3E39] bg-[#141413] px-2.5 py-0.5 font-mono text-[11px] font-semibold uppercase tracking-wider text-[#F5F3EE]">
              Simulator
            </span>
            <p className="font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-[#B5B2A8]">
              ORP 0.4 · Interactive walkthrough
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={handleReplay}
              className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-[#343430] bg-[#141413] px-3.5 text-xs font-semibold text-[#F5F3EE] transition-[color,border-color,background-color,transform] hover:border-[#52524C] hover:bg-[#22221F] active:scale-[0.98]', darkFocus)}
            >
              <Play className="h-3.5 w-3.5 text-[#E07A63]" aria-hidden="true" />
              Replay wire
            </button>
            <button
              type="button"
              onClick={handleReset}
              className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-[#343430] bg-[#141413] px-3.5 text-xs font-semibold text-[#B5B2A8] transition-[color,border-color,transform] hover:border-[#52524C] hover:text-[#F5F3EE] active:scale-[0.98]', darkFocus)}
            >
              <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
              Reset
            </button>
            <button
              type="button"
              onClick={() => setState((prev) => toggleJsonDrawer(prev))}
              aria-expanded={state.showJson}
              className={cn(
                'inline-flex min-h-11 items-center gap-1.5 rounded-lg border px-3.5 font-mono text-xs font-medium transition-[color,border-color,background-color,transform] active:scale-[0.98]',
                darkFocus,
                state.showJson
                  ? 'border-[#F5F3EE] bg-[#F5F3EE] text-[#141413]'
                  : 'border-[#343430] bg-[#141413] text-[#B5B2A8] hover:border-[#52524C] hover:text-[#F5F3EE]'
              )}
            >
              <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
              View ORP JSON
              {state.showJson ? <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" /> : <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />}
            </button>
          </div>
        </div>

        {/* Template selector pills */}
        <div
          role="tablist"
          aria-label="ORP deal templates"
          className="mt-6 flex flex-wrap gap-2 border-b border-[#2E2E2B] pb-5"
        >
          {DEAL_TEMPLATES.map((item, idx) => {
            const active = item.id === state.templateId
            const showOrpKind = item.label.toLowerCase() !== item.orpKind.toLowerCase()
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
                  'inline-flex min-h-11 items-center gap-2 rounded-full border px-4 py-1.5 text-sm font-semibold transition-[color,border-color,background-color,transform] active:scale-[0.98]',
                  darkFocus,
                  active
                    ? 'border-[#F5F3EE] bg-[#F5F3EE] text-[#141413]'
                    : 'border-[#343430] bg-[#141413] text-[#B5B2A8] hover:border-[#52524C] hover:text-[#F5F3EE]'
                )}
              >
                <span>{item.label}</span>
                {showOrpKind ? (
                  <span
                    className={cn(
                      'font-mono text-[11px] uppercase',
                      active ? 'text-[#52524C]' : 'text-[#B5B2A8]'
                    )}
                  >
                    {item.orpKind}
                  </span>
                ) : null}
              </button>
            )
          })}
        </div>

        {/* Main 2-column simulator body */}
        <div
          id="sim-tabpanel"
          role="tabpanel"
          aria-labelledby={`sim-tab-${state.templateId}`}
          className="mt-6 grid gap-6 lg:grid-cols-[1.15fr_0.85fr] lg:items-start"
        >
          {/* Left: Named Agents + Live ORP Wire Timeline */}
          <div className="flex min-w-0 flex-col gap-4">
            {/* Parties bar */}
            <div className="rounded-2xl border border-[#F7F5F0]/10 bg-[#1B1B19] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">
                    {template.offerer.label}
                  </p>
                  <p className="truncate font-mono text-sm font-semibold text-[#F7F5F0]">
                    {template.offerer.handle}
                  </p>
                  <p className="font-mono text-[11px] text-[#E07A63]">{template.offerer.keyFingerprint}</p>
                </div>

                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-[#C64F38]/40 bg-[#C64F38]/15 text-[#E07A63]">
                  <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                </div>

                <div className="min-w-0 flex-1 text-right">
                  <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">
                    {template.acceptor.label}
                  </p>
                  <p className="truncate font-mono text-sm font-semibold text-[#F7F5F0]">
                    {template.acceptor.handle}
                  </p>
                  <p className="font-mono text-[11px] text-[#B5B2A8]">{template.acceptor.keyFingerprint}</p>
                </div>
              </div>

              {template.extraParty ? (
                <div className="mt-3 flex items-center justify-between border-t border-[#F7F5F0]/10 pt-2.5 font-mono text-[11px] text-[#B8BFB2]">
                  <span>3rd seat · {template.extraParty.label}: {template.extraParty.handle}</span>
                  <span className="text-[#E07A63]">{template.extraParty.keyFingerprint}</span>
                </div>
              ) : null}
            </div>

            {/* Wire steps */}
            <ol className="flex flex-col gap-3" aria-label="ORP deal execution steps">
              {/* Step 1: discover */}
              <TimelineStep
                index="01"
                verb="discover"
                status={stageReached(state, 'discover') ? 'done' : 'pending'}
                active={state.stage === 'discover'}
                reduced={reduced}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-sm font-medium text-[#F7F5F0]">{template.discoverMatch}</p>
                  <span className="rounded-md bg-[#C64F38]/20 px-2 py-0.5 font-mono text-[11px] text-[#E07A63]">
                    {template.discoverScore}
                  </span>
                </div>
              </TimelineStep>

              {/* Step 2: offer */}
              <TimelineStep
                index="02"
                verb="offer"
                status={stageReached(state, 'offer') ? 'done' : 'pending'}
                active={state.stage === 'offer'}
                reduced={reduced}
              >
                <p className="text-sm font-semibold text-[#F7F5F0]">{terms.headline}</p>
                <p className="mt-1 text-xs text-[#B8BFB2]">{terms.windowLabel}</p>
                <div className="mt-2.5 flex flex-wrap items-center gap-2">
                  <span
                    data-terms-hash
                    className="inline-flex items-center gap-1.5 rounded-lg border border-[#F7F5F0]/12 bg-[#141413] px-2.5 py-1 font-mono text-[11px] text-[#B8BFB2]"
                    title={terms.termsHash}
                  >
                    <Hash className="h-3 w-3 text-[#E07A63]" aria-hidden="true" />
                    {shortHash(terms.termsHash)}
                  </span>
                  <button
                    type="button"
                    onClick={() => setState((prev) => toggleJsonDrawer(prev))}
                    className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-[#F7F5F0]/15 bg-[#141413] px-3 font-mono text-xs text-[#E07A63] transition-colors hover:border-[#C64F38]', darkFocus)}
                  >
                    <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
                    {state.showJson ? 'Hide ORP JSON' : 'Inspect JSON'}
                  </button>
                </div>
              </TimelineStep>

              {/* Step 3: counter (shown when countered) */}
              {state.countered ? (
                <TimelineStep
                  index="03"
                  verb="counter"
                  accent="amber"
                  status="done"
                  active={state.stage === 'counter'}
                  reduced={reduced}
                >
                  <p className="text-sm font-medium text-[#F7F5F0]">{template.counterNote}</p>
                  <p className="mt-1.5 font-mono text-[11px] text-[#E07A63]">
                    Updated terms_hash → {shortHash(template.counteredTerms.termsHash)}
                  </p>
                </TimelineStep>
              ) : null}

              {/* Step 4: Human Approval Gate (ask mode) */}
              <li
                data-human-gate
                className={cn(
                  'rounded-xl border p-4 transition-colors',
                  isWaitingHuman
                    ? 'border-[#C64F38]/60 bg-[#141413]'
                    : isRefused
                      ? 'border-red-400/45 bg-[#141413]'
                      : isApprovedFlow
                        ? 'border-[#3E5C45] bg-[#141413]'
                        : 'border-[#2E2E2B] bg-[#141413]/70 opacity-60'
                )}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-[11px] font-semibold text-[#E07A63]">
                      {state.countered ? '04' : '03'}
                    </span>
                    <span className="inline-flex items-center gap-1.5 font-mono text-xs font-semibold uppercase tracking-wider text-[#F5F3EE]">
                      <UserCheck className="h-3.5 w-3.5 text-[#E07A63]" aria-hidden="true" />
                      Human Approval Gate
                    </span>
                    <span className="rounded border border-[#343430] bg-[#1B1B19] px-2 py-0.5 font-mono text-[11px] uppercase text-[#B5B2A8]">
                      ask mode
                    </span>
                  </div>

                  {isApprovedFlow ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs font-semibold text-[#86C79A]">
                      <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                      Approved
                    </span>
                  ) : isRefused ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs font-semibold text-red-400">
                      <Ban className="h-3.5 w-3.5" aria-hidden="true" />
                      Refused
                    </span>
                  ) : isWaitingHuman ? (
                    <span className="font-mono text-[11px] text-[#E07A63]">Awaiting your switch</span>
                  ) : null}
                </div>

                <p className="mt-2 text-xs leading-relaxed text-[#B5B2A8]">
                  {isWaitingHuman
                    ? 'Nothing publishes or moves until you decide. Approve to sign and execute, Counter to negotiate better terms, or Refuse to halt with zero placement.'
                    : isApprovedFlow
                      ? `Human signed acceptance for ${shortHash(terms.termsHash)}. Agents proceeded to execution.`
                      : isRefused
                        ? 'You refused this proposal. Deal halted cleanly. Zero placement inserted, zero money moved.'
                        : 'Agents will pause here for human sign-off before anything touches your site.'}
                </p>

                {isWaitingHuman ? (
                  <div className="mt-4 flex flex-wrap gap-2.5">
                    <button
                      type="button"
                      data-action="approve"
                      onClick={() => handleDecision('approve')}
                      className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg bg-[#F5F3EE] px-4 text-sm font-semibold text-[#141413] transition-[opacity,transform] hover:opacity-90 active:scale-[0.98]', darkFocus)}
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
                        'inline-flex min-h-11 items-center gap-1.5 rounded-lg border px-4 text-sm font-semibold transition-[color,border-color,transform] active:scale-[0.98]',
                        darkFocus,
                        state.countered
                          ? 'cursor-not-allowed border-[#2E2E2B] bg-[#1B1B19] text-[#7A7A72]'
                          : 'border-[#52524C] bg-[#1B1B19] text-[#F5F3EE] hover:border-[#E07A63] hover:text-[#E07A63]'
                      )}
                    >
                      <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                      {state.countered ? 'Counter Applied' : 'Counter'}
                    </button>
                    <button
                      type="button"
                      data-action="refuse"
                      onClick={() => handleDecision('refuse')}
                      className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-[#343430] bg-transparent px-4 text-sm font-semibold text-[#B5B2A8] transition-[color,border-color,transform] hover:border-red-400/50 hover:text-red-300 active:scale-[0.98]', darkFocus)}
                    >
                      <Ban className="h-4 w-4" aria-hidden="true" />
                      Refuse
                    </button>
                  </div>
                ) : null}
              </li>

              {/* Step 5: active -> execute -> receipt (post-approval) */}
              {isApprovedFlow ? (
                <motion.li
                  data-receipt-panel
                  initial={reduced ? false : { opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                  className={cn(
                    'rounded-2xl border p-4',
                    isReceipt
                      ? 'border-[#C64F38] bg-[#C64F38]/14'
                      : 'border-[#F7F5F0]/15 bg-[#1B1B19]'
                  )}
                >
                  <div className="flex flex-wrap items-center gap-2 font-mono text-[11px]">
                    <StagePill label="active" done={stageReached(state, 'active')} active={state.stage === 'active'} />
                    <span className="text-[#8E9687]" aria-hidden="true">→</span>
                    <StagePill label="execute" done={stageReached(state, 'execute')} active={state.stage === 'execute'} />
                    <span className="text-[#8E9687]" aria-hidden="true">→</span>
                    <StagePill label="ORP Receipt" done={isReceipt} active={isReceipt} />
                  </div>

                  {isReceipt ? (
                    <div className="mt-3 flex flex-col gap-2.5 border-t border-[#C64F38]/30 pt-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#E07A63]">
                          <FileCheck2 className="h-4 w-4" aria-hidden="true" />
                          Verified ORP Receipt issued
                        </span>
                        <span
                          data-proof-id
                          className="rounded-md border border-[#C64F38]/50 bg-[#141413] px-2.5 py-1 font-mono text-xs font-semibold text-[#E07A63]"
                        >
                          {template.proofId}
                        </span>
                      </div>
                      <p className="break-all font-mono text-xs text-[#B8BFB2]">
                        GET /orp/proof/{template.proofId} · hash-chained &amp; signed by both agents
                      </p>
                      <ul className="mt-1 grid gap-1.5 text-xs text-[#D4D8CE]">
                        {template.hardChecks.map((check) => (
                          <li key={check} className="flex items-center gap-2">
                            <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[#E07A63]" aria-hidden="true" />
                            <span>{check}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : (
                    <p className="mt-3 flex items-center gap-2 text-xs text-[#B8BFB2]">
                      <Zap className="h-3.5 w-3.5 text-[#E07A63]" aria-hidden="true" />
                      {state.stage === 'active'
                        ? 'Signing hash-chained acceptance envelope…'
                        : 'Verifying hard checks and issuing ORP Receipt…'}
                    </p>
                  )}
                </motion.li>
              ) : null}
            </ol>

            {/* Toggleable ORP JSON Drawer */}
            <AnimatePresence initial={false}>
              {state.showJson ? (
                <motion.div
                  data-orp-json-drawer
                  initial={reduced ? false : { opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={reduced ? undefined : { opacity: 0, height: 0 }}
                  transition={{ duration: 0.2 }}
                  className="overflow-hidden rounded-2xl border border-[#F7F5F0]/15 bg-[#0B0D0A]"
                >
                  <div className="flex items-center justify-between border-b border-[#F7F5F0]/10 px-4 py-2.5">
                    <span className="font-mono text-xs text-[#E07A63]">
                      ORP 0.4 Wire Payload · {template.dealId}
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyJson}
                      className={cn('inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-[#F7F5F0]/15 bg-[#1B1B19] px-3 font-mono text-xs text-[#F7F5F0] hover:border-[#C64F38]', darkFocus)}
                    >
                      {copiedJson ? <Check className="h-3.5 w-3.5 text-[#86C79A]" /> : <Copy className="h-3.5 w-3.5" />}
                      {copiedJson ? 'Copied' : 'Copy JSON'}
                    </button>
                  </div>
                  <pre className="max-h-72 overflow-auto p-4 font-mono text-xs leading-relaxed text-[#D4D8CE]">
                    <code>{envelopeJson}</code>
                  </pre>
                </motion.div>
              ) : null}
            </AnimatePresence>
          </div>

          {/* Right: Live Preview Artifact Panel */}
          <div className="relative min-w-0 overflow-hidden rounded-xl border border-[#2E2E2B] bg-[#141413] p-5">

            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#F7F5F0]/10 pb-3.5">
              <span className="font-mono text-[11px] font-medium uppercase tracking-[0.12em] text-[#B8BFB2]">
                Resulting Artifact
              </span>
              {isReceipt ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-[#2E4D36]/45 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-[#86C79A]">
                  <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                  Simulated · Verified in demo
                </span>
              ) : isRefused ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/20 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-red-300">
                  <Ban className="h-3.5 w-3.5" aria-hidden="true" />
                  Halted · 0 Placements
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-[#C64F38]/20 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-[#E07A63]">
                  Preview · Uncommitted
                </span>
              )}
            </div>

            {isRefused ? (
              <div data-refused-preview className="mt-5 rounded-xl border border-red-500/25 bg-[#141413] p-5 text-center">
                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-red-500/15 text-red-300">
                  <Ban className="h-5 w-5" aria-hidden="true" />
                </div>
                <h3 className="mt-3 font-display text-lg font-bold text-[#F7F5F0]">
                  Zero placement. Page untouched.
                </h3>
                <p className="mt-1.5 text-sm leading-relaxed text-[#B8BFB2]">
                  Because <code className="font-mono text-xs text-[#E07A63]">ask</code> mode gated execution, refusing the offer stopped the deal before any link or card reached your site.
                </p>
                <div className="mt-4 grid grid-cols-3 gap-2 border-t border-[#F7F5F0]/10 pt-4 font-mono text-xs">
                  <div>
                    <p className="text-[#8E9687]">Inserted</p>
                    <p className="mt-0.5 font-semibold text-[#F7F5F0]">0 boxes</p>
                  </div>
                  <div>
                    <p className="text-[#8E9687]">Cash moved</p>
                    <p className="mt-0.5 font-semibold text-[#F7F5F0]">€0</p>
                  </div>
                  <div>
                    <p className="text-[#8E9687]">Status</p>
                    <p className="mt-0.5 font-semibold text-red-300">refused</p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleReset}
                  className={cn('mt-5 inline-flex min-h-11 items-center gap-1.5 rounded-xl border border-[#F7F5F0]/20 bg-[#1B1B19] px-4 text-xs font-semibold text-[#F7F5F0] hover:border-[#C64F38]', darkFocus)}
                >
                  <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
                  Re-open at human gate
                </button>
              </div>
            ) : (
              <div className="mt-4 flex flex-col gap-4">
                {/* Light paper-contrast preview card to mirror how it sits in a real post */}
                <div
                  className={cn(
                    'rounded-xl border p-4 transition-colors',
                    isReceipt
                      ? 'border-[#C64F38] bg-[#F7F5F0] text-[#141413]'
                      : 'border-[#F7F5F0]/15 bg-[#141413] text-[#F7F5F0]'
                  )}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span
                      className={cn(
                        'rounded-md px-2 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wider',
                        isReceipt
                          ? 'bg-[#C64F38]/15 text-[#C64F38]'
                          : 'bg-[#C64F38]/20 text-[#E07A63]'
                      )}
                    >
                      {artifact.badge}
                    </span>
                    <span
                      className={cn(
                        'font-mono text-[11px]',
                        isReceipt ? 'text-[#5C6257]' : 'text-[#8E9687]'
                      )}
                    >
                      {template.dealId}
                    </span>
                  </div>

                  <h3 className="mt-3 font-display text-lg font-bold leading-snug">
                    {artifact.title}
                  </h3>
                  <p className={cn('mt-0.5 text-xs', isReceipt ? 'text-[#5C6257]' : 'text-[#B8BFB2]')}>
                    {artifact.subtitle}
                  </p>
                  <p className={cn('mt-2.5 text-sm leading-relaxed', isReceipt ? 'text-[#232720]' : 'text-[#D4D8CE]')}>
                    {artifact.body}
                  </p>

                  {/* Template-specific extra visuals: Swap receipts or Rev-share ledger */}
                  {artifact.swapSides ? (
                    <div className="mt-3 grid gap-2">
                      {artifact.swapSides.map((side) => (
                        <div
                          key={side.publisher}
                          className={cn(
                            'flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs',
                            isReceipt
                              ? 'border-[#232720]/15 bg-white text-[#141413]'
                              : 'border-[#F7F5F0]/10 bg-[#1B1B19] text-[#F7F5F0]'
                          )}
                        >
                          <div>
                            <p className="font-semibold">{side.publisher} · {side.issue}</p>
                            <p className={isReceipt ? 'text-[#5C6257]' : 'text-[#B8BFB2]'}>{side.target}</p>
                          </div>
                          <span className="rounded-md bg-[#C64F38]/20 px-2 py-0.5 font-mono text-[10px] font-semibold text-[#C64F38]">
                            {isReceipt ? side.verified : 'Queued'}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : null}

                  {artifact.ledgerRows ? (
                    <div className="mt-3 overflow-hidden rounded-lg border border-current/15">
                      <table className="w-full text-left text-xs">
                        <thead>
                          <tr className={isReceipt ? 'bg-[#ECEFE2] text-[#5C6257]' : 'bg-[#1B1B19] text-[#8E9687]'}>
                            <th className="px-2.5 py-1.5 font-mono font-medium">Party</th>
                            <th className="px-2.5 py-1.5 font-mono font-medium">Split</th>
                            <th className="px-2.5 py-1.5 text-right font-mono font-medium">Payout</th>
                          </tr>
                        </thead>
                        <tbody>
                          {artifact.ledgerRows.map((row) => (
                            <tr key={row.party} className="border-t border-current/10">
                              <td className="px-2.5 py-1.5 font-medium">
                                {row.party}{' '}
                                <span className={isReceipt ? 'text-[#5C6257]' : 'text-[#8E9687]'}>({row.role})</span>
                              </td>
                              <td className="px-2.5 py-1.5 font-mono tabular">{row.share}</td>
                              <td className="px-2.5 py-1.5 text-right font-mono font-semibold tabular">{row.payout}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ) : null}

                  <div
                    className={cn(
                      'mt-3.5 flex flex-wrap items-center justify-between gap-2 border-t pt-2.5 font-mono text-[11px]',
                      isReceipt ? 'border-[#232720]/12 text-[#5C6257]' : 'border-[#F7F5F0]/10 text-[#8E9687]'
                    )}
                  >
                    <span>{artifact.disclosureLabel}</span>
                    <span className="truncate">{artifact.trackedUrl}</span>
                  </div>
                </div>

                {/* Metrics row */}
                <div className="grid grid-cols-3 gap-2">
                  {artifact.metrics.map((m) => (
                    <div
                      key={m.label}
                      className="rounded-xl border border-[#F7F5F0]/10 bg-[#141413] p-3"
                    >
                      <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">{m.label}</p>
                      <p
                        className={cn(
                          'mt-1 font-mono text-sm font-semibold tabular',
                          m.accent === 'emerald'
                            ? 'text-[#E07A63]'
                            : m.accent === 'amber'
                              ? 'text-[#E07A63]'
                              : 'text-[#F7F5F0]'
                        )}
                      >
                        {m.value}
                      </p>
                    </div>
                  ))}
                </div>

                {/* Terms fingerprint footer */}
                <div className="rounded-xl border border-[#F7F5F0]/10 bg-[#141413] p-3 font-mono text-[11px] text-[#B8BFB2]">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[#8E9687]">TERMS FINGERPRINT</span>
                    <span className="text-[#E07A63]">{state.countered ? 'v2 (countered)' : 'v1 (initial)'}</span>
                  </div>
                  <p className="mt-1 break-all text-[#F7F5F0]">{shortHash(terms.termsHash)}</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}

function TimelineStep({
  index,
  verb,
  status,
  active,
  accent = 'emerald',
  reduced,
  children,
}: {
  index: string
  verb: string
  status: 'done' | 'pending'
  active: boolean
  accent?: 'emerald' | 'amber'
  reduced: boolean
  children: React.ReactNode
}) {
  const isAmber = accent === 'amber'
  return (
    <motion.li
      initial={reduced ? false : { opacity: 0, y: 6 }}
      animate={{ opacity: status === 'done' ? 1 : 0.45, y: 0 }}
      transition={{ duration: 0.2 }}
      className={cn(
        'rounded-2xl border p-4 transition-colors',
        active
          ? isAmber
            ? 'border-[#C64F38] bg-[#C64F38]/10'
            : 'border-[#C64F38] bg-[#C64F38]/12'
          : 'border-[#F7F5F0]/10 bg-[#1B1B19]'
      )}
    >
      <div className="flex items-center gap-2">
        <span
          className={cn(
            'inline-flex h-6 w-6 items-center justify-center rounded-full font-mono text-[11px] font-bold',
            status === 'done'
              ? isAmber
                ? 'bg-[#C64F38]/25 text-[#E07A63]'
                : 'bg-[#C64F38]/25 text-[#E07A63]'
              : 'bg-[#141413] text-[#8E9687]'
          )}
        >
          {index}
        </span>
        <span
          className={cn(
            'font-mono text-xs font-semibold uppercase tracking-wider',
            isAmber ? 'text-[#E07A63]' : 'text-[#E07A63]'
          )}
        >
          {verb}
        </span>
      </div>
      <div className="mt-2.5">{children}</div>
    </motion.li>
  )
}

function StagePill({ label, done, active }: { label: string; done: boolean; active: boolean }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5',
        active
          ? 'border-[#C64F38] bg-[#C64F38] text-[#F7F5F0] font-semibold'
          : done
            ? 'border-[#C64F38]/50 bg-[#C64F38]/20 text-[#E07A63]'
            : 'border-[#F7F5F0]/15 bg-[#141413] text-[#8E9687]'
      )}
    >
      {done ? <Check className="h-3 w-3" aria-hidden="true" /> : null}
      {label}
    </span>
  )
}
