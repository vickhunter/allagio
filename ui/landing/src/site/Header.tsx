import { useEffect, useRef, useState } from 'react'
import { Logo } from '@/components/Logo'
import { CommandPalette, type Command } from '@/components/CommandPalette'
import { signInHref, useSource, withSource } from '@/lib/source'
import { useCaseLinks } from '@/use-cases/links.mjs'
import { ChevronDown } from 'lucide-react'

export type PageKey = 'home' | 'makers' | 'publishers' | 'protocol' | 'setup' | 'legal' | 'agents' | 'you' | 'use-cases' | 'reference' | 'missing'
export const siteCommands: Command[] = [
  { id: 'home', label: 'Home', keys: 'h', href: '/' },
  { id: 'maker', label: 'Makers: every advantage', keys: 'm', href: '/makers/' },
  { id: 'publisher', label: 'Publishers: every advantage', keys: 'p', href: '/publishers/' },
  { id: 'protocol', label: 'Protocol', href: '/protocol/' },
  { id: 'you', label: 'Your limits', href: '/you/' },
  { id: 'agents', label: 'MCP and setup', href: '/landing-v9/agents/' },
  { id: 'faq', label: 'Money and FAQ', href: '#faq' },
  { id: 'join', label: 'Join', keys: 'j', href: '#join' },
]

// Shared header.
export function Header({ page }: { page: PageKey }) {
  const source = useSource()
  const [open, setOpen] = useState(false)
  const menuRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setOpen(false)
      menuRef.current?.focus()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open])
  const current = page === 'setup' ? 'publishers' : page
  const link = (href: string, label: string, key: PageKey, tone: string) => (
    <a href={withSource(href, source)} aria-current={current === key ? 'page' : undefined} data-track="nav_click" data-track-to={key}
      className={`inline-flex min-h-11 min-w-11 items-center rounded-md px-2.5 transition-colors duration-150 sm:px-3 ${tone} ${current === key ? 'bg-s2 font-semibold' : ''}`}>{label}</a>
  )
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-white">
      <div className="site-bar">
        <Logo />
        <button ref={menuRef} type="button" className="site-menu-btn" aria-expanded={open} aria-controls="site-menu" onClick={() => setOpen((value) => !value)}>
          {open ? 'Close menu' : 'Menu'}
        </button>
        <nav id="site-menu" aria-label="Main" className={open ? 'site-menu is-open text-sm' : 'site-menu text-sm'}>
          {link('/you/', 'Your limits', 'you', 'text-t1 hover:bg-s2')}
          {link('/makers/', 'Makers', 'makers', 'text-maker hover:bg-maker-soft')}
          {link('/publishers/', 'Publishers', 'publishers', 'text-pub hover:bg-pub-soft')}
          <div className="group relative">
            <a href={withSource('/use-cases/', source)} aria-current={current === 'use-cases' ? 'page' : undefined} data-track="nav_click" data-track-to="use-cases"
              className={`inline-flex min-h-11 min-w-11 items-center gap-1 rounded-md px-2.5 text-t1 transition-colors duration-150 hover:bg-s2 sm:px-3 ${current === 'use-cases' ? 'bg-s2 font-semibold' : ''}`}>
              Use cases
              <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 text-t2 transition-transform duration-150 group-hover:rotate-180 group-focus-within:rotate-180" />
            </a>
            <div className="invisible absolute left-0 top-full z-40 hidden w-64 translate-y-1 rounded-xl border border-line bg-white p-1.5 opacity-0 shadow-lg transition-all duration-150 group-focus-within:visible group-focus-within:translate-y-0 group-focus-within:opacity-100 group-hover:visible group-hover:translate-y-0 group-hover:opacity-100 md:block">
              {useCaseLinks.map((item) => (
                <a
                  key={item.id}
                  href={withSource(`/use-cases/#${item.id}`, source)}
                  data-track="nav_click"
                  data-track-to={`use-cases-${item.id}`}
                  className="block rounded-lg px-3 py-2 text-[13px] font-medium text-t1 transition-colors hover:bg-s2"
                >
                  {item.title}
                </a>
              ))}
            </div>
          </div>
          {link('/protocol/', 'Protocol', 'protocol', 'text-t1 hover:bg-s2')}
          {link('/reference', 'Reference', 'reference', 'text-t1 hover:bg-s2')}
          {link('/agents', 'Agents', 'agents', 'text-t1 hover:bg-s2')}
        </nav>
        <div className="flex shrink-0 items-center gap-1.5">
          <CommandPalette commands={siteCommands.map((command) => ({ ...command, href: command.href.startsWith('#') ? command.href : withSource(command.href, source) }))} />
          <a href={signInHref('/owner', source)} data-track="nav_cta" className="inline-flex min-h-11 min-w-11 items-center rounded-full bg-t1 px-3.5 text-[13px] font-semibold text-s0">Sign in</a>
        </div>
      </div>
    </header>
  )
}
