import { useState } from 'react'
import { PostPreview } from '@/components/PostPreview'
import { Page } from '@/site/Page'
import { signInHref, useSource } from '@/lib/source'
import { approvalModes, personSignsInBecause } from '@/lib/modes.mjs'

type ModeId = (typeof approvalModes)[number]['id']

type StepStatus = 'todo' | 'done'

const primaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)] sm:w-auto'
const secondaryClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-line-strong bg-s0 px-5 font-semibold text-t1 transition-colors duration-150 hover:bg-s2 sm:w-auto'

function StatusPill({ status }: { status: StepStatus }) {
  const done = status === 'done'
  return (
    <span data-step-status className={`inline-flex shrink-0 items-center whitespace-nowrap rounded-full px-2.5 py-1 text-[13px] font-semibold leading-none ${done ? 'bg-pub text-s0' : 'bg-s2 text-t1'}`}>
      {done ? 'Done' : 'To do'}
    </span>
  )
}

// Signed-out setup. Plugin install and site connection cannot be read from here, so each step stays "To do".
// The first open step is the one filled action. Connect and save use the existing owner sign-in.
export function App() {
  const source = useSource()
  const [mode, setMode] = useState<ModeId>('manual')
  const connectHref = signInHref('/owner/connect-site', source)
  const saveHref = signInHref('/owner', source)
  const steps: { title: string; body: string; href: string; action: string; status: StepStatus; track: Record<string, string>; extra?: 'mode' }[] = [
    { title: 'Install the plugin', body: 'Upload it in WordPress and turn it on.', href: '/downloads/offer-rails.zip', action: 'Download the plugin', status: 'todo', track: { 'data-track': 'zip_download' } },
    { title: 'Connect your site', body: 'Sign in with Google, then link the site where you installed the plugin.', href: connectHref, action: 'Connect your site', status: 'todo', track: { 'data-track': 'cta_click', 'data-track-cta': 'connect_site', 'data-track-kind': 'sign_in_click' } },
    { title: 'Choose the approval mode', body: `${approvalModes[0].name} is selected. A new site waits for a person to approve each placement before it is published. That starting choice stays when claim checks are on. Sign in to save it on your account.`, href: saveHref, action: 'Save this mode', status: 'todo', track: { 'data-track': 'cta_click', 'data-track-cta': 'save_mode', 'data-track-kind': 'sign_in_click' }, extra: 'mode' },
  ]
  const primary = steps.findIndex((step) => step.status === 'todo')
  return (
    <Page page="setup">
      <section aria-labelledby="setup-h" className="mx-auto max-w-3xl px-5 pb-16 pt-12 md:pt-16">
        <p className="eyebrow">Publisher setup</p>
        <h1 id="setup-h" className="h-display mt-4 text-[40px] sm:text-6xl">Set up your site.</h1>
        <p className="mt-5 text-lg leading-relaxed text-t2">Three steps if you use WordPress. Then Allagio can place offers on your posts.</p>
        <p className="mt-3 text-[15px] leading-relaxed text-t2">{personSignsInBecause}</p>
        <details className="mt-6 rounded-2xl border border-line bg-s1 p-5">
          <summary className="cursor-pointer text-[15px] font-semibold">I'm not on WordPress</summary>
          <div className="mt-3 text-[15px] leading-relaxed text-t2">
            <p>Agent only for now. A newsletter, Substack, Ghost, or any other place you can paste a link does not use this plugin, and this page does not mint a link.</p>
            <p className="mt-2">Your agent prepares a placement. You sign in with Google to approve it on the dashboard. Then you paste the tracked link. Click counts are in testing.</p>
            <p className="mt-3"><a className="inline-flex min-h-11 items-center font-semibold underline underline-offset-4" href="/publishers/#add-yours">Add your site or newsletter</a></p>
          </div>
        </details>
        <section aria-labelledby="preview-optional" className="mt-8">
          <h2 id="preview-optional" className="font-display text-2xl font-bold">Optional: preview what an offer would look like</h2>
          <p className="mt-2 text-[15px] leading-relaxed text-t2">This is not a setup step. It does not place an offer.</p>
          <div className="mt-4">
            <PostPreview titleLevel={3} />
          </div>
        </section>
        <ol className="mt-6 space-y-4">
          {steps.map((step, index) => (
            <li key={step.title} className="rounded-2xl border border-line bg-s1 p-5">
              <div className="flex flex-nowrap items-center gap-2">
                <p data-step-label className="font-mono text-xs text-pub">Step {index + 1}</p>
                <StatusPill status={step.status} />
              </div>
              <h2 className="mt-2 font-display text-2xl font-bold">{step.title}</h2>
              <p className="mt-2 text-[15px] leading-relaxed text-t2">{step.body}</p>
              {step.extra === 'mode' && (
                <fieldset className="mt-4">
                  <legend className="sr-only">Approval mode</legend>
                  <div className="grid gap-2">
                    {approvalModes.map((item) => (
                      <label key={item.id} className={`flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border px-3 py-2 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-setup ${mode === item.id ? 'border-pub bg-pub-soft' : 'border-line bg-s0'}`}>
                        <input type="radio" className="sr-only" name="approval-mode" value={item.id} checked={mode === item.id} onChange={() => setMode(item.id)} />
                        <span><span className="font-semibold">{item.name}</span> <span className="text-t2">{item.note}</span></span>
                      </label>
                    ))}
                  </div>
                </fieldset>
              )}
              <a href={step.href} data-step-action {...(index === primary ? { 'data-primary-action': true } : {})} {...step.track} className={`mt-5 ${index === primary ? primaryClass : secondaryClass}`}>{step.action}</a>
            </li>
          ))}
        </ol>
      </section>
    </Page>
  )
}
