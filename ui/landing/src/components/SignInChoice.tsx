import { useRef, useState } from 'react'
import { ArrowRight, Package, PanelTop, X } from 'lucide-react'
import { track } from '@/lib/track'
import { roleReturn, signInHref, useSource } from '@/lib/source'

// D-048 sign-in: two clear choices, each opens a short "what happens next" step, then Google sign-in.
// Native <dialog>.showModal(): focus is trapped, Esc closes, focus returns to the opener.
// "I run a site" returns to /owner. "I sell a product" returns to /owner?setup=maker.
type Role = 'maker' | 'publisher'

const copy: Record<Role, { title: string; steps: string[]; note: string }> = {
  maker: {
    title: 'You sell a product',
    steps: [
      'Sign in with Google.',
      'Add your product: name, link and a short description.',
      'Set the commission you will pay on each sale.',
      'Publishers whose posts fit your product can then show it. You pay only when a sale comes from one of their posts.',
    ],
    note: 'You can change or pause your offer at any time.',
  },
  publisher: {
    title: 'You run a site',
    steps: [
      'Sign in with Google.',
      'Connect your WordPress site with the Offer Rails plugin.',
      'We suggest products that fit your posts. You see the exact box and where it goes.',
      'You approve each placement. When a reader buys, you earn the commission.',
    ],
    note: 'Nothing appears on your site until you approve it. You can remove a box at any time.',
  },
}

export function SignInChoice({ where, only }: { where: string; only?: Role }) {
  const source = useSource()
  const ref = useRef<HTMLDialogElement>(null)
  const [role, setRole] = useState<Role>(only ?? 'maker')
  const open = (r: Role) => { setRole(r); track('role_pick', { role: r, where }); ref.current?.showModal() }
  const c = copy[role]
  const roles: Role[] = only ? [only] : ['maker', 'publisher']
  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row">
        {roles.includes('maker') && <button type="button" onClick={() => open('maker')} data-track="cta_click" data-track-cta={`${where}_maker`}
          className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border-2 border-maker bg-maker-soft px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s1">
          <Package className="size-5 text-maker" aria-hidden /> I sell a product</button>}
        {roles.includes('publisher') && <button type="button" onClick={() => open('publisher')} data-track="cta_click" data-track-cta={`${where}_publisher`}
          className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border-2 border-pub bg-pub-soft px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s1">
          <PanelTop className="size-5 text-pub" aria-hidden /> I run a site</button>}
      </div>
      <dialog ref={ref} aria-labelledby={`${where}-dlg-h`} className="signin-dlg m-auto w-[min(92vw,30rem)] rounded-2xl border border-line bg-s0 p-0 text-t1 backdrop:bg-[rgb(var(--rgb-text-1)/.35)] backdrop:backdrop-blur-sm"
        onClick={(e) => { if (e.target === ref.current) ref.current?.close() }}>
        <div className="p-6">
          <div className="flex items-start justify-between gap-3">
            <h2 id={`${where}-dlg-h`} className="font-display text-2xl font-bold tracking-tight">{c.title}</h2>
            <button type="button" onClick={() => ref.current?.close()} className="inline-flex size-11 shrink-0 items-center justify-center rounded-lg hover:bg-s2" aria-label="Close"><X className="size-5" aria-hidden /></button>
          </div>
          <p className="mt-1 text-t2">What happens next:</p>
          <ol className="mt-4 space-y-3">
            {c.steps.map((s, i) => <li key={s} className="grid grid-cols-[1.75rem_1fr] gap-2 text-[15px] leading-relaxed"><span className={`font-display font-bold ${role === 'maker' ? 'text-maker' : 'text-pub'}`}>{i + 1}</span>{s}</li>)}
          </ol>
          <p className="mt-4 rounded-xl border border-line bg-s1 p-3 text-sm text-t2">{c.note}</p>
          <a href={signInHref(roleReturn(role), source)} data-primary-action data-track="cta_click" data-track-cta={`google_${role}`} data-track-kind="sign_in_click"
            className="mt-5 inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)]">
            Continue with Google <ArrowRight className="size-4" aria-hidden /></a>
        </div>
      </dialog>
    </div>
  )
}
