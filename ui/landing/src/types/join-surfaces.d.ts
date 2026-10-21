declare module '@/surfaces.mjs' {
  export type SurfaceState = 'Live' | 'Beta' | 'Planned'
  export type Surface = {
    name: string
    state: SurfaceState
    audience: 'publisher' | 'maker' | 'both'
    note: string
  }
  export const surfaces: readonly Surface[]
  export function surfacesFor(audience: 'publisher' | 'maker'): Surface[]
  export function splitSurfaces(audience: 'publisher' | 'maker'): { live: Surface[]; planned: Surface[] }
}

declare module '@/components/join-prompt.mjs' {
  import type { ReactElement } from 'react'
  export const joinPrompt: string
  export const joinLead: string
  export const joinHeading: string
  export function JoinPrompt(): ReactElement
}
