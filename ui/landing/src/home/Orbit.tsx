import type { CSSProperties } from 'react'

// Ornament only. Six original spheres ride one ellipse. They are not a connection.
const spheres = [
  { cls: 'sphere-moss', a: '-110deg' },
  { cls: 'sphere-clay', a: '-68deg' },
  { cls: 'sphere-sand', a: '168deg' },
  { cls: 'sphere-stone', a: '138deg' },
  { cls: 'sphere-ink', a: '-12deg' },
  { cls: 'sphere-blue', a: '32deg' },
] as const

export function Orbit() {
  return (
    <div className="orbit" aria-hidden="true">
      <div className="orbit-ring orbit-ring-a" />
      <div className="orbit-ring orbit-ring-b" />
      {spheres.map((sphere) => (
        <span key={sphere.cls} className={`sphere ${sphere.cls}`} style={{ '--a0': sphere.a, '--a': sphere.a } as CSSProperties} />
      ))}
    </div>
  )
}
