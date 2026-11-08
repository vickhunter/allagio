import {
  deskAsk,
  deskComing,
  deskContent,
  deskLabel,
  deskMcp,
  deskOffer,
  deskPlacement,
  deskWorks,
  fixtureCommission,
  fixtureName,
  fixtureSummary,
} from '@/home/desk-copy.mjs'

// A post with the fixture offer sitting in it. Text stays readable before motion ends.
export function DistributionDesk() {
  return (
    <figure className="desk" aria-labelledby="desk-label">
      <figcaption id="desk-label" className="desk-caption">
        <span className="desk-label">{deskLabel}</span>
        <span className="desk-chip">Works today</span>
        <span className="desk-chip desk-chip-wait">Coming</span>
      </figcaption>
      <div className="desk-stage">
        <div className="desk-offer">
          <svg className="desk-mark" viewBox="0 0 48 36" aria-hidden="true">
            <rect x="8" y="4" width="32" height="28" rx="2" fill="#f7f7ef" stroke="#232720" strokeWidth="1.5" />
            <path d="M16 4v28" stroke="#3f5c35" strokeWidth="2" />
            <path d="M21 12h14M21 18h14M21 24h9" stroke="#232720" strokeWidth="1.2" opacity=".45" />
          </svg>
          <div className="desk-offer-copy">
            <p className="desk-k">Offer</p>
            <p className="desk-offer-name">{fixtureName}</p>
            <p className="desk-body">{fixtureSummary}</p>
            <p className="desk-body">{fixtureCommission}</p>
          </div>
          <p className="sr-only">{deskOffer}</p>
        </div>
        <div className="desk-rail" aria-hidden="true"><span /></div>
        <article className="desk-article">
          <p className="desk-k">Post</p>
          <p className="desk-article-title">{deskContent}</p>
          <div className="desk-bars" aria-hidden="true"><span /><span /><span /></div>
          <div className="desk-inset">
            <p className="desk-k">Placement</p>
            <p className="desk-inset-name">{fixtureName}</p>
            <p className="desk-body">{deskPlacement}</p>
          </div>
          <div className="desk-bars desk-bars-tail" aria-hidden="true"><span /><span /></div>
        </article>
      </div>
      <p className="desk-coming">{deskComing}</p>
      <p className="desk-status-line">{deskWorks}</p>
      <p className="desk-note">{deskAsk}</p>
      <p className="desk-note">{deskMcp}</p>
    </figure>
  )
}
