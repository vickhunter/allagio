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
      <div className="border-t border-[rgb(var(--rgb-footer-line))]">
        <div className="mx-auto grid max-w-6xl gap-6 px-5 py-8 text-sm sm:grid-cols-[1fr_auto] sm:items-end">
          <div className="space-y-3">
            <Logo />
            <nav aria-label="Footer" className="flex flex-wrap gap-x-5 gap-y-2">
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/', source)}>Home</a>
              <a className="inline-flex min-h-11 min-w-11 items-center text-maker underline-offset-4 hover:underline" href={withSource('/makers/', source)}>For makers</a>
              <a className="inline-flex min-h-11 min-w-11 items-center text-pub underline-offset-4 hover:underline" href={withSource('/publishers/', source)}>For publishers</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/protocol/', source)}>Protocol</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/you/', source)}>Your limits</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/privacy', source)}>Privacy</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/terms', source)}>Terms</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href="/landing-v9/how-it-works/">How it works</a>
              <a className="inline-flex min-h-11 min-w-11 items-center underline-offset-4 hover:underline" href={withSource('/landing-v9/agents/', source)}>MCP and setup</a>
            </nav>
            <p className="text-t2">{workingName} Agents find, match and place. Humans get the results. Drawn and built in Padova.</p>
          </div>
          <div className="flex flex-col items-start gap-3 sm:items-end">
            <Blip say="Next stop: Prato della Valle." />
            <p className="font-mono text-xs text-t2">Press <Kbd>?</Kbd> for shortcuts</p>
          </div>
        </div>
      </div>
    </footer>
  )
}
