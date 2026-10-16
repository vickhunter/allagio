import { useEffect, useMemo, useRef, useState } from 'react'
import { Kbd } from '@/components/ui/input'
import { Blip } from '@/components/Blip'
import { track } from '@/lib/track'

export type Command = { id: string; label: string; keys?: string; href: string }

// ⌘K / Ctrl+K palette (rule 29). Native <dialog>: focus trapped, Esc closes, focus returns.
function nav(h: string) { if (h.startsWith('#')) location.hash = h.slice(1); else location.href = h }

export function CommandPalette({ commands }: { commands: Command[] }) {
  const ref = useRef<HTMLDialogElement>(null)
  const [q, setQ] = useState('')
  const [i, setI] = useState(0)
  const list = useMemo(() => commands.filter((c) => c.label.toLowerCase().includes(q.toLowerCase())), [q, commands])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = (e.target as HTMLElement).closest('input,textarea,select,[contenteditable]')
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); open() }
      else if (!typing && !ref.current?.open) {
        if (e.key === '?') open()
        const hit = commands.find((c) => c.keys && c.keys.toLowerCase() === e.key.toLowerCase())
        if (hit) { track('shortcut', { id: hit.id }); nav(hit.href) }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })
  function open() { setQ(''); setI(0); ref.current?.showModal(); track('palette_open') }
  function go(c: Command) { ref.current?.close(); track('palette_command', { id: c.id }); nav(c.href) }

  return (
    <>
      <button type="button" onClick={open} data-track="palette_button" className="hidden min-h-11 min-w-11 items-center gap-2 rounded-lg border border-line bg-s1 px-2.5 text-[13px] text-t2 hover:bg-s2 sm:inline-flex">
        Jump to… <Kbd>⌘K</Kbd>
      </button>
      <dialog ref={ref} aria-label="Command palette" className="m-auto mt-[14vh] w-[min(560px,92vw)] rounded-2xl border border-line bg-s1 p-0 text-t1 backdrop:bg-inv/30 backdrop:backdrop-blur-sm open:[animation:fade-up_300ms_var(--ease-menu)]"
        onClick={(e) => { if (e.target === ref.current) ref.current?.close() }}>
        <label className="sr-only" htmlFor="palette-q">Search commands</label>
        <input id="palette-q" autoFocus value={q} onChange={(e) => { setQ(e.target.value); setI(0) }} placeholder="Search sections or start as maker / publisher…"
          className="w-full border-b border-line bg-transparent px-4 py-3.5 text-base outline-none"
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') { e.preventDefault(); setI((n) => Math.min(n + 1, list.length - 1)) }
            if (e.key === 'ArrowUp') { e.preventDefault(); setI((n) => Math.max(n - 1, 0)) }
            if (e.key === 'Enter' && list[i]) go(list[i])
          }} role="combobox" aria-expanded="true" aria-controls="palette-list" aria-activedescendant={list[i] ? `cmd-${list[i].id}` : undefined} />
        {list.length ? (
          <ul id="palette-list" role="listbox" aria-label="Commands" className="max-h-80 overflow-auto p-2">
            {list.map((c, n) => (
              <li key={c.id} id={`cmd-${c.id}`} role="option" aria-selected={n === i} onMouseEnter={() => setI(n)} onClick={() => go(c)}
                className={`flex min-h-11 cursor-pointer items-center justify-between rounded-lg px-3 text-sm ${n === i ? 'bg-s2' : ''}`}>
                {c.label}{c.keys && <Kbd>{c.keys.toUpperCase()}</Kbd>}
              </li>
            ))}
          </ul>
        ) : (
          <div className="flex items-center gap-3 p-4 text-sm text-t2"><Blip /> Nothing called “{q}”. Try “maker”, “publisher” or “pricing”.</div>
        )}
        <p className="border-t border-line px-4 py-2 font-mono text-[11px] text-t2">↑↓ move · ↵ open · esc close · ? opens this</p>
      </dialog>
    </>
  )
}
