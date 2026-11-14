import { useSource, withSource } from '@/lib/source'
import { brand } from '@/content'

// Approved icon A, "rails merge". Paths are the small-size vector
// (a-rails-merge-favicon.svg). Strokes follow the text color so the
// header and footer stay on the page theme. The wordmark text stays.
export function Logo({ inverse = false }: { inverse?: boolean }) {
  const source = useSource()
  const plate = inverse ? 'rgb(var(--rgb-surface-inv))' : 'rgb(var(--rgb-surface-0))'
  return (
    <a href={withSource('/', source)} className="inline-flex min-h-11 min-w-11 items-center gap-2 font-display text-[17px] font-bold tracking-tight" aria-label={`${brand}, home`}>
      <svg width="22" height="22" viewBox="0 0 32 32" aria-hidden="true">
        <g fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
          <path d="M2.4 9.6 C 8.8 9.6 11.6 9.6 14.8 15.2" strokeWidth="3.6" />
          <path d="M2.4 22.4 C 8.8 22.4 11.6 22.4 14.8 16.8" strokeWidth="3.6" />
          <path d="M14.2 16 H 29.4" strokeWidth="3.8" />
        </g>
        <circle cx="17.2" cy="16" r="5" fill={plate} />
        <circle cx="17.2" cy="16" r="4.15" fill="rgb(var(--rgb-maker-brand))" />
      </svg>
      {brand}
    </a>
  )
}
