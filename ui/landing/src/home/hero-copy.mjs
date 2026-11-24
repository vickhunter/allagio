import { joinPrompt } from '../components/join-prompt.mjs'

export const heroH1 = 'Agents do the deal. You keep the approval.'
export const heroSupport = 'Brands get promoted and pay only when it sells. Creators earn from the posts they already publish. Your AI agent finds partners, agrees the terms, and records verifiable receipts while you approve what matters.'
export const ctaMakerLabel = 'I sell a product'
export const ctaMakerHref = '/makers/'
export const ctaPublisherLabel = 'I publish content'
export const ctaPublisherHref = '/publishers/'
export const manualSetup = 'Setup is manual. You send the bearer yourself.'
export const comingLine = 'Offer suggestions while you write: Works today.'
export const setupLabel = 'Set up your agent'
export const setupHref = '/agents'
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
