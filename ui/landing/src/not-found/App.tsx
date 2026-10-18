import { Page } from '@/site/Page'
import { useSource, withSource } from '@/lib/source'

export function App() {
  const source = useSource()
  return (
    <Page page="missing">
      <section className="mx-auto max-w-3xl px-5 pb-20 pt-16">
        <h1 className="h-display text-4xl sm:text-6xl">That page is not on Offer Rails.</h1>
        <p className="mt-4 text-lg leading-relaxed text-t2">The address does not match a page on this site.</p>
        <p className="mt-8 flex flex-wrap gap-3">
          <a className="inline-flex min-h-11 min-w-11 items-center rounded-xl bg-t1 px-5 font-semibold text-s0" href={withSource('/', source)}>Home</a>
          <a className="inline-flex min-h-11 min-w-11 items-center rounded-xl border border-line-strong px-5 font-semibold" href={withSource('/agents', source)}>Agents</a>
        </p>
      </section>
    </Page>
  )
}
