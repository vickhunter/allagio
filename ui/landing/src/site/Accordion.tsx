import { useId, useState } from 'react'
import { track } from '@/lib/track'
// PostHog-style accordion: the panel animates grid-template-rows 0fr -> 1fr over 300ms (--dur-panel).
export function Accordion({ items, tone = 'neutral' }: { items: { q: string; a: string }[]; tone?: 'neutral' | 'maker' | 'pub' }) {
  const [open, setOpen] = useState<number | null>(0)
  const uid = useId()
  const mark = tone === 'maker' ? 'text-maker' : tone === 'pub' ? 'text-pub' : 'text-t2'
  return (
    <div className="divide-y divide-line border-y border-line">
      {items.map((f, n) => {
        const isOpen = open === n
        return (
          <div key={f.q}>
            <h3>
              <button type="button" id={`${uid}-b${n}`} aria-expanded={isOpen} aria-controls={`${uid}-p${n}`}
                onClick={() => { setOpen(isOpen ? null : n); if (!isOpen) track('faq_open', { q: f.q }) }}
                className="flex min-h-12 w-full items-center justify-between gap-4 py-4 text-left font-semibold transition-colors duration-150 hover:text-setup">
                {f.q}
                <span aria-hidden className={`font-mono text-lg ${mark} transition-transform duration-200 ${isOpen ? 'rotate-45' : ''}`}>+</span>
              </button>
            </h3>
            <div id={`${uid}-p${n}`} role="region" aria-labelledby={`${uid}-b${n}`} className="acc" data-open={isOpen} inert={!isOpen}>
              <div className="overflow-hidden"><p className="max-w-prose pb-5 leading-relaxed text-t2">{f.a}</p></div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
