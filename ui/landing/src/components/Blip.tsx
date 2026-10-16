import { cn } from '@/lib/utils'
// BLIP: our own cursor-shaped mascot. Margins and empty states only, never next to money or data (rule 35).
export function Blip({ className, say }: { className?: string; say?: string }) {
  return (
    <span className={cn('inline-flex items-end gap-2', className)}>
      <svg width="34" height="40" viewBox="0 0 34 40" aria-hidden="true" className="shrink-0 [animation:bob_3.2s_ease-in-out_infinite]">
        <path d="M4 3 L29 21 L18 23 L24 36 L18 38.5 L12.5 26 L4 33 Z" fill="rgb(var(--rgb-surface-1))" stroke="rgb(var(--rgb-text-1))" strokeWidth="2.2" strokeLinejoin="round" />
        <g style={{ transformOrigin: '13px 17px' }} className="[animation:blink_4s_infinite]">
          <ellipse cx="11" cy="17" rx="1.6" ry="2.2" fill="rgb(var(--rgb-text-1))" />
          <ellipse cx="16" cy="19.5" rx="1.6" ry="2.2" fill="rgb(var(--rgb-text-1))" />
        </g>
        <circle cx="21" cy="25" r="1.8" fill="rgb(var(--rgb-maker-brand))" opacity=".7" />
      </svg>
      {say && <span className="mb-4 rounded-lg rounded-bl-none border border-line bg-s1 px-2.5 py-1 text-[13px] text-t2">{say}</span>}
    </span>
  )
}
