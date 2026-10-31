declare module '@/directory/state.mjs' {
  export function directoryFailureStatus(error: { status?: number; message?: string } | null): string
  export function directoryFailureCopy(error: { status?: number; message?: string } | null): string
  export function directoryEmptyCopy(kind: 'maker' | 'publisher'): string
  export function directoryChrome(view: string, total: number): {
    heading: boolean
    search: boolean
    error: boolean
    empty: boolean
    list: boolean
    nomatch: boolean
    add: boolean
  }
}

declare module '@/directory/client.mjs' {
  export class DirectoryFailure extends Error {
    status: number
    constructor(status: number, message: string)
  }
  export function wantsFixtures(): boolean
  export function fixtureHeaders(): Record<string, string>
  export function makersUrl(query?: Record<string, string | number | undefined>): string
  export function publishersUrl(query?: Record<string, string | number | undefined>): string
  export function memberUrl(kind: string, slug: string): string
  export function httpsUrl(value: string | null | undefined): string | null
  export function logoSrc(value: string | null | undefined): string | null
  export function memberRoute(pathname: string): { kind: 'maker' | 'publisher'; slug: string } | null
  export function listDirectory(kind: 'maker' | 'publisher', query?: Record<string, string | number | undefined>, fetchImpl?: typeof fetch): Promise<{
    items: Array<{
      id: string
      slug: string
      kind: 'maker' | 'publisher'
      name: string
      url: string
      logo_url: string | null
      summary: string
      languages: string[]
      categories?: string[]
      topics?: string[]
      open_offers?: number
      monthly_pages?: number
      joined_at: string
      verified: boolean
    }>
    next_cursor: string | null
    total: number
  }>
  export function getMember(kind: 'maker' | 'publisher', slug: string, fetchImpl?: typeof fetch): Promise<{
    id: string
    slug: string
    kind: 'maker' | 'publisher'
    name: string
    url: string
    logo_url: string | null
    summary: string
    languages: string[]
    categories?: string[]
    topics?: string[]
    open_offers?: number
    monthly_pages?: number
    joined_at: string
    verified: boolean
  }>
}

declare module '@/directory/prompt.mjs' {
  export const installUrl: string
  export const installLead: string
  export const installCommand: string
  export const agentPrompt: string
  export const makerPrompt: string
}
