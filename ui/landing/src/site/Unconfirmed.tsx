import type { ReactNode } from 'react'
// Pricing caveat: Victor has NOT confirmed "free to join". Every instance renders through here, visibly marked.
export function Unconfirmed({ children, inverse = false }: { children: ReactNode; inverse?: boolean }) {
  return (
    <span data-unconfirmed="pricing" title="Draft copy: 'free to join' is not confirmed by Victor yet">
      {children}
      <span className={`ml-1 rounded border border-dashed px-1 align-[1px] font-mono text-[10px] uppercase tracking-wider ${inverse ? 'border-tinv2 text-tinv' : 'border-maker text-maker'}`}>unconfirmed</span>
    </span>
  )
}
