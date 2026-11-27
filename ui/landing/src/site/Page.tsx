import { useEffect, type ReactNode } from 'react'
import { Header, type PageKey } from './Header'
import { Footer } from './Footer'
import { useLocale } from '@/i18n'
export function Page({ page, children }: { page: PageKey; children: ReactNode }) {
  const { t } = useLocale()
  useEffect(() => {
    if (!window.location.hash) return
    let id = window.location.hash.slice(1)
    try { id = decodeURIComponent(id) } catch { /* use the raw fragment */ }
    const scroll = () => {
      const target = document.getElementById(id)
      if (!target) return
      const top = target.getBoundingClientRect().top + window.scrollY - 84
      window.scrollTo({ top: Math.max(0, top) })
    }
    scroll()
    const frame = window.requestAnimationFrame(scroll)
    const timeout = window.setTimeout(scroll, 400)
    window.addEventListener('load', scroll, { once: true })
    return () => {
      window.cancelAnimationFrame(frame)
      window.clearTimeout(timeout)
      window.removeEventListener('load', scroll)
    }
  }, [])
  return (
    <div id="top" className="min-h-screen">
      <a href="#main" className="skip sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-50 focus:inline-flex focus:min-h-11 focus:items-center focus:rounded-md focus:bg-s1 focus:px-3">{t.chrome.skipToContent}</a>
      <Header page={page} />
      <main id="main">{children}</main>
      <Footer />
    </div>
  )
}
