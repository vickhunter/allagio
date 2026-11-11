declare module '@/lib/preview-groups.mjs' {
  export const publicLinksNote: string
  export const suggestedNote: string
  export const closestLead: string
  export const notifyLabel: string
  export const notifyStorageKey: string
  export const notifyLimit: number
  export const readNext: string
  export type ReadFailure = 'private' | 'unread' | 'empty'
  export type VerifiedOffer = { verified: true; name: string; blurb: string }
  export type UnverifiedProgram = {
    verified: false
    name: string
    blurb: string
    program_url: string
    source_url: string
    checked_at: string
    topics: string[]
  }
  export type PreviewSuggestion = { kind: string; name: string; line: string; note: string }
  export type PreviewGroups = {
    title: string
    author: string
    excerpt: string
    suggestion: PreviewSuggestion | null
    verified: VerifiedOffer[]
    suggested: UnverifiedProgram[]
    closest: UnverifiedProgram[]
    readFailure: ReadFailure | null
    kind: 'groups' | 'closest' | 'unreadable'
    slot: string | null
  }
  export function readFailureCopy(reason: ReadFailure): { message: string; next: string }
  export function classifyReadFailure(body: unknown): ReadFailure | null
  export function includeUnverifiedFixture(search: string): boolean
  export function groupsFixtureBody(): unknown
  export function groupPreview(body: unknown, options?: { includeFixture?: boolean }): PreviewGroups
  export function adaptPreview(body: unknown, options?: { includeFixture?: boolean }): PreviewGroups
  export function orderedNames(grouped: PreviewGroups): string[]
  export function readNotifyIntent(storage: Pick<Storage, 'getItem'>): string[]
  export function storeNotifyIntent(storage: Pick<Storage, 'getItem' | 'setItem'>, url: string): string[]
}
