declare module '@/agents/facts.mjs' {
  export const mcpUrl: string
  export const pagePath: string
  export const aliasPath: string
  export const mcpConfig: string
  export const signInNote: string
  export const agentPrompt: string
  export const cursorInstallUrl: string
  export const installs: readonly { id: string; label: string; snippet: string }[]
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
