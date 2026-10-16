import { useSource, withSource } from '@/lib/source'

export function Logo({ inverse = false }: { inverse?: boolean }) {
  const source = useSource()
  return (
    <a href={withSource('/', source)} className="inline-flex min-h-11 min-w-11 items-center gap-2 font-display text-[17px] font-bold tracking-tight" aria-label="Offer Rails, home">
      <svg width="26" height="18" viewBox="0 0 26 18" aria-hidden="true">
        <path d="M1 6h24M1 12h24" stroke="currentColor" strokeWidth="2" />
        {[4, 10, 16, 22].map((x) => <path key={x} d={`M${x} 3v12`} stroke="currentColor" strokeWidth="1.5" opacity=".5" />)}
        <circle cx="17" cy="9" r="4" fill="rgb(var(--rgb-maker-brand))" stroke={inverse ? 'rgb(var(--rgb-surface-inv))' : 'rgb(var(--rgb-surface-0))'} strokeWidth="1.5" />
      </svg>
      Offer Rails
    </a>
  )
}
