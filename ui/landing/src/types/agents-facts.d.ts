declare module '@/agents/facts.mjs' {
  export const mcpUrl: string
  export const pagePath: string
  export const aliasPath: string
  export const mcpConfig: string
  export const signInNote: string
  export const agentPrompt: string
  export const cursorInstallUrl: string
  export const installs: readonly { id: string; label: string; snippet: string; tested: boolean; proof: string }[]
  export const deployNote: string
  export const approvalNote: string
  export const orderedSteps: readonly { n: number; title: string; body: string; details?: readonly string[] }[]
  export const deviceSignIn: {
    intro: string
    points: readonly ({ text: string; before?: never; link?: never; after?: never } | { text?: never; before: string; after: string; link: { href: string; label: string } })[]
  }
  export const docLinks: readonly string[]
  export const steps: readonly { title: string; body: string }[]
  export const modes: readonly { name: string; body: string }[]
  export const tools: readonly { name: string; soon: boolean; line: string }[]
  export const rates: readonly { route: string; limit: string; over: string }[]
  export const errors: readonly { when: string; see: string }[]
  export const deviceSteps: readonly { title: string; body: string }[]
  export const agentAccount: {
    route: string
    publisherRequest: string
    makerRequest: string
    publisherResponse: string
    makerResponse: string
    rules: string
  }
  export const aloneToday: readonly { title: string; body: string }[]
  export function llmsText(): string
}
