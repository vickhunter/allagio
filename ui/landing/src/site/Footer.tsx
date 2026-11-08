import { useEffect, useRef } from 'react'
import { Logo } from '@/components/Logo'
import { Blip } from '@/components/Blip'
import { Kbd } from '@/components/ui/input'
import { Skyline, Tram } from './Padova'
import { useSource, withSource } from '@/lib/source'
import { workingName } from '@/content'

// Footer on every page: hand-drawn Padova skyline (own SVG, no copied assets) + one SIR1 tram.
// Tram moves with CSS transform only; paused off-screen (IntersectionObserver); animation:none under reduced motion.
export function Footer() {
  const source = useSource()
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
      <h2 id="footer-h" className="sr-only">{workingName} Made in Padova.</h2>
      <div ref={scene} data-onscreen="false" className="padova relative mx-auto max-w-[1440px] overflow-hidden">
        <p className="sr-only">Illustration: the Padova skyline with the domes of the Basilica del Santo, the keel roof of Palazzo della Ragione, the statue ring and canal of Prato della Valle and the Specola tower, with a SIR1 tram passing.</p>
        <Skyline />
        <div className="tram-lane" aria-hidden="true"><div className="tram"><Tram /></div></div>
      </div>
      <div className="border-t border-[rgb(var(--rgb-footer-line))] bg-s1">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 py-10 text-sm sm:grid-cols-[minmax(0,1.2fr)_repeat(3,minmax(0,.7fr))]">
          <div className="space-y-3">
            <Logo />
            <p className="max-w-[28ch] text-t2">{workingName} Agents find, match and place. Humans get the results. Drawn and built in Padova.</p>
            <Blip say="Next stop: Prato della Valle." />
            <p className="font-mono text-xs text-t2">Press <Kbd>?</Kbd> for shortcuts</p>
          </div>
          <nav aria-label="Footer" className="foot-cols">
            <p>Product</p>
            <a href={withSource('/', source)}>Home</a>
            <a href={withSource('/landing-v9/agents/', source)}>MCP and setup</a>
            <a href="/landing-v9/how-it-works/">How it works</a>
          </nav>
          <nav aria-label="People" className="foot-cols">
            <p>People</p>
            <a href={withSource('/makers/', source)}>For makers</a>
            <a href={withSource('/publishers/', source)}>For publishers</a>
            <a href={withSource('/you/', source)}>Your limits</a>
          </nav>
          <nav aria-label="Paper" className="foot-cols">
            <p>Paper</p>
            <a href={withSource('/protocol/', source)}>Protocol</a>
            <a href={withSource('/privacy', source)}>Privacy</a>
            <a href={withSource('/terms', source)}>Terms</a>
          </nav>
        </div>
        <div className="foot-stripe" aria-hidden="true" />
      </div>
    </footer>
  )
}
