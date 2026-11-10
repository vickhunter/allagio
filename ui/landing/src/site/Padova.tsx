// Padova, drawn by hand in SVG for Allagio (no traced photos, no third-party assets).
// Left→right: Prato della Valle (statue ring + canal), Basilica del Santo (domes + bell towers),
// Palazzo della Ragione (keel roof over the loggias), La Specola (observatory tower).
const C = {
  far: 'rgb(226 217 198)', wall: 'rgb(232 222 200)', wall2: 'rgb(219 205 178)', roof: 'rgb(176 112 84)',
  lead: 'rgb(150 142 126)', dome: 'rgb(198 186 160)', ink: 'rgb(110 98 80)', water: 'rgb(196 204 192)', grass: 'rgb(208 211 180)', stone: 'rgb(238 232 216)',
}
const S = { stroke: C.ink, strokeWidth: 1.6, strokeLinejoin: 'round' as const, strokeLinecap: 'round' as const }

function Statue({ x, y, s = 1 }: { x: number; y: number; s?: number }) {
  return (
    <g transform={`translate(${x} ${y}) scale(${s})`}>
      <rect x="-3.5" y="-9" width="7" height="9" fill={C.stone} {...S} strokeWidth={1.1} />
      <path d="M-2 -9 L-2.4 -17 Q0 -19 2.4 -17 L2 -9 Z" fill={C.dome} {...S} strokeWidth={1} />
      <circle cx="0" cy="-20" r="2" fill={C.dome} {...S} strokeWidth={1} />
    </g>
  )
}

function Prato() {
  const cx = 245, cy = 262, rx = 168, ry = 15
  const ring = (n: number, front: boolean) => Array.from({ length: n }, (_, i) => {
    const t = Math.PI * (i + 0.5) / n
    const x = cx - Math.cos(t) * (rx - 6)
    const y = cy + (front ? 1 : -1) * Math.sin(t) * (ry - 3)
    return <Statue key={`${front}${i}`} x={x} y={y} s={front ? 1.15 : 0.85} />
  })
  return (
    <g>
      <ellipse cx={cx} cy={cy} rx={rx + 22} ry={ry + 9} fill={C.water} {...S} />
      <ellipse cx={cx} cy={cy} rx={rx} ry={ry} fill={C.grass} {...S} />
      {ring(17, false)}
      <ellipse cx={cx} cy={cy} rx={52} ry={5} fill="none" {...S} strokeWidth={1} strokeDasharray="3 4" />
      {/* four bridges across the canal */}
      {[cx - rx - 11, cx + rx + 11].map((x) => <path key={x} d={`M${x - 10} ${cy - 2} q10 -8 20 0`} fill={C.stone} {...S} />)}
      {[cx - 60, cx + 60].map((x) => <path key={x} d={`M${x - 9} ${cy + ry + 6} q9 -7 18 0`} fill={C.stone} {...S} />)}
      {ring(13, true)}
    </g>
  )
}

function Dome({ x, y, r, lantern = true }: { x: number; y: number; r: number; lantern?: boolean }) {
  return (
    <g>
      <rect x={x - r * 0.8} y={y - 4} width={r * 1.6} height={6} fill={C.wall2} {...S} />
      <path d={`M${x - r} ${y - 4} A${r} ${r * 0.95} 0 0 1 ${x + r} ${y - 4} Z`} fill={C.dome} {...S} />
      <path d={`M${x} ${y - 4 - r * 0.95} V${y - 4}`} stroke={C.ink} strokeWidth={0.8} opacity={0.5} />
      {lantern && <g><rect x={x - 3} y={y - 4 - r * 0.95 - 7} width={6} height={7} fill={C.stone} {...S} strokeWidth={1.1} /><path d={`M${x - 3.5} ${y - 11 - r * 0.95} L${x} ${y - 18 - r * 0.95} L${x + 3.5} ${y - 11 - r * 0.95}`} fill={C.lead} {...S} strokeWidth={1.1} /></g>}
    </g>
  )
}

function BellTower({ x, top }: { x: number; top: number }) {
  return (
    <g>
      <rect x={x - 6} y={top + 30} width={12} height={200 - top - 30} fill={C.wall} {...S} />
      <rect x={x - 7.5} y={top + 26} width={15} height={8} fill={C.stone} {...S} />
      <rect x={x - 5} y={top + 12} width={10} height={14} fill={C.wall} {...S} />
      <path d={`M${x - 2} ${top + 16} v6 M${x + 2} ${top + 16} v6`} stroke={C.ink} strokeWidth={1.2} />
      <path d={`M${x - 6} ${top + 12} L${x} ${top - 14} L${x + 6} ${top + 12} Z`} fill={C.lead} {...S} />
      <path d={`M${x} ${top - 14} v-6`} stroke={C.ink} strokeWidth={1.2} />
    </g>
  )
}

function Santo() {
  return (
    <g>
      {/* nave + transept body */}
      <rect x="590" y="178" width="200" height="72" fill={C.wall2} {...S} />
      {/* back row of domes, smaller */}
      <Dome x={640} y={180} r={22} />
      <Dome x={745} y={180} r={22} />
      <BellTower x={612} top={92} />
      <BellTower x={772} top={98} />
      {/* central conical dome on a drum */}
      <rect x="670" y="128" width="40" height="52" fill={C.wall} {...S} />
      {[678, 690, 702].map((x) => <path key={x} d={`M${x} 140 v16`} stroke={C.ink} strokeWidth={1.2} />)}
      <path d="M664 130 L690 72 L716 130 Z" fill={C.dome} {...S} />
      <rect x="687" y="58" width="6" height="14" fill={C.stone} {...S} strokeWidth={1.1} />
      <path d="M690 58 v-8" stroke={C.ink} strokeWidth={1.2} />
      {/* front domes */}
      <Dome x={612} y={200} r={20} lantern={false} />
      <Dome x={690} y={196} r={26} />
      <Dome x={765} y={200} r={20} lantern={false} />
      {/* facade with gable, rose window, loggia arches and two small turrets */}
      <rect x="500" y="168" width="108" height="82" fill={C.wall} {...S} />
      <path d="M500 168 L554 128 L608 168 Z" fill={C.wall} {...S} />
      <circle cx="554" cy="152" r="8" fill={C.stone} {...S} />
      <path d="M548 152 h12 M554 146 v12" stroke={C.ink} strokeWidth={0.9} />
      {[512, 532, 576, 596].map((x) => <path key={x} d={`M${x - 6} 222 v-14 a6 6 0 0 1 12 0 v14`} fill={C.wall2} {...S} strokeWidth={1.2} />)}
      <path d="M543 250 v-30 a11 11 0 0 1 22 0 v30" fill={C.wall2} {...S} />
      {[500, 608].map((x) => <g key={x}><rect x={x - 4} y="138" width="8" height="30" fill={C.wall} {...S} /><path d={`M${x - 5} 138 L${x} 120 L${x + 5} 138 Z`} fill={C.lead} {...S} /></g>)}
    </g>
  )
}

function Ragione() {
  const arches = (y: number, h: number, n: number, x0: number, w: number) => Array.from({ length: n }, (_, i) => {
    const step = w / n, x = x0 + i * step + 3
    return <path key={`${y}${i}`} d={`M${x} ${y + h} v${-h + step / 2 - 3} a${step / 2 - 3} ${step / 2 - 3} 0 0 1 ${step - 6} 0 v${h - step / 2 + 3}`} fill={C.wall2} {...S} strokeWidth={1.1} />
  })
  return (
    <g>
      <rect x="862" y="180" width="236" height="70" fill={C.wall} {...S} />
      <path d="M862 214 H1098" stroke={C.ink} strokeWidth={1.2} />
      {arches(186, 24, 14, 862, 236)}
      {arches(220, 30, 10, 862, 236)}
      {/* the keel roof: an upturned ship's hull, ribbed in lead */}
      <path d="M850 182 C858 136 1102 136 1110 182 Z" fill={C.lead} {...S} />
      {[0.18, 0.34, 0.5, 0.66, 0.82].map((t) => { const x = 852 + t * 256; return <path key={t} d={`M${x} 182 Q${980 + (x - 980) * 0.7} 158 ${980 + (x - 980) * 0.5} 148`} fill="none" stroke={C.stone} strokeWidth={1} opacity={0.6} /> })}
      <path d="M852 182 H1108" stroke={C.ink} strokeWidth={2} />
    </g>
  )
}

function Specola() {
  return (
    <g>
      <path d="M1196 250 L1201 86 H1243 L1248 250 Z" fill={C.wall2} {...S} />
      {/* crenellations */}
      <path d="M1197 86 v-10 h7 v6 h6 v-6 h7 v6 h6 v-6 h7 v6 h6 v-6 h7 v10 Z" fill={C.wall} {...S} />
      {/* observatory terrace + turret */}
      <rect x="1207" y="52" width="30" height="24" fill={C.wall} {...S} />
      <path d="M1205 52 A17 12 0 0 1 1239 52 Z" fill={C.dome} {...S} />
      <path d="M1222 41 v-8" stroke={C.ink} strokeWidth={1.2} />
      {[110, 150, 190].map((y) => <rect key={y} x="1218" y={y} width="8" height="14" rx="4" fill={C.ink} opacity={0.55} />)}
      <rect x="1213" y="58" width="5" height="10" rx="2.5" fill={C.ink} opacity={0.5} />
      <rect x="1226" y="58" width="5" height="10" rx="2.5" fill={C.ink} opacity={0.5} />
      {/* castle wall at its foot */}
      <path d="M1160 250 V214 h86 V250" fill={C.wall} {...S} />
      <path d="M1160 214 v-6 h8 v6 h8 v-6 h8 v6 h8 v-6 h8 v6" fill="none" {...S} />
    </g>
  )
}

function Houses({ x, w, seed }: { x: number; w: number; seed: number }) {
  const out = []
  let cx = x, i = 0
  while (cx < x + w) {
    const hw = 26 + ((seed * (i + 3) * 7) % 22), hh = 34 + ((seed * (i + 5) * 11) % 30)
    out.push(<g key={i}><rect x={cx} y={250 - hh} width={hw} height={hh} fill={i % 2 ? C.wall : C.stone} {...S} strokeWidth={1.2} /><path d={`M${cx - 2} ${250 - hh} L${cx + hw / 2} ${244 - hh - 8} L${cx + hw + 2} ${250 - hh} Z`} fill={C.roof} {...S} strokeWidth={1.2} /><rect x={cx + hw / 2 - 3} y={258 - hh} width="6" height="9" fill={C.ink} opacity={0.4} /></g>)
    cx += hw + 2; i++
  }
  return <g>{out}</g>
}

function Cypress({ x, h = 50 }: { x: number; h?: number }) {
  return <path d={`M${x} 250 C${x - 9} ${250 - h * 0.4} ${x - 5} ${250 - h * 0.85} ${x} ${250 - h} C${x + 5} ${250 - h * 0.85} ${x + 9} ${250 - h * 0.4} ${x} 250 Z`} fill="rgb(150 158 120)" {...S} strokeWidth={1.2} />
}

export function Skyline() {
  return (
    <svg viewBox="0 28 1440 272" className="block h-auto w-full min-w-[720px] -translate-x-[calc((720px-100%)*0.42)] sm:min-w-0 sm:translate-x-0" aria-hidden="true" focusable="false">
      {/* far roofline */}
      <path d="M0 250 V214 h40 l12 -10 l12 10 h60 v-12 h30 v12 h70 l18 -14 l18 14 h120 v-18 h24 v18 h200 l10 -8 l10 8 h160 v-14 h40 v14 h170 l14 -12 l14 12 h120 v-10 h60 v10 h188 V250 Z" fill={C.far} />
      <Houses x={430} w={64} seed={3} />
      <Houses x={800} w={56} seed={5} />
      <Houses x={1112} w={44} seed={7} />
      <Houses x={1262} w={178} seed={4} />
      <Santo />
      <Ragione />
      <Specola />
      <Cypress x={455} h={56} /><Cypress x={470} h={44} /><Cypress x={1150} h={50} /><Cypress x={1290} h={46} />
      {/* street and the SIR1 single central guide rail (Translohr) */}
      <rect x="0" y="250" width="1440" height="50" fill="rgb(222 214 196)" />
      <Prato />
      <path d="M0 250 H1440" stroke={C.ink} strokeWidth={1.6} />
      <path d="M0 292 H1440" stroke={C.ink} strokeWidth={1.4} opacity={0.6} />
      <path d="M0 292 H1440" stroke={C.ink} strokeWidth={1} strokeDasharray="10 14" opacity={0.35} transform="translate(0 -3)" />
    </svg>
  )
}

// SIR1: white articulated rubber-tyred tram, red band, pantograph. Own drawing.
export function Tram() {
  const mod = (x: number, w: number, nose?: 'l' | 'r') => (
    <g key={x}>
      <path d={nose === 'l' ? `M${x + 10} 6 H${x + w} V34 H${x} V18 Q${x} 6 ${x + 10} 6 Z` : nose === 'r' ? `M${x} 6 H${x + w - 10} Q${x + w} 6 ${x + w} 18 V34 H${x} Z` : `M${x} 6 H${x + w} V34 H${x} Z`} fill="rgb(251 250 244)" stroke={C.ink} strokeWidth={1.4} strokeLinejoin="round" />
      <rect x={x + 4} y={10} width={w - 8} height={11} rx={2} fill="rgb(92 98 87)" />
      <rect x={x} y={25} width={w} height={4} fill="rgb(198 79 56)" />
      <circle cx={x + w / 2} cy={36} r={3.4} fill="rgb(60 56 48)" />
    </g>
  )
  return (
    <svg viewBox="0 0 196 42" width="196" height="42" focusable="false">
      <path d="M86 6 L96 -4 L108 -4" fill="none" stroke={C.ink} strokeWidth={1.4} />
      {mod(0, 46, 'l')}{mod(48, 48)}{mod(98, 48)}{mod(148, 46, 'r')}
      <text x="9" y="19" fontFamily="var(--font-mono)" fontSize="6.5" fill="rgb(251 250 244)">SIR1</text>
    </svg>
  )
}
