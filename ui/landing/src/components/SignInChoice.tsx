import { useRef } from 'react'
import { ArrowRight, Package, PanelTop, X } from 'lucide-react'
import { track } from '@/lib/track'
import { roleReturn, signInHref, useSource, withSource } from '@/lib/source'

// "I sell a product" opens a short step, then Google sign-in back to /owner?setup=maker.
// "I run a site" opens /publishers/setup. Native <dialog>.showModal() traps focus; Esc closes it.
type Role = 'maker' | 'publisher'

const makerCopy = {
  title: 'You sell a product',
  steps: [
    'Sign in with Google.',
    'Add your product: name, link and a short description.',
    'Set the commission you will pay on each sale.',
    'Publishers whose posts fit your product can then show it. You pay only when a sale comes from one of their posts.',
  ],
  note: 'You can change or pause your offer at any time.',
}

export function SignInChoice({ where, only }: { where: string; only?: Role }) {
  const source = useSource()
  const ref = useRef<HTMLDialogElement>(null)
  const roles: Role[] = only ? [only] : ['maker', 'publisher']
  const publisherHref = withSource('/publishers/setup', source)
  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row">
        {roles.includes('maker') && <button type="button" onClick={() => { track('role_pick', { role: 'maker', where }); ref.current?.showModal() }} data-track="cta_click" data-track-cta={`${where}_maker`}
          className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border-2 border-maker bg-maker-soft px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s1">
          <Package className="size-5 text-maker" aria-hidden /> I sell a product</button>}
        {roles.includes('publisher') && <a href={publisherHref} data-track="cta_click" data-track-cta={`${where}_publisher`} {...(only === 'publisher' ? { 'data-primary-action': true } : {})}
          className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border-2 border-pub bg-pub-soft px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s1">
          <PanelTop className="size-5 text-pub" aria-hidden /> I run a site</a>}
      </div>
      {roles.includes('maker') && <dialog ref={ref} aria-labelledby={`${where}-dlg-h`} className="signin-dlg m-auto w-[min(92vw,30rem)] rounded-2xl border border-line bg-s0 p-0 text-t1 backdrop:bg-[rgb(var(--rgb-text-1)/.35)] backdrop:backdrop-blur-sm"
        onClick={(e) => { if (e.target === ref.current) ref.current?.close() }}>
        <div className="p-6">
          <div className="flex items-start justify-between gap-3">
            <h2 id={`${where}-dlg-h`} className="font-display text-2xl font-bold tracking-tight">{makerCopy.title}</h2>
            <button type="button" onClick={() => ref.current?.close()} className="inline-flex size-11 shrink-0 items-center justify-center rounded-lg hover:bg-s2" aria-label="Close"><X className="size-5" aria-hidden /></button>
          </div>
          <p className="mt-1 text-t2">What happens next:</p>
          <ol className="mt-4 space-y-3">
            {makerCopy.steps.map((s, i) => <li key={s} className="grid grid-cols-[1.75rem_1fr] gap-2 text-[15px] leading-relaxed"><span className="font-display font-bold text-maker">{i + 1}</span>{s}</li>)}
          </ol>
          <p className="mt-4 rounded-xl border border-line bg-s1 p-3 text-sm text-t2">{makerCopy.note}</p>
          <a href={signInHref(roleReturn('maker'), source)} data-primary-action data-track="cta_click" data-track-cta="google_maker" data-track-kind="sign_in_click"
            className="mt-5 inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)]">
            Continue with Google <ArrowRight className="size-4" aria-hidden /></a>
        </div>
      </dialog>}
    </div>
  )
}
