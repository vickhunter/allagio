import { defaultPublicBaseURL } from '../public-base.mjs'

// The line an agent can run, plus the install command proven on /agents.
export const installUrl = `${defaultPublicBaseURL}/mcp`
export const installLead = 'claude mcp add --scope user --transport http allagio '
export const installCommand = installLead + installUrl
export const agentPrompt = 'Add my site or newsletter to the Allagio directory\n' + installCommand
export const makerPrompt = 'Add my product to the Allagio directory\n' + installCommand
