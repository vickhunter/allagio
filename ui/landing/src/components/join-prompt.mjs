import { createElement, useState } from 'react'

export const joinPrompt = 'Join with POST /v1/agent-tokens and send the bearer.'
export const joinLead = 'Give your agent this line to start'
export const joinHeading = `Tell your agent: ${joinPrompt}`

// line=false keeps the copy control and drops the printed payload. The homepage
// shows that payload in Setup details instead of the hero.
export function JoinPrompt({ line = true } = {}) {
  const [copied, setCopied] = useState(false)
  const button = createElement('button', {
    type: 'button',
    className: 'inline-flex min-h-11 min-w-11 items-center rounded-[10px] bg-t1 px-5 text-sm font-semibold text-s0',
    onClick: async () => {
      try {
        await navigator.clipboard.writeText(joinPrompt)
        setCopied(true)
      } catch {
        setCopied(false)
      }
    },
  }, 'Copy setup prompt')
  const row = line
    ? createElement('div', { className: 'mt-3 flex flex-wrap items-center gap-3' },
      button,
      createElement('a', { href: '/llms.txt', className: 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline' }, 'llms.txt'))
    : button
  return createElement('div', { 'data-join-prompt': '1' },
    line ? createElement('p', { className: 'text-sm font-semibold text-t1' }, joinLead) : null,
    line ? createElement('pre', { className: 'mt-3 whitespace-pre-wrap rounded-xl border border-line bg-s0 p-4 font-mono text-sm leading-relaxed text-t1' }, joinPrompt) : null,
    row,
    createElement('p', { role: 'status', 'aria-live': 'polite', className: 'mt-2 min-h-6 text-sm text-t2' }, copied ? 'Copied.' : ''),
  )
}
