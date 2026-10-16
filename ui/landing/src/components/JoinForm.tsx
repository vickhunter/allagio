import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { track } from '@/lib/track'
import { roleReturn, signInHref, useOfferConfig, useSource } from '@/lib/source'

type Role = 'maker' | 'publisher' | 'both'

export function JoinForm({ role: initial = 'both', cta = 'Sign in with Google', id = 'join' }: { role?: Role; cta?: string; inverse?: boolean; id?: string }) {
  const source = useSource()
  const { install, plugin } = useOfferConfig()
  const [role, setRole] = useState<Role>(initial)
  const [copied, setCopied] = useState(false)
  const href = signInHref(roleReturn(role), source)
  return (
    <div id={id} className="flex flex-col gap-3">
      <fieldset className="flex flex-wrap gap-2">
        <legend className="mb-2 text-[13px] text-t2">I want to</legend>
        {(['maker', 'publisher', 'both'] as Role[]).map((r) => (
          <label key={r} className="inline-flex min-h-11 min-w-11 cursor-pointer items-center rounded-lg border border-line-strong bg-s1 px-3 text-sm has-[:checked]:border-t1 has-[:checked]:font-semibold has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-setup">
            <input type="radio" className="sr-only" name="or-role" checked={role === r} onChange={() => { setRole(r); track('role_pick', { role: r }) }} />
            {r === 'maker' ? 'Sell something' : r === 'publisher' ? 'Monetize my space' : 'Both'}
          </label>
        ))}
      </fieldset>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <Button variant="solid" asChild>
          <a href={href} data-track="join_submit" data-track-role={role} data-track-cta={cta}>{cta}</a>
        </Button>
        <a href={signInHref('/owner/settings', source)} data-track="secondary_click" data-track-to="settings" className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline">MCP and setup</a>
      </div>
      {install ? (
        <div className="rounded-xl border border-line bg-s0 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <label htmlFor="install" className="text-[13px] font-semibold text-t2">Run in your product repository</label>
            <button type="button" className="inline-flex min-h-11 min-w-11 items-center rounded-lg border border-line-strong bg-s1 px-3 text-sm font-semibold" data-track="cta_click" data-track-cta="copy_install" onClick={async () => {
              try {
                await navigator.clipboard.writeText(install)
                setCopied(true)
              } catch {
                setCopied(false)
              }
            }}>Copy command</button>
          </div>
          <textarea id="install" className="mt-2 min-h-11 w-full resize-none rounded-lg border border-line bg-s1 p-3 font-mono text-sm" readOnly rows={3} spellCheck={false} value={install} />
          <p className="copy-status mt-1 text-[13px] text-t2" role="status" aria-live="polite">{copied ? 'Copied.' : 'Paste this as is. One install step.'}</p>
        </div>
      ) : (
        <p className="text-[13px] text-t2">The install command is published with the hosted download. Sign in to register a product meanwhile.</p>
      )}
      {plugin ? (
        <a className="inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4" href="/downloads/affilio.zip" data-track="zip_download">Download WordPress plugin</a>
      ) : (
        <p className="text-[13px] text-t2">The WordPress plugin download is prepared with the hosted package.</p>
      )}
      <p className="text-[13px] text-t2">We never hold your money.</p>
    </div>
  )
}
