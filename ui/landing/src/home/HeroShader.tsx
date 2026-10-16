import { Suspense, lazy, useEffect, useState } from 'react'
import { reducedMotion } from '@/lib/utils'

// Rule 37: ONE free Shaders effect (MeshGradient, MIT, npm "shaders") behind the hero. Never Pro.
// The static fallback is always painted first; the WebGPU canvas only mounts when navigator.gpu exists and
// motion is allowed, and is removed again if the library reports the GPU unavailable.
// All stops are light warm neutrals, so t1/t2 text on top keeps AA (checked by axe on the fallback).
// The library (~700 kB gz) is only fetched when WebGPU exists and motion is allowed.
const ShaderLayer = lazy(() => import('./ShaderLayer'))
export function HeroShader() {
  const [on, setOn] = useState(false)
  const [state, setState] = useState<'fallback' | 'webgpu'>('fallback')
  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => {
      const ok = typeof navigator !== 'undefined' && 'gpu' in navigator && !media.matches && !reducedMotion()
      setOn(ok)
      if (!ok) setState('fallback')
    }
    update()
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])
  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0 -z-10" data-hero-bg={state}>
      <div className="absolute inset-0 hero-fallback" />
      {on && (
        <div className="shader-in absolute inset-0">
          <Suspense fallback={null}><ShaderLayer onReady={() => setState('webgpu')} onUnavailable={() => { setOn(false); setState('fallback') }} /></Suspense>
        </div>
      )}
      <div className="absolute inset-x-0 bottom-0 h-24 bg-gradient-to-b from-transparent to-s0" />
    </div>
  )
}
