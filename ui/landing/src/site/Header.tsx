import { useEffect, useState } from 'react'
import { Pause, Play } from 'lucide-react'
import { Logo } from '@/components/Logo'
import { CommandPalette, type Command } from '@/components/CommandPalette'
import { track } from '@/lib/track'
import { signInHref, useSource, withSource } from '@/lib/source'

export type PageKey = 'home' | 'makers' | 'publishers' | 'protocol' | 'setup' | 'legal' | 'agents' | 'missing'
export const siteCommands: Command[] = [
  { id: 'home', label: 'Home', keys: 'h', href: '/' },
  { id: 'maker', label: 'Makers: every advantage', keys: 'm', href: '/makers/' },
  { id: 'publisher', label: 'Publishers: every advantage', keys: 'p', href: '/publishers/' },
  { id: 'protocol', label: 'Protocol', href: '/protocol/' },
  { id: 'agents', label: 'MCP and setup', href: '/landing-v9/agents/' },
  { id: 'faq', label: 'Money and FAQ', href: '#faq' },
  { id: 'join', label: 'Join', keys: 'j', href: '#join' },
]

// Shared header. One "Pause motion" switch stops every ambient loop on the page (WCAG 2.2.2).
export function Header({ page }: { page: PageKey }) {
  const source = useSource()
  const [paused, setPaused] = useState(false)
  useEffect(() => { document.documentElement.dataset.motion = paused ? 'paused' : 'on' }, [paused])
  const current = page === 'setup' ? 'publishers' : page
  const link = (href: string, label: string, key: PageKey, tone: string) => (
    <a href={withSource(href, source)} aria-current={current === key ? 'page' : undefined} data-track="nav_click" data-track-to={key}
      className={`inline-flex min-h-11 min-w-11 items-center rounded-md px-2.5 transition-colors duration-150 sm:px-3 ${tone} ${current === key ? 'bg-s2 font-semibold' : ''}`}>{label}</a>
  )
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-s0/90 backdrop-blur">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-x-2 gap-y-1 px-4 py-2.5 sm:px-5">
        <Logo />
        <nav aria-label="Main" className="order-last flex w-full flex-wrap gap-0.5 text-sm sm:order-none sm:w-auto">
          {link('/makers/', 'Makers', 'makers', 'text-maker hover:bg-maker-soft')}
          {link('/publishers/', 'Publishers', 'publishers', 'text-pub hover:bg-pub-soft')}
          {link('/protocol/', 'Protocol', 'protocol', 'text-t1 hover:bg-s2')}
          {link('/agents', 'Agents', 'agents', 'text-t1 hover:bg-s2')}
        </nav>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <button type="button" onClick={() => { setPaused((p) => !p); track('motion_toggle', { paused: !paused }) }} aria-pressed={paused}
            className="inline-flex min-h-11 min-w-11 items-center justify-center gap-1.5 rounded-lg border border-line bg-s1 px-2.5 text-[13px] text-t2 transition-colors duration-150 hover:bg-s2">
            {paused ? <Play className="size-3.5" aria-hidden /> : <Pause className="size-3.5" aria-hidden />}<span className="hidden sm:inline">{paused ? 'Play' : 'Pause'} motion</span><span className="sr-only sm:hidden">{paused ? 'Play' : 'Pause'} motion</span>
          </button>
          <CommandPalette commands={siteCommands.map((command) => ({ ...command, href: command.href.startsWith('#') ? command.href : withSource(command.href, source) }))} />
          <a href={signInHref('/owner', source)} data-track="nav_cta" className="inline-flex min-h-11 min-w-11 items-center rounded-lg border border-line-strong bg-s1 px-3 text-[13px] font-semibold transition-colors duration-150 hover:bg-s2">Sign in</a>
        </div>
      </div>
    </header>
  )
}
