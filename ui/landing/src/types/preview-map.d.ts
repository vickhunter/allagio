declare module '@/lib/preview-map.mjs' {
  export const slots: readonly string[]
  export const barCount: number
  export const previewTimeoutMs: number
  export const rateLimitPauseMs: number
  export function slotLabel(slot: string): string
  export function slotAfterBar(slot: string): number
  export function previewErrorCopy(code: number): { message: string; next: string }
  export function codeFromStatus(status: number, aborted: boolean): number
  export function parsePreview(body: unknown): { title: string; offer: { name: string; blurb: string } | null; slot: string | null } | null
  export function readPreviewFixture(search: string): string
  export function fixtureBody(name: string): unknown
  export class PreviewStatus extends Error {
    code: number
    constructor(code: number)
  }
}
