import {
  deskAsk,
  deskComing,
  deskContent,
  deskLabel,
  deskMcp,
  deskOffer,
  deskPlacement,
  deskWorks,
} from '@/home/desk-copy.mjs'

const steps = [
  { k: 'Offer', body: deskOffer },
  { k: 'Content', body: deskContent },
  { k: 'Placement', body: deskPlacement },
] as const

// Original offer → content → placement stage. Cards stay readable before motion ends.
export function DistributionDesk() {
  return (
    <figure className="desk" aria-labelledby="desk-label">
      <figcaption id="desk-label" className="desk-caption">
        <span className="desk-label">{deskLabel}</span>
        <span className="desk-chip">Works today</span>
      </figcaption>
      <p className="desk-status-line">{deskWorks}</p>
      <p className="desk-coming"><span className="desk-chip desk-chip-wait">Coming</span> {deskComing.replace(/^Coming\.\s*/, '')}</p>
      <ol className="desk-steps">
        {steps.map((step) => (
          <li key={step.k} className="desk-step">
            <p className="desk-k">{step.k}</p>
            <p className="desk-body">{step.body}</p>
          </li>
        ))}
      </ol>
      <p className="desk-note">{deskAsk}</p>
      <p className="desk-note">{deskMcp}</p>
    </figure>
  )
}
