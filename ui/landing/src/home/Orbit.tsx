// Original orbit. Six plain spheres and two thin ellipses. No copied marks, no customer names.
const spheres = [
  { cls: 'sphere-moss', style: { top: '-14%', left: '18%' } },
  { cls: 'sphere-clay', style: { top: '-16%', left: '64%' } },
  { cls: 'sphere-sand', style: { top: '34%', left: '-6%' } },
  { cls: 'sphere-stone', style: { top: '46%', left: '6%' } },
  { cls: 'sphere-ink', style: { top: '18%', left: '88%' } },
  { cls: 'sphere-blue', style: { top: '58%', left: '82%' } },
] as const

export function Orbit() {
  return (
    <div className="orbit" aria-hidden="true">
      <svg className="orbit-ring" viewBox="0 0 760 280">
        <ellipse cx="380" cy="140" rx="330" ry="112" fill="none" stroke="currentColor" strokeWidth="1" />
        <ellipse cx="380" cy="140" rx="286" ry="92" fill="none" stroke="currentColor" strokeWidth="1" />
      </svg>
      <div className="orbit-spin">
        {spheres.map((sphere) => <span key={sphere.cls} className={`sphere ${sphere.cls}`} style={sphere.style} />)}
      </div>
    </div>
  )
}
