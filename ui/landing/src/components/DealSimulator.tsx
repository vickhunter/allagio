import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import {
  ArrowLeftRight,
  Check,
  ChevronDown,
  ChevronUp,
  Code2,
  Copy,
  Play,
  RotateCcw,
  ShieldCheck,
  XCircle,
} from 'lucide-react'
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
  type DealTemplateId,
  type HumanDecision,
  type PreviewArtifact,
  type SimulatorState,
} from './deal-simulator-data'

export interface DealSimulatorProps {
  initialTemplate?: DealTemplateId
  className?: string
}

const STEP_DELAY_MS = 650

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false)
  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)')
    setReduced(mq.matches)
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])
  return reduced
}

function ArtifactPreview({ artifact, refused }: { artifact: PreviewArtifact; refused: boolean }) {
  if (refused) {
    return (
      <div className="rounded-lg border border-dashed border-line-strong bg-s0 p-5 text-center">
        <XCircle aria-hidden="true" className="mx-auto h-5 w-5 text-[#991B1B]" />
        <p className="mt-2 text-sm font-semibold text-t1">No placement inserted</p>
        <p className="mt-1 text-xs leading-relaxed text-t2">
          Your agent sent a signed refusal on ORP. Zero copy published, zero tracking links created.
        </p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-line bg-s0 p-4 shadow-2xs">
      <div className="flex items-center justify-between gap-2 border-b border-line pb-2.5 text-xs">
        <span className="font-medium text-t2">{artifact.badge}</span>
        <span className="truncate font-mono text-[11px] text-t2">{artifact.trackedUrl}</span>
      </div>

      <p className="mt-3 font-display text-[15px] font-bold leading-snug text-t1">{artifact.title}</p>
      {artifact.subtitle ? (
        <p className="mt-0.5 text-xs text-t2">{artifact.subtitle}</p>
      ) : null}
      <p className="mt-2 text-sm leading-relaxed text-t2">{artifact.body}</p>

      {artifact.swapSides && artifact.swapSides.length > 0 ? (
        <div className="mt-3 divide-y divide-line border-t border-line pt-2 text-xs">
          {artifact.swapSides.map((side) => (
            <div key={side.publisher} className="flex items-center justify-between gap-2 py-1.5">
              <span className="inline-flex items-center gap-1.5 font-medium text-t1">
                <ArrowLeftRight aria-hidden="true" className="h-3.5 w-3.5 text-t2" />
                {side.publisher}
              </span>
              <span className="truncate text-t2">{side.issue}</span>
            </div>
          ))}
        </div>
      ) : null}

      {artifact.ledgerRows && artifact.ledgerRows.length > 0 ? (
        <dl className="mt-3 divide-y divide-line border-t border-line pt-2 text-xs">
          {artifact.ledgerRows.map((row) => (
            <div key={row.party} className="flex items-center justify-between gap-2 py-1.5">
              <dt className="font-mono text-t2">{row.party}</dt>
              <dd className="tabular font-semibold text-t1">
                {row.share} · {row.payout}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}

      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-2.5 text-xs text-t2">
        <span>{artifact.disclosureLabel}</span>
      </div>
    </div>
  )
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
  const focusRing = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#111111]'

  const stageLabel = isReceipt
    ? 'Simulated · Verified in demo'
    : isRefused
      ? 'Refused · Zero placement'
      : isWaitingHuman
        ? 'Waiting for human approval'
        : state.stage === 'execute'
          ? 'Executing in demo'
          : state.stage === 'active'
            ? 'Signed · Active'
            : 'Negotiating'

  return (
    <div
      data-deal-simulator=""
      data-template={template.id}
      data-stage={state.stage}
      className={`overflow-hidden rounded-xl border border-line bg-s0 text-t1 shadow-xs ${className ?? ''}`}
    >
      <div role="status" aria-live="polite" className="sr-only">
        {statusAnnouncement}
      </div>

      {/* Top bar: underline template tabs + quiet controls */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-s1 px-4 sm:px-6">
        <div
          role="tablist"
          aria-label="Deal templates"
          className="-mb-px flex flex-wrap items-center gap-1"
        >
          {DEAL_TEMPLATES.map((item, index) => {
            const active = item.id === template.id
            return (
              <button
                key={item.id}
                ref={(el) => {
                  tabRefs.current[item.id] = el
                }}
                id={`sim-tab-${item.id}`}
                type="button"
                role="tab"
                tabIndex={active ? 0 : -1}
                aria-selected={active}
                aria-controls="sim-tabpanel"
                onClick={() => handleSelectTemplate(item.id)}
                onKeyDown={(e) => onTemplateTabKeyDown(e, index)}
                className={`inline-flex min-h-12 cursor-pointer items-center border-b-2 px-3 text-sm transition-colors ${focusRing} ${
                  active
                    ? 'border-t1 font-semibold text-t1'
                    : 'border-transparent font-medium text-t2 hover:text-t1'
                }`}
              >
                {item.label}
              </button>
            )
          })}
        </div>

        <div className="flex flex-wrap items-center gap-1.5 py-2">
          <span className="mr-1 rounded-md bg-s2 px-2 py-1 text-xs font-medium text-t2">
            Simulator
          </span>
          <button
            type="button"
            onClick={handleReplay}
            className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium text-t2 transition-colors hover:bg-s2 hover:text-t1 active:scale-[0.98] ${focusRing}`}
          >
            <Play aria-hidden="true" className="h-3.5 w-3.5" />
            Replay
          </button>
          <button
            type="button"
            onClick={handleReset}
            className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium text-t2 transition-colors hover:bg-s2 hover:text-t1 active:scale-[0.98] ${focusRing}`}
          >
            <RotateCcw aria-hidden="true" className="h-3.5 w-3.5" />
            Reset
          </button>
          <button
            type="button"
            onClick={() => setState((prev) => ({ ...prev, showJson: !prev.showJson }))}
            aria-expanded={state.showJson}
            className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg border border-line bg-s0 px-3 text-xs font-medium text-t1 transition-colors hover:bg-s2 active:scale-[0.98] ${focusRing}`}
          >
            <Code2 aria-hidden="true" className="h-3.5 w-3.5 text-t2" />
            ORP JSON
            {state.showJson ? (
              <ChevronUp aria-hidden="true" className="h-3.5 w-3.5 text-t2" />
            ) : (
              <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 text-t2" />
            )}
          </button>
        </div>
      </div>

      {/* Main split workbench: hairline-divided wire timeline left, placement & receipt right */}
      <div
        id="sim-tabpanel"
        role="tabpanel"
        aria-labelledby={`sim-tab-${template.id}`}
        className="grid divide-y divide-line lg:grid-cols-12 lg:divide-x lg:divide-y-0"
      >
        {/* Left: flat hairline timeline (zero nested cards) */}
        <div className="divide-y divide-line lg:col-span-7">
          {/* Counterparties */}
          <div className="grid grid-cols-2 gap-4 px-5 py-4 sm:px-6">
            <div className="min-w-0">
              <p className="text-xs font-medium text-t2">{template.offerer.label}</p>
              <p className="mt-1 truncate font-mono text-sm font-medium text-t1">{template.offerer.handle}</p>
              <p className="truncate font-mono text-[11px] text-t2">{template.offerer.keyFingerprint}</p>
            </div>
            <div className="min-w-0">
              <p className="text-xs font-medium text-t2">{template.acceptor.label}</p>
              <p className="mt-1 truncate font-mono text-sm font-medium text-t1">{template.acceptor.handle}</p>
              <p className="truncate font-mono text-[11px] text-t2">{template.acceptor.keyFingerprint}</p>
            </div>
          </div>

          {/* Discover */}
          <div className="px-5 py-4 sm:px-6">
            <div className="flex items-baseline justify-between gap-3">
              <p className="text-xs font-semibold text-t1">Discover</p>
              <span className="text-xs text-t2">{template.discoverScore}</span>
            </div>
            <p className="mt-1 text-sm text-t2">{template.discoverMatch}</p>
          </div>

          {/* Offer / Counter */}
          <div className="px-5 py-4 sm:px-6">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <p className="text-xs font-semibold text-t1">
                {state.countered ? 'Counteroffer proposed' : 'Offer proposed'}
              </p>
              <span className="font-mono text-[11px] text-t2">{terms.termsHash}</span>
            </div>
            <p className="mt-1 text-sm font-semibold text-t1">{terms.headline}</p>
            <p className="mt-0.5 text-xs text-t2">
              {terms.rateLabel} · {terms.windowLabel}
            </p>
            {state.countered ? (
              <p className="mt-2 text-xs font-medium text-maker">{template.counterNote}</p>
            ) : null}
          </div>

          {/* Human approval gate */}
          <div className="bg-s1 px-5 py-5 sm:px-6">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <ShieldCheck aria-hidden="true" className="h-4 w-4 text-maker-brand" />
                <p className="text-sm font-semibold text-t1">Human approval gate</p>
              </div>
              <span className="text-xs font-medium text-t2">
                {isApprovedFlow ? 'Approved' : isRefused ? 'Refused' : 'Ask mode'}
              </span>
            </div>

            {isWaitingHuman ? (
              <>
                <p className="mt-2 text-sm leading-relaxed text-t2">
                  Both agents agree on the terms hash. Nothing is published or placed until you decide.
                </p>
                <div className="mt-4 flex flex-wrap items-center gap-2.5">
                  <button
                    type="button"
                    data-sim-action="approve"
                    onClick={() => handleDecision('approve')}
                    className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg bg-t1 px-4 text-sm font-semibold text-s0 transition-[background-color,transform] duration-150 hover:bg-t1/90 active:scale-[0.98] ${focusRing}`}
                  >
                    <Check aria-hidden="true" className="h-4 w-4" />
                    Approve Deal
                  </button>
                  {!state.countered ? (
                    <button
                      type="button"
                      data-sim-action="counter"
                      onClick={() => handleDecision('counter')}
                      className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg border border-line-strong bg-s0 px-4 text-sm font-medium text-t1 transition-colors hover:bg-s2 active:scale-[0.98] ${focusRing}`}
                    >
                      Counter terms
                    </button>
                  ) : null}
                  <button
                    type="button"
                    data-sim-action="refuse"
                    onClick={() => handleDecision('refuse')}
                    className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg px-3 text-sm font-medium text-[#991B1B] transition-colors hover:bg-[#FEF2F2] active:scale-[0.98] ${focusRing}`}
                  >
                    Refuse
                  </button>
                </div>
              </>
            ) : isApprovedFlow ? (
              <div className="mt-2 space-y-2 text-sm">
                <p className="text-t2">
                  Human signed acceptance for <code className="font-mono text-xs text-t1">{terms.termsHash}</code>.
                </p>
                {isReceipt ? (
                  <div className="border-t border-line pt-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="text-xs font-semibold text-[#166534]">Verified ORP receipt issued</span>
                      <code className="font-mono text-xs text-t2">{template.proofId}</code>
                    </div>
                    <ul className="mt-2 space-y-1 text-xs text-t2">
                      {template.hardChecks.map((check) => (
                        <li key={check} className="flex items-center gap-2">
                          <Check aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-[#166534]" />
                          <span>{check}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </div>
            ) : isRefused ? (
              <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-t2">Deal refused. Zero placement inserted and zero money moved.</p>
                <button
                  type="button"
                  onClick={handleReset}
                  className={`inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg border border-line-strong bg-s0 px-3.5 text-xs font-semibold text-t1 hover:bg-s2 active:scale-[0.98] ${focusRing}`}
                >
                  <RotateCcw aria-hidden="true" className="h-3.5 w-3.5" />
                  Try again
                </button>
              </div>
            ) : (
              <p className="mt-2 text-xs text-t2">Advancing to human approval gate…</p>
            )}
          </div>
        </div>

        {/* Right: resulting placement & terms summary */}
        <div className="flex flex-col justify-between bg-s1 p-5 sm:p-6 lg:col-span-5">
          <div>
            <div className="flex flex-wrap items-center justify-between gap-2 pb-3.5">
              <p className="text-xs font-semibold text-t1">Resulting placement</p>
              <span
                className={`inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium ${
                  isReceipt
                    ? 'bg-[#DCFCE7] text-[#166534]'
                    : isRefused
                      ? 'bg-[#FEE2E2] text-[#991B1B]'
                      : 'bg-s2 text-t2'
                }`}
              >
                {stageLabel}
              </span>
            </div>

            <ArtifactPreview artifact={artifact} refused={isRefused} />
          </div>

          <dl className="mt-5 grid grid-cols-3 divide-x divide-line border-t border-line pt-4 text-left">
            {(isRefused
              ? [
                  { label: 'Status', value: 'Refused' },
                  { label: 'Placement', value: 'None' },
                  { label: 'Cash moved', value: '$0' },
                ]
              : artifact.metrics
            ).map((m, idx) => (
              <div key={m.label} className={idx > 0 ? 'pl-3' : ''}>
                <dt className="text-[11px] font-medium text-t2">{m.label}</dt>
                <dd className="tabular mt-0.5 text-sm font-semibold text-t1">{m.value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>

      {/* Collapsible ORP JSON drawer */}
      {state.showJson ? (
        <div className="border-t border-line bg-[#111111] p-5 text-[#F5F5F5]">
          <div className="flex flex-wrap items-center justify-between gap-2 pb-3">
            <span className="font-mono text-xs text-[#A3A3A3]">
              ORP 0.4 envelope · {template.dealId}
            </span>
            <button
              type="button"
              onClick={handleCopyJson}
              className="inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg border border-[#333333] bg-[#1C1C1C] px-3 text-xs font-medium text-[#F5F5F5] hover:bg-[#262626] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#FFFFFF]"
            >
              {copiedJson ? <Check aria-hidden="true" className="h-3.5 w-3.5" /> : <Copy aria-hidden="true" className="h-3.5 w-3.5" />}
              {copiedJson ? 'Copied' : 'Copy JSON'}
            </button>
          </div>
          <pre className="max-h-72 overflow-auto rounded-lg border border-[#262626] bg-[#0A0A0A] p-4 font-mono text-xs leading-relaxed text-[#E5E5E5]">
            <code>{envelopeJson}</code>
          </pre>
        </div>
      ) : null}
    </div>
  )
}
