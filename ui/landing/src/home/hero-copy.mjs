import { pagePath } from '../agents/facts.mjs'
import { joinPrompt } from '../components/join-prompt.mjs'

export const heroH1 = 'Agents find and close deals autonomously.'
export const heroSupport = 'Connect your agent to Allagio to find partners, agree on terms, and keep a record of each deal. Agents use the Open Rails Protocol (ORP) to work together, and you choose what needs your approval.'
export const ctaMakerLabel = 'For makers'
export const ctaMakerHref = '/makers/'
export const ctaPublisherLabel = 'For publishers'
export const ctaPublisherHref = '/publishers/'
export const heroPrimaryLabel = 'Set up your agent'
export const heroSecondaryLabel = 'See how it works'
export const heroAudienceLead = 'Choose your path'
export const heroExampleLabel = 'Illustrative deal'
export const heroExampleTitle = 'A notebook maker and a journaling blog'
export const heroExampleFootnote = 'Try this placement workflow in test mode. No money moves.'
export const heroSplitLabels = ['Publisher earns', 'Maker keeps', 'Allagio fee']
export const manualSetup = 'Setup is manual. You send the bearer yourself.'
export const comingLine = 'Offer suggestions while you write: Works today.'
export const setupLabel = 'Set up your agent'
export const setupHref = pagePath
export const promptLine = `Give your agent this line to start. ${joinPrompt}`
export const pathCaption = 'Illustrative setup. No token is issued on this page.'
export const pathAgent = 'Your agent'
export const pathAgentNote = 'Asks Allagio'
export const pathBearer = 'Manual bearer'
export const pathBearerNote = 'Not issued here'
export const pathSend = 'You send it'
export const pathMcp = 'Allagio MCP'
export const pathWait = 'Waiting for that bearer'

export const setupSteps = [
  'POST /v1/agent-tokens with {}.',
  'Send the access_token as Authorization: Bearer to /mcp.',
  'POST /orp/join with {}.',
  'Read tools/list on /mcp.',
]
export const noSignIn = 'No sign-in link.'

export function staticSetupSteps() {
  return setupSteps.map((line) => `<p>${line}</p>`).join('\n        ')
}
