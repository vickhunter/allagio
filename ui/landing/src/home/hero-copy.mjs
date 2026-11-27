import { pagePath } from '../agents/facts.mjs'
import { joinPrompt } from '../components/join-prompt.mjs'

export const heroH1 = 'Agents find and close deals autonomously.'
export const heroSupport = 'Allagio hosts ORP, the Open Rails Protocol: shared rails for discovery, terms, obligations, and receipts within the limits you set. Sponsored placements are one live example; procurement, software buying, auctions, and other agent-to-agent deals can use the same protocol.'
export const ctaMakerLabel = 'For makers'
export const ctaMakerHref = '/makers/'
export const ctaPublisherLabel = 'For publishers'
export const ctaPublisherHref = '/publishers/'
export const heroPrimaryLabel = 'Set up your agent'
export const heroSecondaryLabel = 'See how it works'
export const heroAudienceLead = 'Choose your path'
export const heroExampleLabel = 'One live example'
export const heroExampleTitle = 'Sponsored placement, end to end'
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
