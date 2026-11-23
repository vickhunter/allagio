import { useEffect, useRef } from 'react'
import { Logo } from '@/components/Logo'
import { Blip } from '@/components/Blip'
import { Kbd } from '@/components/ui/input'
import { Skyline, Tram } from './Padova'
import { useSource, withSource } from '@/lib/source'
import { useLocale } from '@/i18n'

// Footer on every page: hand-drawn Padova skyline (own SVG, no copied assets) + one SIR1 tram.
// Tram moves with CSS transform only; paused off-screen (IntersectionObserver); animation:none under reduced motion.
export function Footer() {
  const source = useSource()
  const { t } = useLocale()
  const scene = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = scene.current
    if (!el || !('IntersectionObserver' in window)) return
    const io = new IntersectionObserver(([e]) => { el.dataset.onscreen = String(e.isIntersecting) }, { rootMargin: '40px' })
    io.observe(el)
    return () => io.disconnect()
  }, [])
  return (
    <footer className="mt-0 bg-[rgb(var(--rgb-footer))] text-t1" aria-labelledby="footer-h">
      <h2 id="footer-h" className="sr-only">{t.footer.heading}</h2>
      <div ref={scene} data-onscreen="false" className="padova relative mx-auto max-w-[1440px] overflow-hidden">
        <p className="sr-only">{t.footer.skylineAlt}</p>
        <Skyline />
        <div className="tram-lane" aria-hidden="true"><div className="tram"><Tram /></div></div>
      </div>
      <div className="border-t border-[rgb(var(--rgb-footer-line))] bg-s1">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-10 text-sm sm:grid-cols-[minmax(0,1.2fr)_repeat(3,minmax(0,.7fr))]">
          <div className="space-y-3">
            <Logo />
            <p className="max-w-[28ch] text-t2">{t.footer.tagline}</p>
            <Blip say={t.footer.blip} />
            <p className="font-mono text-xs text-t2">Press <Kbd>?</Kbd> {t.footer.shortcuts}</p>
          </div>
          <nav aria-label="Footer" className="foot-cols">
            <p>{t.footer.product}</p>
            <a href={withSource('/', source)}>{t.footer.home}</a>
            <a href={withSource('/use-cases/', source)}>{t.chrome.navTemplates}</a>
            <a href={withSource('/agents', source)}>{t.footer.mcpSetup}</a>
          </nav>
          <nav aria-label="People" className="foot-cols">
            <p>{t.footer.people}</p>
            <a href={withSource('/makers/', source)}>{t.footer.forMakers}</a>
            <a href={withSource('/publishers/', source)}>{t.footer.forPublishers}</a>
            <a href={withSource('/you/', source)}>{t.footer.yourLimits}</a>
          </nav>
          <nav aria-label="Paper" className="foot-cols">
            <p>{t.footer.paper}</p>
            <a href={withSource('/protocol/', source)}>{t.footer.protocol}</a>
            <a href="/docs">{t.chrome.navDocs}</a>
            <a href={withSource('/reference', source)}>{t.footer.reference}</a>
            <a href={withSource('/privacy', source)}>{t.footer.privacy}</a>
            <a href={withSource('/terms', source)}>{t.footer.terms}</a>
          </nav>
        </div>
        <div className="foot-stripe" aria-hidden="true" />
      </div>
    </footer>
  )
}
