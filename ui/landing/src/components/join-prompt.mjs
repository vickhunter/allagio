import { createElement, useEffect, useRef, useState } from 'react'

export const joinPrompt = 'Join with POST /v1/agent-tokens and send the bearer.'
export const joinLead = 'Give your agent this line to start'
export const joinHeading = `Tell your agent: ${joinPrompt}`

// Small monochrome marks for the clients the setup prompt works in. They are
// simplified glyphs, not official logos: asterisk star (Claude), knot bubble
// (ChatGPT), spark (Gemini), cube (Cursor), bracket (VS Code), ring (Copilot),
// wave (Windsurf), chevron terminal (opencode).
const glyph = (d, title) =>
  createElement('svg', { viewBox: '0 0 16 16', width: 14, height: 14, 'aria-label': title, role: 'img', className: 'shrink-0 opacity-80' },
    createElement('path', { d, fill: 'currentColor' }))

export const clientMarks = [
  { name: 'Claude', d: 'M8 1.5 9.7 6.3 14.5 8 9.7 9.7 8 14.5 6.3 9.7 1.5 8 6.3 6.3Z' },
  { name: 'ChatGPT', d: 'M8 2a6 6 0 1 0 0 12 6 6 0 0 0 0-12Zm0 2 3.5 2v4L8 12l-3.5-2V6L8 4Z' },
  { name: 'Gemini', d: 'M8 1l1.8 5.2L15 8l-5.2 1.8L8 15l-1.8-5.2L1 8l5.2-1.8L8 1Z' },
  { name: 'Cursor', d: 'M8 1.8 14 5v6l-6 3.2L2 11V5l6-3.2Zm0 2L4 6.2v3.6l4 2 4-2V6.2L8 3.8Z' },
  { name: 'VS Code', d: 'M11 2 5 5.5 3 4l-1 1v6l1 1 2-1.5L11 14l3-1.5v-9L11 2Zm0 3.5v5L7 8l4-2.5Z' },
  { name: 'Copilot', d: 'M8 3C5 3 2.5 5.6 2.5 9c0 1.1.9 2 2 2h7c1.1 0 2-.9 2-2C13.5 5.6 11 3 8 3Zm-2 5.5a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5Zm4 0a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5Z' },
  { name: 'Windsurf', d: 'M2 12.5c1.2-4.5 3.6-6.5 6-4.7 1.6 1.2 3.4 1 6-1.3v4.8c-2.6 2.6-5.2 2.8-7.2 1.2-1.4-1.2-3.1-.9-4.8 2.4V12.5Z' },
  { name: 'opencode', d: 'M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v9a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 12.5v-9Zm3 2.7 2.3 1.8L5 9.8v1.7l3.7-2.7v-1.6L5 4.5v1.7Zm4 5.3h4V9.9H9v1.6Z' },
]

const marks = clientMarks.slice(0, 4).map((m) => glyph(m.d, m.name))

const check = createElement('svg', { viewBox: '0 0 16 16', width: 14, height: 14, 'aria-hidden': true, className: 'shrink-0' },
  createElement('path', { d: 'M2.5 8.5 6 12 13.5 4.5', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', strokeLinejoin: 'round' }))

// line=false keeps the copy control and drops the printed payload. The homepage
// shows that payload in Setup details instead of the hero.
export function JoinPrompt({ line = true } = {}) {
  const [copied, setCopied] = useState(false)
  const timer = useRef(null)
  useEffect(() => () => clearTimeout(timer.current), [])
  const button = createElement('button', {
    type: 'button',
    className: copied
      ? 'inline-flex min-h-11 min-w-11 items-center gap-2.5 rounded-[10px] bg-moss px-5 text-sm font-semibold text-s0 transition-colors duration-200'
      : 'inline-flex min-h-11 min-w-11 items-center gap-2.5 rounded-[10px] bg-t1 px-5 text-sm font-semibold text-s0 transition-colors duration-200 hover:bg-[rgb(60_66_55)]',
    onClick: async () => {
      try {
        await navigator.clipboard.writeText(joinPrompt)
        setCopied(true)
        clearTimeout(timer.current)
        timer.current = setTimeout(() => setCopied(false), 2000)
      } catch {
        setCopied(false)
      }
    },
  },
    copied
      ? check
      : createElement('span', { 'aria-hidden': true, className: 'flex items-center gap-1' }, ...marks),
    copied ? 'Copied' : 'Copy setup prompt')
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
