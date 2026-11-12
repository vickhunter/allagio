import { Marquee } from '@/components/ui/marquee'
import { clientMarks } from '@/components/join-prompt.mjs'

const heading = 'Runs in the agents you already use.'

export function ClientStrip() {
  return (
    <section id="clients" aria-labelledby="clients-h" className="border-t border-line bg-s1 py-10">
      <h2 id="clients-h" className="sr-only">{heading}</h2>
      <p aria-hidden="true" className="text-center text-[13px] font-semibold uppercase tracking-[0.14em] text-t2">{heading}</p>
      <Marquee pauseOnHover className="mt-6 [--duration:28s]" repeat={2}>
        {clientMarks.map((mark) => (
          <span
            key={mark.name}
            className="inline-flex items-center gap-2 rounded-full border border-line bg-s1 px-4 py-2 text-[13px] font-semibold text-t1"
          >
            <svg viewBox="0 0 24 24" width={14} height={14} role="img" aria-label={mark.name} className="shrink-0 opacity-80">
              <path d={mark.d} fill="currentColor" />
            </svg>
            {mark.name}
          </span>
        ))}
      </Marquee>
      <p className="mt-6 text-center">
        <a href="/agents" data-track="clients-agents" className="text-[13px] font-semibold text-t1 underline decoration-line underline-offset-4 transition-colors hover:decoration-t1">
          Install guides for every client
        </a>
      </p>
    </section>
  )
}
