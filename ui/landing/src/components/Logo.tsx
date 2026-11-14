import { useSource, withSource } from '@/lib/source'
import { brand } from '@/content'

type LogoTone = 'light' | 'dark'

// Approved icon A, "rails merge". Paths are the small-size vector.
// Rails follow the text color. The ring is the surface behind the node:
// paper on light, #1B1E18 on dark. The site itself is light, so nothing
// passes tone="dark" yet.
export function Logo({ inverse = false, tone = 'light' }: { inverse?: boolean; tone?: LogoTone }) {
  const source = useSource()
  const ink = logoInk(tone, inverse)
  return (
    <a href={withSource('/', source)} className="inline-flex min-h-11 min-w-11 items-center gap-2 font-display text-[17px] font-bold tracking-tight" aria-label={`${brand}, home`}>
      <svg width="22" height="22" viewBox="0 0 32 32" aria-hidden="true">
        <g fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
          <path d="M2.4 9.6 C 8.8 9.6 11.6 9.6 14.8 15.2" strokeWidth="3.6" />
          <path d="M2.4 22.4 C 8.8 22.4 11.6 22.4 14.8 16.8" strokeWidth="3.6" />
          <path d="M14.2 16 H 29.4" strokeWidth="3.8" />
        </g>
        <circle cx="17.2" cy="16" r="5" fill={ink.plate} />
        <circle cx="17.2" cy="16" r="4.15" fill={ink.node} />
      </svg>
      {brand}
    </a>
  )
}

function logoInk(tone: LogoTone, inverse: boolean) {
  switch (tone) {
    case 'light':
      return {
        plate: inverse ? 'rgb(var(--rgb-surface-inv))' : 'rgb(var(--rgb-surface-0))',
        node: 'rgb(var(--rgb-maker-brand))',
      }
    case 'dark':
      return { plate: '#1B1E18', node: '#D85A42' }
    default: {
      const unexpected: never = tone
      return unexpected
    }
  }
}
