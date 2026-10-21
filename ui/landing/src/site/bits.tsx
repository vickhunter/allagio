import { ArrowRight, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Kbd } from '@/components/ui/input'
import { JoinForm } from '@/components/JoinForm'
import { hardChecks, modes } from '@/content'
import { roleReturn, signInHref, useSource } from '@/lib/source'

// The ONE raised keycap per page. It opens Google sign-in and returns to the owner record.
export function PrimaryCta({ label = 'Sign in with Google', where, role = 'maker' }: { label?: string; where: string; role?: 'maker' | 'publisher' }) {
  const source = useSource()
  const href = signInHref(roleReturn(role === 'publisher' ? 'publisher' : 'maker'), source)
  return (
    <div className="flex flex-col items-start gap-2">
      <Button variant="keycap" asChild><a href={href} data-primary-action data-track="cta_click" data-track-cta={where} data-track-kind="sign_in_click">{label} <ArrowRight aria-hidden /></a></Button>
      <p className="text-[13px] text-t2">Google sign-in. We never hold your money. Or press <Kbd>J</Kbd>.</p>
    </div>
  )
}

export function Checks({ id = 'checks', eyebrow }: { id?: string; eyebrow: string }) {
  return (
    <section id={id} aria-labelledby={`${id}-h`} className="border-t border-line">
      <div className="mx-auto grid max-w-6xl gap-10 px-5 py-16 md:grid-cols-[1fr_2fr] md:py-20">
        <div><p className="eyebrow">{eyebrow}</p><h2 id={`${id}-h`} className="h-display mt-3 text-3xl sm:text-4xl">Nine checks no agent can skip.</h2></div>
        <div className="grid gap-10 sm:grid-cols-2">
          <ol className="space-y-2.5 text-[15px]">{hardChecks.map((c, n) => <li key={c} className="flex gap-3 border-b border-line pb-2.5"><span className="w-5 font-mono text-xs leading-6 text-t2">{n + 1}</span><Check className="mt-1 size-4 shrink-0 text-pub" aria-hidden />{c}</li>)}</ol>
          <div className="space-y-6">
            {modes.map((m, n) => <div key={m.name}><p className="font-mono text-xs text-t2">Mode {n + 1}</p><h3 className="mt-1 font-display text-lg font-semibold">{m.name}</h3><p className="mt-1 text-sm leading-relaxed text-t2">{m.note}</p></div>)}
            <p className="rounded-xl border border-line bg-s1 p-4 text-sm leading-relaxed text-t2">Agents can only make settings stricter. Loosening a mode needs you, signed in.</p>
          </div>
        </div>
      </div>
    </section>
  )
}

export function JoinSection({ title, sub, role }: { title: string; sub: string; role?: 'maker' | 'publisher' }) {
  return (
    <section aria-labelledby="join-h" className="border-t border-line bg-s1">
      <div className="mx-auto grid max-w-6xl gap-8 px-5 py-16 md:grid-cols-2 md:py-20">
        <div><h2 id="join-h" className="h-display text-3xl sm:text-4xl">{title}</h2><p className="mt-3 text-t2">{sub}</p></div>
        <JoinForm id="join" role={role} cta={role === 'maker' ? 'Start as a maker' : role === 'publisher' ? 'Start as a publisher' : 'Sign in with Google'} />
      </div>
    </section>
  )
}
