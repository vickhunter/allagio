import { useState } from 'react'
import { Page } from '@/site/Page'
import { signInHref, useSource } from '@/lib/source'

const approvalModes = [
  { id: 'manual', name: 'Manual', note: 'You approve each placement.' },
  { id: 'auto', name: 'Auto', note: 'Placements that pass your rules go live.' },
  { id: 'yolo', name: 'YOLO', note: 'Anything that passes the checks goes live. Undo is one tap.' },
] as const

type ModeId = (typeof approvalModes)[number]['id']

const actionClass = 'inline-flex min-h-12 w-full items-center justify-center rounded-xl bg-t1 px-5 font-semibold text-s0 transition-colors duration-150 hover:bg-[rgb(60_66_55)] sm:w-auto'

// Signed-out setup. Plugin install and site connection cannot be read from here, so each step stays "To do".
// Connect and save both go through the existing owner sign-in. This page does not add an API.
export function App() {
  const source = useSource()
  const [mode, setMode] = useState<ModeId>('yolo')
  const connectHref = signInHref('/owner/connect-site', source)
  const saveHref = signInHref('/owner', source)
  return (
    <Page page="setup">
      <section aria-labelledby="setup-h" className="mx-auto max-w-3xl px-5 pb-16 pt-12 md:pt-16">
        <p className="eyebrow">Publisher setup</p>
        <h1 id="setup-h" className="h-display mt-4 text-[40px] sm:text-6xl">Set up your site.</h1>
        <p className="mt-5 text-lg leading-relaxed text-t2">Three steps. Then Offer Rails can place offers on your posts.</p>
        <ol className="mt-10 space-y-4">
          <li className="rounded-2xl border border-line bg-s1 p-5">
            <p className="font-mono text-xs text-pub">Step 1</p>
            <div className="mt-2 flex flex-wrap items-baseline justify-between gap-2">
              <h2 className="font-display text-2xl font-bold">Install the plugin</h2>
              <p className="font-mono text-xs text-t2">To do</p>
            </div>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">Upload it in WordPress and turn it on.</p>
            <a href="/downloads/affilio.zip" data-step-action data-track="zip_download" className={`mt-5 ${actionClass}`}>Download the plugin</a>
          </li>
          <li className="rounded-2xl border border-line bg-s1 p-5">
            <p className="font-mono text-xs text-pub">Step 2</p>
            <div className="mt-2 flex flex-wrap items-baseline justify-between gap-2">
              <h2 className="font-display text-2xl font-bold">Connect your site</h2>
              <p className="font-mono text-xs text-t2">To do</p>
            </div>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">Sign in with Google, then link the site where you installed the plugin.</p>
            <a href={connectHref} data-step-action data-track="cta_click" data-track-cta="connect_site" data-track-kind="sign_in_click" className={`mt-5 ${actionClass}`}>Connect your site</a>
          </li>
          <li className="rounded-2xl border border-line bg-s1 p-5">
            <p className="font-mono text-xs text-pub">Step 3</p>
            <div className="mt-2 flex flex-wrap items-baseline justify-between gap-2">
              <h2 className="font-display text-2xl font-bold">Choose the approval mode</h2>
              <p className="font-mono text-xs text-t2">To do</p>
            </div>
            <p className="mt-2 text-[15px] leading-relaxed text-t2">YOLO is selected. Sign in to save it on your account.</p>
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
            <a href={saveHref} data-step-action data-track="cta_click" data-track-cta="save_mode" data-track-kind="sign_in_click" className={`mt-5 ${actionClass}`}>Save this mode</a>
          </li>
        </ol>
      </section>
    </Page>
  )
}
