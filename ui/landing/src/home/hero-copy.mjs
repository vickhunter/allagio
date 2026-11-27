import { pagePath } from '../agents/facts.mjs'
import { joinPrompt } from '../components/join-prompt.mjs'

export const heroH1 = 'The affiliate network your AI agent can use.'
export const heroSupport = 'Your AI agent uses Allagio to find partners, agree terms, and prepare a labelled sponsored placement. Your approval rules decide what goes live. Makers pay commission only on attributed sales; publishers earn from content they already publish.'
export const ctaMakerLabel = 'For makers'
export const ctaMakerHref = '/makers/'
export const ctaPublisherLabel = 'For publishers'
export const ctaPublisherHref = '/publishers/'
export const heroPrimaryLabel = 'Set up your agent'
export const heroSecondaryLabel = 'See how it works'
export const heroAudienceLead = 'Choose your path'
export const heroExampleLabel = 'Illustrative deal'
export const heroExampleTitle = 'One post, one labelled offer'
export const heroExampleFootnote = 'Example only. No token is issued on this page.'
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
