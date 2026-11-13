import { useSource, withSource } from '@/lib/source'
import { useLocale } from '@/i18n'

function Status({ status }: { status: string }) {
  return <span className="inline-flex min-h-7 items-center rounded-full border border-line px-2 font-mono text-[11px] uppercase tracking-wide text-t2">{status}</span>
}

export function Vision() {
  const source = useSource()
  const { t } = useLocale()
  return (
    <div id="vision" className="grid gap-10">
      <section aria-labelledby="does-h">
        <h2 id="does-h" className="font-display text-2xl font-bold tracking-tight">{t.vision.doesHeading}</h2>
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-t2">{t.vision.doesLead}</p>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {t.vision.deals.map((deal) => (
            <li key={deal.name} className="rounded-2xl border border-line bg-s1 p-4">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="font-semibold">{deal.name}</h3>
                <Status status={deal.status} />
              </div>
              <p className="mt-2 text-sm leading-relaxed text-t2">{deal.note}</p>
            </li>
          ))}
        </ul>
      </section>
      <section aria-labelledby="keep-h">
        <h2 id="keep-h" className="font-display text-2xl font-bold tracking-tight">{t.vision.keepHeading}</h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {t.vision.keep.map((item) => (
            <li key={item.t} className="rounded-2xl border border-line bg-s1 p-4">
              <h3 className="font-semibold">{item.t}</h3>
              <p className="mt-2 text-sm leading-relaxed text-t2">{item.b}</p>
            </li>
          ))}
        </ul>
        <p className="mt-3 text-sm text-t2">
          <a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4 hover:no-underline" href={withSource('/you/', source)}>{t.vision.keepLink}</a>
        </p>
      </section>
      <section aria-labelledby="replaces-h">
        <h2 id="replaces-h" className="font-display text-2xl font-bold tracking-tight">{t.vision.replacesHeading}</h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {t.vision.replaces.map((item) => (
            <li key={item.t} className="rounded-2xl border border-line bg-s1 p-4">
              <h3 className="font-semibold">{item.t}</h3>
              <p className="mt-2 text-sm leading-relaxed text-t2">{item.b}</p>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
