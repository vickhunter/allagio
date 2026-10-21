import { JoinPrompt } from '@/components/join-prompt.mjs'
import { signInHref, useOfferConfig, useSource, withSource } from '@/lib/source'

type Role = 'maker' | 'publisher'

export function JoinForm({ role, cta = 'Sign in with Google', id = 'join', note }: { role?: Role; cta?: string; id?: string; note?: string }) {
  const source = useSource()
  const { install, plugin } = useOfferConfig()
  const href = signInHref(role === 'publisher' ? '/publishers/setup' : role === 'maker' ? '/owner?setup=maker' : '/owner', source)
  return (
    <div id={id} className="flex flex-col gap-3">
      <JoinPrompt />
      {note ? <p className="text-sm text-t2">{note}</p> : null}
      <details className="rounded-xl border border-line bg-s0">
        <summary className="inline-flex min-h-11 cursor-pointer items-center px-3 text-sm font-semibold">Set it up yourself</summary>
        <div className="flex flex-col gap-3 px-3 pb-3">
          <a href={href} data-track="join_submit" data-track-role={role || 'unset'} data-track-cta={cta} className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline">{cta}</a>
          {install ? (
            <div>
              <p className="text-[13px] font-semibold text-t2">Install command</p>
              <pre className="mt-2 whitespace-pre-wrap rounded-lg border border-line bg-s1 p-3 font-mono text-sm">{install}</pre>
              <p className="mt-1 text-[13px] text-t2">Paste this as is. One install step.</p>
            </div>
          ) : (
            <p className="text-[13px] text-t2">The install command is published with the hosted download. Sign in to register a product meanwhile.</p>
          )}
          <a href={withSource('/landing-v9/agents/', source)} data-track="secondary_click" data-track-to="agents" className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline">MCP and setup</a>
          {plugin ? (
            <a className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4" href="/downloads/affilio.zip" data-track="zip_download">Download WordPress plugin</a>
          ) : (
            <p className="text-[13px] text-t2">The WordPress plugin download is prepared with the hosted package.</p>
          )}
        </div>
      </details>
      <p className="text-[13px] text-t2">We never hold your money.</p>
    </div>
  )
}
