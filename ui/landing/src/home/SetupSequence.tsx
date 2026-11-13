import { useLocale } from '@/i18n'

// Illustrative path only. It does not issue a token or show a live connection.
export function SetupSequence() {
  const { t } = useLocale()
  return (
    <figure className="path" aria-labelledby="path-label">
      <figcaption id="path-label" className="path-caption">{t.connect.pathCaption}</figcaption>
      <div className="path-row">
        <div className="path-node">
          <svg className="path-mark" viewBox="0 0 48 48" aria-hidden="true">
            <circle cx="24" cy="16" r="7" fill="none" stroke="currentColor" strokeWidth="1.6" />
            <path d="M10 40c2.2-8 8-12 14-12s11.8 4 14 12" fill="none" stroke="currentColor" strokeWidth="1.6" />
          </svg>
          <p className="path-k">{t.connect.pathAgent}</p>
          <p className="path-note">{t.connect.pathAgentNote}</p>
        </div>
        <div className="path-rail" aria-hidden="true"><span /></div>
        <div className="path-node">
          <svg className="path-mark" viewBox="0 0 48 48" aria-hidden="true">
            <rect x="8" y="14" width="32" height="20" rx="4" fill="none" stroke="currentColor" strokeWidth="1.6" strokeDasharray="3 2" />
            <path d="M18 24h.1M24 24h.1M30 24h.1" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
          </svg>
          <p className="path-k">{t.connect.pathBearer}</p>
          <p className="path-note">{t.connect.pathBearerNote}</p>
          <p className="path-note">{t.connect.pathSend}</p>
        </div>
        <div className="path-rail" aria-hidden="true"><span /></div>
        <div className="path-node">
          <svg className="path-mark" viewBox="0 0 48 48" aria-hidden="true">
            <rect x="10" y="12" width="28" height="22" rx="3" fill="none" stroke="currentColor" strokeWidth="1.6" />
            <path d="M18 34v4M30 34v4M16 38h16" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          </svg>
          <p className="path-k">{t.connect.pathMcp}</p>
          <p className="path-note">{t.connect.pathWait}</p>
        </div>
      </div>
    </figure>
  )
}
