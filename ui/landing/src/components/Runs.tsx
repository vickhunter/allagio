import { orderedSurfaces, surfaceStatus, type Surface } from '@/surfaces.mjs'

function Row({ item, notes }: { item: Surface; notes: boolean }) {
  const name = item.href
    ? <a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4" href={item.href}>{item.name}</a>
    : <span className="font-semibold">{item.name}</span>
  return (
    <li className="flex min-h-11 flex-col justify-center gap-1 rounded-xl border border-line bg-s1 px-4 py-3 text-sm">
      <span className="flex items-center justify-between gap-3">
        {name}
        <span className="text-right text-t2">{surfaceStatus(item)}</span>
      </span>
      <time dateTime={item.date} className="text-[12px] text-t2">{item.date}</time>
      {notes ? <span className="text-t2">{item.note}</span> : <span className="sr-only">{item.note}</span>}
    </li>
  )
}

export function Runs({ audience, notes = false }: { audience: 'publisher' | 'maker'; notes?: boolean }) {
  const rows = orderedSurfaces(audience)
  return (
    <div data-runs={audience}>
      <ul className="grid gap-2 sm:grid-cols-3">
        {rows.map((item) => <Row key={item.name} item={item} notes={notes} />)}
      </ul>
    </div>
  )
}
