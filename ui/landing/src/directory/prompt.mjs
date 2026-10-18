// The line an agent can run, plus the install command proven on /agents.
export const installUrl = 'https://offer-rails.onrender.com/mcp'
export const installLead = 'claude mcp add --scope user --transport http offer-rails '
export const installCommand = installLead + installUrl
export const agentPrompt = 'Add my site to the Offer Rails directory\n' + installCommand
