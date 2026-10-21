import { splitSurfaces, type Surface } from '@/surfaces.mjs'

function Row({ item, notes, quiet }: { item: Surface; notes: boolean; quiet?: boolean }) {
  return (
    <li className={quiet
      ? 'rounded-xl border border-line px-4 py-3 text-sm text-t2'
      : 'flex min-h-11 flex-col justify-center gap-1 rounded-xl border border-line bg-s1 px-4 py-3 text-sm'}>
      <span className="flex items-center justify-between gap-3">
        <span className={quiet ? 'font-semibold text-t1' : 'font-semibold'}>{item.name}</span>
        {quiet ? null : <span className="text-t2">{item.state}</span>}
      </span>
      {notes ? <span className="text-t2">{item.note}</span> : <span className="sr-only">{item.note}</span>}
    </li>
  )
}

export function Runs({ audience, notes = false }: { audience: 'publisher' | 'maker'; notes?: boolean }) {
  const { live, planned } = splitSurfaces(audience)
  return (
    <div data-runs={audience}>
      <ul className="grid gap-2 sm:grid-cols-3">
        {live.map((item) => <Row key={item.name} item={item} notes={notes} />)}
      </ul>
      {planned.length ? (
        <details className="mt-3">
          <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm font-semibold text-t2">Planned</summary>
          <ul className="mt-2 grid gap-2 sm:grid-cols-3">
            {planned.map((item) => <Row key={item.name} item={item} notes={notes} quiet />)}
          </ul>
        </details>
      ) : null}
    </div>
  )
}
