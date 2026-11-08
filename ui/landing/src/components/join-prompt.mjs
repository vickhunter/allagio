import { createElement, useState } from 'react'

export const joinPrompt = 'Join with POST /v1/agent-tokens and send the bearer.'
export const joinLead = 'Give your agent this line to start'
export const joinHeading = `Tell your agent: ${joinPrompt}`

export function JoinPrompt() {
  const [copied, setCopied] = useState(false)
  return createElement('div', { 'data-join-prompt': '1' },
    createElement('p', { className: 'text-sm font-semibold text-t1' }, joinLead),
    createElement('pre', { className: 'mt-3 whitespace-pre-wrap rounded-xl border border-line bg-s0 p-4 font-mono text-sm leading-relaxed text-t1' }, joinPrompt),
    createElement('div', { className: 'mt-3 flex flex-wrap items-center gap-3' },
      createElement('button', {
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
      }, 'Copy setup prompt'),
      createElement('a', { href: '/llms.txt', className: 'inline-flex min-h-11 min-w-11 items-center font-semibold underline underline-offset-4 hover:no-underline' }, 'llms.txt'),
    ),
    createElement('p', { role: 'status', 'aria-live': 'polite', className: 'mt-2 min-h-6 text-sm text-t2' }, copied ? 'Copied.' : ''),
  )
}
