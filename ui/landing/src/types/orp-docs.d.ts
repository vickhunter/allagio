declare module '@/protocol/generated/catalog.mjs' {
  export const catalog: {
    version: string
    license: string
    schemas: {
      file: string
      version: string
      id: string
      slug: string
      href: string
      title: string
      description: string
      kind: string
      type: string
      required: string[]
      anyOf: string[][]
      sections: {
        name: string
        description: string
        closed: boolean
        fields: { name: string; required: boolean; type: string; description: string }[]
      }[]
      valid: { name: string; json: string }[]
      invalid: { name: string; why: string; json: string }[]
    }[]
    versions: string[]
    conformance: string[]
    sdk: string[]
    sdkReadme: string
    exampleCount: number
    invalidCount: number
  }
}

declare module '@/protocol/route.mjs' {
  export function routeFor(pathname: string): { name: string; version: string; slug: string }
}

declare module '@/protocol/hosted.mjs' {
  export const discoveryName: string
  export const handledMessages: readonly string[]
  export const hostOnlyTypes: readonly string[]
  export const hostOnlyRefusal: string
  export const joinSteps: string
  export const noMoney: string
  export const hostSpeechSpeaks: string
  export const hostSpeechUnknown: string
  export function hostSpeech(doc: unknown): string
  export const receiptRefusal: string
  export const otherRefusal: string
  export const payRefusal: string
}
