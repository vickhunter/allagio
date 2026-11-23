import { useEffect, useMemo, useState } from 'react'
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
  Sparkles,
  UserCheck,
  Zap,
} from 'lucide-react'
import { BorderBeam } from '@/components/ui/border-beam'
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

  return (
    <section
      aria-labelledby="deal-simulator-h"
      data-deal-simulator
      data-template={state.templateId}
      data-stage={state.stage}
      className={cn(
        'relative overflow-hidden rounded-3xl border border-[#F7F5F0]/12 bg-[#111310] text-[#F7F5F0] shadow-2xl',
        className
      )}
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(80%_60%_at_15%_0%,rgba(46,125,79,0.16),transparent_60%),radial-gradient(60%_50%_at_95%_10%,rgba(217,119,6,0.10),transparent_55%)]"
      />

      <div className="relative p-5 sm:p-8 lg:p-10">
        {/* Top bar */}
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-2xl">
            <p className="inline-flex items-center gap-2 font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-[#6EE7A0]">
              <Sparkles className="h-3.5 w-3.5" aria-hidden="true" />
              ORP 0.4 · Interactive Deal Simulator
            </p>
            <h2
              id="deal-simulator-h"
              className="mt-2 font-display text-2xl font-bold tracking-[-0.03em] text-[#F7F5F0] sm:text-3xl lg:text-4xl"
            >
              Watch two agents negotiate — and hold the final switch.
            </h2>
            <p className="mt-2 text-sm leading-relaxed text-[#B8BFB2] sm:text-base">
              Pick a deal template, inspect the signed ORP wire payload, then approve, counter, or refuse at the human gate.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={handleReplay}
              className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-[#F7F5F0]/15 bg-[#181B17] px-3.5 text-xs font-semibold text-[#F7F5F0] transition-colors hover:border-[#2E7D4F] hover:bg-[#1E241C]"
            >
              <Play className="h-3.5 w-3.5 text-[#6EE7A0]" aria-hidden="true" />
              Replay wire
            </button>
            <button
              type="button"
              onClick={handleReset}
              className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-[#F7F5F0]/15 bg-[#181B17] px-3.5 text-xs font-semibold text-[#B8BFB2] transition-colors hover:border-[#F7F5F0]/30 hover:text-[#F7F5F0]"
            >
              <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
              Reset
            </button>
            <button
              type="button"
              onClick={() => setState((prev) => toggleJsonDrawer(prev))}
              aria-expanded={state.showJson}
              className={cn(
                'inline-flex min-h-10 items-center gap-1.5 rounded-xl border px-3.5 font-mono text-xs font-medium transition-colors',
                state.showJson
                  ? 'border-[#2E7D4F] bg-[#2E7D4F]/20 text-[#6EE7A0]'
                  : 'border-[#F7F5F0]/15 bg-[#181B17] text-[#B8BFB2] hover:border-[#F7F5F0]/30 hover:text-[#F7F5F0]'
              )}
            >
              <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
              View ORP JSON
              {state.showJson ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
            </button>
          </div>
        </div>

        {/* Template selector pills */}
        <div
          role="tablist"
          aria-label="ORP deal templates"
          className="mt-6 flex flex-wrap gap-2 border-b border-[#F7F5F0]/10 pb-5"
        >
          {DEAL_TEMPLATES.map((item) => {
            const active = item.id === state.templateId
            return (
              <button
                key={item.id}
                type="button"
                role="tab"
                aria-selected={active}
                data-template-pill={item.id}
                onClick={() => handleSelectTemplate(item.id)}
                className={cn(
                  'inline-flex min-h-10 items-center gap-2 rounded-full border px-4 py-1.5 text-sm font-semibold transition-all',
                  active
                    ? 'border-[#2E7D4F] bg-[#2E7D4F] text-[#F7F5F0] shadow-[0_0_20px_rgba(46,125,79,0.35)]'
                    : 'border-[#F7F5F0]/15 bg-[#181B17] text-[#B8BFB2] hover:border-[#F7F5F0]/35 hover:text-[#F7F5F0]'
                )}
              >
                <span>{item.label}</span>
                <span
                  className={cn(
                    'rounded-full px-1.5 py-0.5 font-mono text-[10px] uppercase',
                    active ? 'bg-[#111310]/30 text-[#F7F5F0]' : 'bg-[#111310] text-[#8E9687]'
                  )}
                >
                  {item.orpKind}
                </span>
              </button>
            )
          })}
        </div>

        {/* Main 2-column simulator body */}
        <div className="mt-6 grid min-w-0 gap-6 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)] lg:items-start">
          {/* Left: Named Agents + Live ORP Wire Timeline */}
          <div className="flex min-w-0 flex-col gap-4">
            {/* Parties bar */}
            <div className="rounded-2xl border border-[#F7F5F0]/10 bg-[#181B17] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">
                    {template.offerer.label}
                  </p>
                  <p className="truncate font-mono text-sm font-semibold text-[#F7F5F0]">
                    {template.offerer.handle}
                  </p>
                  <p className="font-mono text-[11px] text-[#6EE7A0]">{template.offerer.keyFingerprint}</p>
                </div>

                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-[#2E7D4F]/40 bg-[#2E7D4F]/15 text-[#6EE7A0]">
                  <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                </div>

                <div className="min-w-0 flex-1 text-right">
                  <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">
                    {template.acceptor.label}
                  </p>
                  <p className="truncate font-mono text-sm font-semibold text-[#F7F5F0]">
                    {template.acceptor.handle}
                  </p>
                  <p className="font-mono text-[11px] text-[#F59E0B]">{template.acceptor.keyFingerprint}</p>
                </div>
              </div>

              {template.extraParty ? (
                <div className="mt-3 flex items-center justify-between border-t border-[#F7F5F0]/10 pt-2.5 font-mono text-[11px] text-[#B8BFB2]">
                  <span>3rd seat · {template.extraParty.label}: {template.extraParty.handle}</span>
                  <span className="text-[#6EE7A0]">{template.extraParty.keyFingerprint}</span>
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
                  <span className="rounded-md bg-[#2E7D4F]/20 px-2 py-0.5 font-mono text-[11px] text-[#6EE7A0]">
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
                    className="inline-flex items-center gap-1.5 rounded-lg border border-[#F7F5F0]/12 bg-[#111310] px-2.5 py-1 font-mono text-[11px] text-[#B8BFB2]"
                    title={terms.termsHash}
                  >
                    <Hash className="h-3 w-3 text-[#6EE7A0]" aria-hidden="true" />
                    {shortHash(terms.termsHash)}
                  </span>
                  <button
                    type="button"
                    onClick={() => setState((prev) => toggleJsonDrawer(prev))}
                    className="inline-flex items-center gap-1 rounded-lg border border-[#F7F5F0]/12 bg-[#111310] px-2.5 py-1 font-mono text-[11px] text-[#6EE7A0] transition-colors hover:border-[#2E7D4F]"
                  >
                    <Code2 className="h-3 w-3" aria-hidden="true" />
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
                  <p className="mt-1.5 font-mono text-[11px] text-[#F59E0B]">
                    Updated terms_hash → {shortHash(template.counteredTerms.termsHash)}
                  </p>
                </TimelineStep>
              ) : null}

              {/* Step 4: Human Approval Gate (ask mode) */}
              <li
                data-human-gate
                className={cn(
                  'rounded-2xl border p-4 transition-colors',
                  isWaitingHuman
                    ? 'border-[#D97706] bg-[#D97706]/10 shadow-[0_0_24px_rgba(217,119,6,0.14)]'
                    : isRefused
                      ? 'border-red-500/40 bg-red-950/25'
                      : isApprovedFlow
                        ? 'border-[#2E7D4F]/50 bg-[#2E7D4F]/10'
                        : 'border-[#F7F5F0]/10 bg-[#181B17]/60 opacity-60'
                )}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-[#D97706]/20 font-mono text-[11px] font-bold text-[#F59E0B]">
                      {state.countered ? '04' : '03'}
                    </span>
                    <span className="inline-flex items-center gap-1.5 font-mono text-xs font-semibold uppercase tracking-wider text-[#F7F5F0]">
                      <UserCheck className="h-3.5 w-3.5 text-[#F59E0B]" aria-hidden="true" />
                      Human Approval Gate
                    </span>
                    <span className="rounded-md border border-[#D97706]/40 bg-[#D97706]/15 px-2 py-0.5 font-mono text-[10px] uppercase text-[#F59E0B]">
                      ask mode
                    </span>
                  </div>

                  {isApprovedFlow ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs font-semibold text-[#6EE7A0]">
                      <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                      Approved
                    </span>
                  ) : isRefused ? (
                    <span className="inline-flex items-center gap-1 font-mono text-xs font-semibold text-red-400">
                      <Ban className="h-3.5 w-3.5" aria-hidden="true" />
                      Refused
                    </span>
                  ) : isWaitingHuman ? (
                    <span className="font-mono text-[11px] text-[#F59E0B]">Awaiting your switch</span>
                  ) : null}
                </div>

                <p className="mt-2 text-xs leading-relaxed text-[#D4D8CE]">
                  {isWaitingHuman
                    ? 'Nothing publishes or moves until you decide. Approve to sign and execute, Counter to negotiate better terms, or Refuse to halt with zero placement.'
                    : isApprovedFlow
                      ? `Human signed acceptance for ${shortHash(terms.termsHash)}. Agents proceeded to execution.`
                      : isRefused
                        ? 'You refused this proposal. Deal halted cleanly — zero placement inserted, zero money moved.'
                        : 'Agents will pause here for human sign-off before anything touches your site.'}
                </p>

                {isWaitingHuman ? (
                  <div className="mt-4 flex flex-wrap gap-2.5">
                    <button
                      type="button"
                      data-action="approve"
                      onClick={() => handleDecision('approve')}
                      className="inline-flex min-h-10 items-center gap-1.5 rounded-xl bg-[#2E7D4F] px-4 text-sm font-semibold text-[#F7F5F0] shadow-sm transition-colors hover:bg-[#35905B]"
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
                        'inline-flex min-h-10 items-center gap-1.5 rounded-xl border px-4 text-sm font-semibold transition-colors',
                        state.countered
                          ? 'cursor-not-allowed border-[#F7F5F0]/10 bg-[#181B17] text-[#8E9687]'
                          : 'border-[#D97706]/60 bg-[#D97706]/15 text-[#FBBF24] hover:bg-[#D97706]/25'
                      )}
                    >
                      <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                      {state.countered ? 'Counter Applied' : 'Counter'}
                    </button>
                    <button
                      type="button"
                      data-action="refuse"
                      onClick={() => handleDecision('refuse')}
                      className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-red-500/35 bg-red-950/30 px-4 text-sm font-semibold text-red-300 transition-colors hover:bg-red-950/60"
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
                      ? 'border-[#2E7D4F] bg-[#2E7D4F]/14'
                      : 'border-[#F7F5F0]/15 bg-[#181B17]'
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
                    <div className="mt-3 flex flex-col gap-2.5 border-t border-[#2E7D4F]/30 pt-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#6EE7A0]">
                          <FileCheck2 className="h-4 w-4" aria-hidden="true" />
                          Verified ORP Receipt issued
                        </span>
                        <span
                          data-proof-id
                          className="rounded-md border border-[#2E7D4F]/50 bg-[#111310] px-2.5 py-1 font-mono text-xs font-semibold text-[#6EE7A0]"
                        >
                          {template.proofId}
                        </span>
                      </div>
                      <p className="break-words font-mono text-xs text-[#B8BFB2]">
                        GET /orp/proof/{template.proofId} · hash-chained &amp; signed by {template.extraParty ? 'all 3 agents' : 'both agents'}
                      </p>
                      <ul className="mt-1 grid gap-1.5 text-xs text-[#D4D8CE]">
                        {template.hardChecks.map((check) => (
                          <li key={check} className="flex items-center gap-2">
                            <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[#6EE7A0]" aria-hidden="true" />
                            <span>{check}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : (
                    <p className="mt-3 flex items-center gap-2 text-xs text-[#B8BFB2]">
                      <Zap className="h-3.5 w-3.5 text-[#6EE7A0]" aria-hidden="true" />
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
                  className="min-w-0 max-w-full overflow-hidden rounded-2xl border border-[#F7F5F0]/15 bg-[#0B0D0A]"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#F7F5F0]/10 px-4 py-2.5">
                    <span className="min-w-0 truncate font-mono text-xs text-[#6EE7A0]">
                      ORP 0.4 Wire Payload · {template.dealId}
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyJson}
                      className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-[#F7F5F0]/15 bg-[#181B17] px-2.5 py-1 font-mono text-[11px] text-[#F7F5F0] hover:border-[#2E7D4F]"
                    >
                      {copiedJson ? <Check className="h-3 w-3 text-[#6EE7A0]" /> : <Copy className="h-3 w-3" />}
                      {copiedJson ? 'Copied' : 'Copy JSON'}
                    </button>
                  </div>
                  <pre className="max-h-72 max-w-full overflow-auto p-4 font-mono text-xs leading-relaxed text-[#D4D8CE]">
                    <code>{envelopeJson}</code>
                  </pre>
                </motion.div>
              ) : null}
            </AnimatePresence>
          </div>

          {/* Right: Live Preview Artifact Panel */}
          <div className="relative min-w-0 overflow-hidden rounded-2xl border border-[#F7F5F0]/12 bg-[#181B17] p-5">
            {isReceipt && !reduced ? (
              <BorderBeam size={180} duration={8} colorFrom="#2E7D4F" colorTo="#D97706" borderWidth={1.5} />
            ) : null}

            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#F7F5F0]/10 pb-3.5">
              <span className="font-mono text-[11px] font-medium uppercase tracking-[0.12em] text-[#B8BFB2]">
                Resulting Artifact
              </span>
              {isReceipt ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-[#2E7D4F]/25 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-[#6EE7A0]">
                  <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                  Live · Verified
                </span>
              ) : isRefused ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/20 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-red-300">
                  <Ban className="h-3.5 w-3.5" aria-hidden="true" />
                  Halted · 0 Placements
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-[#D97706]/20 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-[#FBBF24]">
                  Preview · Uncommitted
                </span>
              )}
            </div>

            {isRefused ? (
              <div data-refused-preview className="mt-5 rounded-xl border border-red-500/25 bg-[#111310] p-5 text-center">
                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-red-500/15 text-red-300">
                  <Ban className="h-5 w-5" aria-hidden="true" />
                </div>
                <h3 className="mt-3 font-display text-lg font-bold text-[#F7F5F0]">
                  Zero placement. Page untouched.
                </h3>
                <p className="mt-1.5 text-sm leading-relaxed text-[#B8BFB2]">
                  Because <code className="font-mono text-xs text-[#FBBF24]">ask</code> mode gated execution, refusing the offer stopped the deal before any link or card reached your site.
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
                  className="mt-5 inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-[#F7F5F0]/20 bg-[#181B17] px-4 text-xs font-semibold text-[#F7F5F0] hover:border-[#2E7D4F]"
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
                    'rounded-xl border p-4 transition-all',
                    isReceipt
                      ? 'border-[#2E7D4F] bg-[#F7F5F0] text-[#111310]'
                      : 'border-[#F7F5F0]/15 bg-[#111310] text-[#F7F5F0]'
                  )}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span
                      className={cn(
                        'rounded-md px-2 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wider',
                        isReceipt
                          ? 'bg-[#2E7D4F]/15 text-[#2E7D4F]'
                          : 'bg-[#2E7D4F]/20 text-[#6EE7A0]'
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
                              ? 'border-[#232720]/15 bg-white text-[#111310]'
                              : 'border-[#F7F5F0]/10 bg-[#181B17] text-[#F7F5F0]'
                          )}
                        >
                          <div>
                            <p className="font-semibold">{side.publisher} · {side.issue}</p>
                            <p className={isReceipt ? 'text-[#5C6257]' : 'text-[#B8BFB2]'}>{side.target}</p>
                          </div>
                          <span className="rounded-md bg-[#2E7D4F]/20 px-2 py-0.5 font-mono text-[10px] font-semibold text-[#2E7D4F]">
                            {isReceipt ? side.verified : 'Queued'}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : null}

                  {artifact.ledgerRows ? (
                    <div className="mt-3 overflow-x-auto rounded-lg border border-current/15">
                      <table className="w-full text-left text-xs">
                        <thead>
                          <tr className={isReceipt ? 'bg-[#ECEFE2] text-[#5C6257]' : 'bg-[#181B17] text-[#8E9687]'}>
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
                      className="rounded-xl border border-[#F7F5F0]/10 bg-[#111310] p-3"
                    >
                      <p className="font-mono text-[10px] uppercase tracking-wider text-[#8E9687]">{m.label}</p>
                      <p
                        className={cn(
                          'mt-1 font-mono text-sm font-semibold tabular',
                          m.accent === 'emerald'
                            ? 'text-[#6EE7A0]'
                            : m.accent === 'amber'
                              ? 'text-[#FBBF24]'
                              : 'text-[#F7F5F0]'
                        )}
                      >
                        {m.value}
                      </p>
                    </div>
                  ))}
                </div>

                {/* Terms fingerprint footer */}
                <div className="rounded-xl border border-[#F7F5F0]/10 bg-[#111310] p-3 font-mono text-[11px] text-[#B8BFB2]">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[#8E9687]">TERMS FINGERPRINT</span>
                    <span className="text-[#6EE7A0]">{state.countered ? 'v2 (countered)' : 'v1 (initial)'}</span>
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
            ? 'border-[#D97706] bg-[#D97706]/10'
            : 'border-[#2E7D4F] bg-[#2E7D4F]/12'
          : 'border-[#F7F5F0]/10 bg-[#181B17]'
      )}
    >
      <div className="flex items-center gap-2">
        <span
          className={cn(
            'inline-flex h-6 w-6 items-center justify-center rounded-full font-mono text-[11px] font-bold',
            status === 'done'
              ? isAmber
                ? 'bg-[#D97706]/25 text-[#FBBF24]'
                : 'bg-[#2E7D4F]/25 text-[#6EE7A0]'
              : 'bg-[#111310] text-[#8E9687]'
          )}
        >
          {index}
        </span>
        <span
          className={cn(
            'font-mono text-xs font-semibold uppercase tracking-wider',
            isAmber ? 'text-[#FBBF24]' : 'text-[#6EE7A0]'
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
          ? 'border-[#2E7D4F] bg-[#2E7D4F] text-[#F7F5F0] font-semibold'
          : done
            ? 'border-[#2E7D4F]/50 bg-[#2E7D4F]/20 text-[#6EE7A0]'
            : 'border-[#F7F5F0]/15 bg-[#111310] text-[#8E9687]'
      )}
    >
      {done ? <Check className="h-3 w-3" aria-hidden="true" /> : null}
      {label}
    </span>
  )
}
