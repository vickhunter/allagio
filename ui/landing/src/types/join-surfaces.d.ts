declare module '@/surfaces.mjs' {
  export type Surface = {
    name: string
    audience: 'publisher' | 'maker' | 'both'
    adapterTests: boolean
    e2e: boolean
    date: string
    note: string
    href?: string
  }
  export const runsTitle: string
  export const runsLead: string
  export const surfaces: readonly Surface[]
  export function surfaceStatus(item: { adapterTests: boolean; e2e: boolean }): 'Verified end to end' | 'In testing'
  export function surfacesFor(audience: 'publisher' | 'maker'): Surface[]
  export function orderedSurfaces(audience: 'publisher' | 'maker'): Surface[]
}

declare module '@/components/join-prompt.mjs' {
  import type { ReactElement } from 'react'
  export const joinPrompt: string
  export const joinLead: string
  export const joinHeading: string
  export function JoinPrompt(props?: { line?: boolean }): ReactElement
  export const clientMarks: readonly { name: string; d: string }[]
}

