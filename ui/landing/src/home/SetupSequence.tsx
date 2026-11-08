import { comingLine, setupSteps } from '@/home/hero-copy.mjs'

// The documented manual order. Every step is visible before the one-shot emphasis.
export function SetupSequence() {
  return (
    <figure className="setup-seq" aria-labelledby="setup-seq-label">
      <figcaption id="setup-seq-label" className="setup-seq-label">
        <span>Illustrative sequence</span>
        <span className="setup-coming">Coming</span>
      </figcaption>
      <p className="setup-coming-line">{comingLine}</p>
      <ol className="setup-steps">
        {setupSteps.map((step, index) => (
          <li key={step} className="setup-step">
            <span className="setup-index" aria-hidden="true">{index + 1}</span>
            <p>{step}</p>
          </li>
        ))}
      </ol>
    </figure>
  )
}
