import { pagePath } from '../agents/facts.mjs'
import { joinPrompt } from '../components/join-prompt.mjs'

// Public setup words. The copy payload stays the existing join prompt.
export const heroH1 = 'Connect your agent to Allagio.'
export const manualSetup = 'Setup is manual. POST /v1/agent-tokens with {}. Send the access_token as Authorization: Bearer to /mcp. Then POST /orp/join with {} and read tools/list on /mcp. No sign-in link.'
export const comingLine = 'Suggesting an offer while you write is Coming.'
export const setupLabel = 'Set up your agent'
export const copyClarify = 'Copy setup prompt copies this public line. It does not create a token or open setup.'
export const setupHref = pagePath
export const promptLine = `Give your agent this line to start. ${joinPrompt}`

export const setupSteps = [
  'POST /v1/agent-tokens with {}.',
  'Send the access_token as Authorization: Bearer to /mcp.',
  'POST /orp/join with {}.',
  'Read tools/list on /mcp.',
]

export function staticSetupSteps() {
  return setupSteps.map((line) => `<p>${line}</p>`).join('\n      ')
}
