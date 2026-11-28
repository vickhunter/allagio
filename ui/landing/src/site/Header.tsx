import { useEffect, useMemo, useRef, useState } from 'react'
import { Logo } from '@/components/Logo'
import { CommandPalette, type Command } from '@/components/CommandPalette'
import { signInHref, useSource, withSource } from '@/lib/source'
import { LangSwitcher } from '@/i18n/LangSwitcher'
import { useLocale } from '@/i18n'
import { ChevronDown } from 'lucide-react'

export type PageKey = 'home' | 'makers' | 'publishers' | 'protocol' | 'setup' | 'legal' | 'agents' | 'you' | 'use-cases' | 'reference' | 'missing'
export const siteCommands: Command[] = [
  { id: 'home', label: 'Home', keys: 'h', href: '/' },
  { id: 'maker', label: 'For makers', keys: 'm', href: '/makers/' },
  { id: 'publisher', label: 'For publishers', keys: 'p', href: '/publishers/' },
  { id: 'protocol', label: 'Protocol', href: '/protocol/' },
  { id: 'you', label: 'Approval rules', href: '/#approval-rules' },
  { id: 'reference', label: 'Reference', href: '/reference' },
  { id: 'agents', label: 'MCP and setup', href: '/landing-v9/agents/' },
  { id: 'faq', label: 'Money and FAQ', href: '#faq' },
  { id: 'join', label: 'Join', keys: 'j', href: '#join' },
]

// Shared header. Centered 3-zone bar: logo left, nav centered on wide screens,
// sign-in + primary CTA right. Below 1280px the destinations sit in a labelled menu.
export function Header({ page }: { page: PageKey }) {
  const source = useSource()
  const { t } = useLocale()
  const [open, setOpen] = useState(false)
  const menuRef = useRef<HTMLButtonElement>(null)
  const navRef = useRef<HTMLElement>(null)
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setOpen(false)
      menuRef.current?.focus()
    }
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null
      if (navRef.current?.contains(target) || menuRef.current?.contains(target)) return
      setOpen(false)
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('pointerdown', onPointerDown)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('pointerdown', onPointerDown)
    }
  }, [open])
  const current = page === 'setup' ? 'publishers' : page
  const commands = useMemo(
    () => siteCommands.map((command) => ({ ...command, label: t.chrome.commands[command.id as keyof typeof t.chrome.commands] ?? command.label })),
    [t]
  )
  const link = (href: string, label: string, key: PageKey) => (
    <a href={withSource(href, source)} aria-current={current === key ? 'page' : undefined} data-track="nav_click" data-track-to={key}
      className={`inline-flex min-h-11 min-w-11 items-center whitespace-nowrap rounded-full px-3 text-t1 transition-colors duration-150 hover:bg-s2 ${current === key ? 'bg-s2 font-semibold' : ''}`}>{label}</a>
  )
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-s0">
      <div className="site-bar">
        <Logo />
        <button ref={menuRef} type="button" className="site-menu-btn" aria-expanded={open} aria-controls="site-menu" onClick={() => setOpen((value) => !value)}>
          {open ? t.chrome.menuClose : t.chrome.menu}
        </button>
        <nav ref={navRef} id="site-menu" aria-label="Main" className={open ? 'site-menu is-open text-sm' : 'site-menu text-sm'} onClick={(event) => { if ((event.target as HTMLElement).closest('a')) setOpen(false) }}>
          {link('/makers/', t.chrome.navMakers, 'makers')}
          {link('/publishers/', t.chrome.navPublishers, 'publishers')}
          <div className="group relative">
            <a href={withSource('/use-cases/', source)} aria-current={current === 'use-cases' ? 'page' : undefined} data-track="nav_click" data-track-to="use-cases"
              className={`inline-flex min-h-11 min-w-11 items-center gap-1 whitespace-nowrap rounded-full px-3 text-t1 transition-colors duration-150 hover:bg-s2 ${current === 'use-cases' ? 'bg-s2 font-semibold' : ''}`}>
              {t.chrome.navUseCases}
              <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 text-t2 transition-transform duration-150 group-hover:rotate-180 group-focus-within:rotate-180" />
            </a>
            <div className="site-menu-submenu mt-1 w-full rounded-xl border border-line bg-s1 p-1.5 shadow-sm xl:invisible xl:absolute xl:left-1/2 xl:top-full xl:z-40 xl:mt-0 xl:w-64 xl:-translate-x-1/2 xl:translate-y-1 xl:opacity-0 xl:shadow-lg xl:transition-all xl:duration-150 xl:group-focus-within:visible xl:group-focus-within:translate-y-0 xl:group-focus-within:opacity-100 xl:group-hover:visible xl:group-hover:translate-y-0 xl:group-hover:opacity-100">
              {t.chrome.useCaseLinks.map((item) => (
                <a
                  key={item.id}
                  href={withSource(`/use-cases/#${item.id}`, source)}
                  data-track="nav_click"
                  data-track-to={`use-cases-${item.id}`}
                  className="site-menu-sub-link block rounded-lg px-3 py-2 text-[13px] font-medium text-t1 transition-colors hover:bg-s2"
                >
                  {item.title}
                </a>
              ))}
            </div>
          </div>
          {link('/protocol/', t.chrome.navProtocol, 'protocol')}
          <span className="xl:hidden">{link('/reference', t.chrome.navReference, 'reference')}</span>
          <span className="sm:hidden">
            <a href={signInHref('/owner', source)} data-track="nav_signin"
              className="inline-flex min-h-11 min-w-11 items-center rounded-full px-3 font-semibold text-t1 transition-colors duration-150 hover:bg-s2">{t.chrome.signIn}</a>
          </span>
          <span className="sm:hidden mt-1 flex px-3">
            <LangSwitcher />
          </span>
        </nav>
        <div className="flex min-w-0 items-center gap-1.5 sm:shrink-0 sm:gap-2">
          <LangSwitcher className="hidden sm:inline-flex" />
          <CommandPalette commands={commands.map((command) => ({ ...command, href: command.href.startsWith('#') ? command.href : withSource(command.href, source) }))} />
          <a href={signInHref('/owner', source)} data-track="nav_signin" className="hidden min-h-11 items-center whitespace-nowrap rounded-full px-3 text-[13px] font-semibold text-t2 transition-colors duration-150 hover:bg-s2 hover:text-t1 sm:inline-flex">{t.chrome.signIn}</a>
          <a href={withSource('/landing-v9/agents/', source)} data-track="nav_cta" className="inline-flex min-h-11 items-center rounded-full bg-t1 px-2.5 text-center text-[13px] leading-tight font-semibold text-s0 transition-[transform,box-shadow] duration-150 hover:-translate-y-px hover:shadow-md sm:px-4 sm:whitespace-nowrap">{t.chrome.setupCta}</a>
        </div>
      </div>
    </header>
  )
}
