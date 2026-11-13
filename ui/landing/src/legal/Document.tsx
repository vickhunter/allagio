import { Page } from '@/site/Page'
import { useSource, withSource } from '@/lib/source'
import { contactEmail, operatorName, updatedISO, updatedLabel } from '@/legal/copy'

export function LegalDocument({
  title,
  lede,
  sections,
  otherHref,
  otherLabel,
}: {
  title: string
  lede: string
  sections: { heading: string; paragraphs: string[]; items?: string[]; after?: string[] }[]
  otherHref: string
  otherLabel: string
}) {
  const source = useSource()
  return (
    <Page page="legal">
      <article className="mx-auto max-w-3xl px-5 pb-16 pt-10 md:pt-14">
        <p className="eyebrow">Legal</p>
        <h1 className="h-display mt-4 text-[40px] sm:text-6xl">{title}</h1>
        <p className="mt-4 text-sm text-t2">
          Last updated <time dateTime={updatedISO}>{updatedLabel}</time>
        </p>
        <p className="mt-5 text-base leading-relaxed text-t1">{lede}</p>
        <p className="mt-4 text-base leading-relaxed text-t1">
          The operator is {operatorName}. The contact address is {contactEmail}.
        </p>
        {sections.map((section) => (
          <section key={section.heading} className="mt-10" aria-labelledby={slug(section.heading)}>
            <h2 id={slug(section.heading)} className="h-display text-2xl sm:text-3xl">{section.heading}</h2>
            {section.paragraphs.map((paragraph) => <p key={paragraph} className="mt-3 text-base leading-relaxed">{paragraph}</p>)}
            {section.items ? <ul className="mt-3 list-disc space-y-2 pl-5 text-base leading-relaxed">{section.items.map((item) => <li key={item}>{item}</li>)}</ul> : null}
            {section.after?.map((paragraph) => <p key={paragraph} className="mt-3 text-base leading-relaxed">{paragraph}</p>)}
          </section>
        ))}
        <p className="mt-10 text-base leading-relaxed">
          <a className="inline-flex min-h-11 items-center underline underline-offset-4" href={withSource(otherHref, source)}>{otherLabel}</a>
        </p>
      </article>
    </Page>
  )
}

function slug(heading: string) {
  return heading.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
}
