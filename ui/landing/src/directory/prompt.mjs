// The line an agent can run, plus the install command proven on /agents.
export const installUrl = 'https://offer-rails.onrender.com/mcp'
export const installLead = 'claude mcp add --scope user --transport http offer-rails '
export const installCommand = installLead + installUrl
export const agentPrompt = 'Add my site or newsletter to the Offer Rails directory\n' + installCommand
export const makerPrompt = 'Add my product to the Offer Rails directory\n' + installCommand
