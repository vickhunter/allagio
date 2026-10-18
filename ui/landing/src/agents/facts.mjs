export const mcpUrl = 'https://offer-rails.onrender.com/mcp'
export const pagePath = '/landing-v9/agents/'
export const aliasPath = '/agents'
export const mcpConfig = '{"mcpServers":{"offer-rails":{"type":"http","url":"https://offer-rails.onrender.com/mcp"}}}'

export const docLinks = [
  '/docs/device-login.md',
  '/docs/settings.md',
  '/docs/listing-card.md',
  '/docs/modes.md',
  '/docs/account.md',
]

export const steps = [
  {
    title: 'Find offers',
    body: 'Send 10 to 12000 characters of the article to find_opportunities (POST /v1/revenue/opportunities). A site grant returns up to 5 offers, or no_match. Any grant can call preview_revenue (POST /v1/revenue/preview). That uses the same match and writes nothing.',
  },
  {
    title: 'Preview a page',
    body: 'preview_page is coming soon and is not on this MCP server yet. Until then, pass the page text to preview_revenue. POST /api/preview can fetch a public URL, and it is not an MCP tool.',
  },
  {
    title: 'Place an offer',
    body: 'Call prepare_placement (POST /v1/placements) with the offer, the page URL, the article text, and copy of at most 140 characters. It returns a proposal and owner_approval_url. A person approves on the dashboard. Then call commit_placement, and after the publisher publishes, record_placement_receipt.',
  },
]

export const modes = [
  {
    name: 'Manual',
    body: 'After the place call, the placement waits. A person approves it on the dashboard. No agent tool can approve.',
  },
  {
    name: 'Auto',
    body: 'Within rules the owner set, hard checks still run. While claims are not checked, the effective mode is forced to manual (approval_reason: claim_check_unavailable), so the placement still waits for a person.',
  },
  {
    name: 'YOLO',
    body: 'Only the dashboard can turn this on. An agent can only tighten a mode. While claims are not checked, this is also forced to manual, so the placement still waits for a person. not_checked, missing, or unknown shows as Not checked yet.',
  },
]

export const tools = [
  { name: 'preview_revenue', soon: false, line: 'Call preview_revenue before you have an account when you want up to five offers for some text, and it writes nothing.' },
  { name: 'preview_page', soon: true, line: 'Coming soon. Not on the hosted MCP server yet.' },
  { name: 'find_opportunities', soon: false, line: 'Call find_opportunities with a site key when you have article text and want up to five offers, or no_match when the fit is weak.' },
  { name: 'prepare_placement', soon: false, line: 'Call prepare_placement when you have an offer and article copy to propose. It does not publish, and a person still has to approve.' },
  { name: 'list_agent_tasks', soon: false, line: 'Call list_agent_tasks to resume: pending means a person is still reviewing, and approved means you can commit once.' },
  { name: 'get_placement', soon: false, line: 'Call get_placement when you need one owned proposal, its terms, whether it is approved, and its receipt.' },
  { name: 'commit_placement', soon: false, line: 'Call commit_placement after a person approves, while the proposal is unexpired. It returns the link and the text, and it does not publish.' },
  { name: 'record_placement_receipt', soon: false, line: 'Call record_placement_receipt after the publisher publishes, to record the approved copy, the disclosure, and the tracked link.' },
  { name: 'undo_placement', soon: false, line: 'Call undo_placement to revoke a placement in the same request.' },
  { name: 'restore_placement', soon: false, line: 'Call restore_placement only to get a sign-in link. It does not restore the placement.' },
  { name: 'get_digest', soon: true, line: 'Coming soon. Not on the hosted MCP server yet.' },
  { name: 'get_balance', soon: false, line: 'Call get_balance when you need the publisher balance. Paid means the owner confirmed receipt.' },
  { name: 'get_payout_history', soon: false, line: 'Call get_payout_history when you need this site\'s payouts and recent commission entries.' },
]

export const signInNote = 'This server needs a Google sign-in today. These snippets do not include an API key. POST /v1/agent-accounts is the no-browser way to get an account.'

export const agentPrompt = 'Use the Offer Rails MCP server. Sign in with Google when it asks. Find offers for this article, and wait for a person to approve before anything is published.'

export const cursorInstallUrl = 'https://cursor.com/en/install-mcp?name=offer-rails&config=eyJ1cmwiOiJodHRwczovL29mZmVyLXJhaWxzLm9ucmVuZGVyLmNvbS9tY3AifQ%3D%3D'

export const installs = [
  { id: 'cursor', label: 'Cursor', snippet: '{"mcpServers":{"offer-rails":{"url":"https://offer-rails.onrender.com/mcp"}}}' },
  { id: 'claude', label: 'Claude Code', snippet: 'claude mcp add --scope user --transport http offer-rails https://offer-rails.onrender.com/mcp' },
  { id: 'vscode', label: 'VS Code', snippet: '{"servers":{"offer-rails":{"type":"http","url":"https://offer-rails.onrender.com/mcp"}}}' },
  { id: 'windsurf', label: 'Windsurf', snippet: '{"mcpServers":{"offer-rails":{"serverUrl":"https://offer-rails.onrender.com/mcp"}}}' },
  { id: 'generic', label: 'Generic', snippet: mcpConfig },
]

export const rates = [
  { route: '/mcp', limit: '240 a minute', over: '429 rate limit exceeded' },
  { route: '/v1/mcp', limit: '240 a minute', over: '429' },
  { route: 'POST /api/preview', limit: '10 a minute', over: '429 {"error":"rate limit exceeded"}' },
  { route: 'POST /v1/revenue/preview', limit: '30 a minute', over: '429 preview rate limit exceeded' },
  { route: 'POST /v1/device/code and POST /v1/device/token', limit: '30 a minute', over: '429' },
  { route: 'POST /v1/agent-accounts, site register, maker register', limit: '8 a minute', over: '429' },
  { route: 'Placement status poll', limit: '1 call / 2 seconds per placement', over: '429' },
  { route: 'GET /v1/go/*', limit: '120 a minute', over: 'untracked 302, never 429' },
]

export const errors = [
  { when: 'Tool failure', see: 'MCP result isError: true, plus the API text' },
  { when: 'Protocol', see: 'JSON-RPC -32601, -32602, or -32603' },
  { when: '400', see: 'invalid payload or invalid url' },
  { when: '401', see: 'missing or invalid bearer' },
  { when: '403', see: 'wrong scope, a project key on a session-only route, blocked address, or origin not allowed' },
  { when: '404', see: 'unknown id' },
  { when: '422 hard check', see: '{check_id, check_text}: disclosure, rel_sponsored, claims_from_offer, copy_length (140), one_per_post, relevance_min, offer_live, revoke_path, cap' },
  { when: 'Preview 422', see: 'fetch failed' },
  { when: '409', see: '{reason, message}: window_expired, paused, not_failed, device_used, account_exists, not_approvable, not_undone, nothing_to_ack, offer_ineligible, offer_unavailable, placement_unavailable, receipt_mismatch, receipt_recorded, publisher_missing, restore_refused, one_per_post' },
  { when: 'Unknown 409 or 422', see: 'The server text, with a generic fallback' },
  { when: 'Device poll 400', see: 'authorization_pending, slow_down, expired, or access_denied' },
  { when: '429', see: 'The rate-limit text above. Wait a minute. Do not retry in a loop.' },
]

export const deviceSteps = [
  {
    title: 'Ask for a code',
    body: 'POST /v1/device/code with {}. A 200 body has device_code, user_code, verification_uri, interval, and expires_in.',
  },
  {
    title: 'A human signs in',
    body: 'The browser step needs a human sign-in. The code response includes verification_uri. This page does not link it.',
  },
  {
    title: 'Poll for the key',
    body: 'POST /v1/device/token with device_code. grant_type is optional. When it is sent, it is urn:ietf:params:oauth:grant-type:device_code. A 200 body has access_token, token_type (Bearer), key_header, scope, and expires_in. While you wait, a 400 body has error, interval, and expires_in.',
  },
]

export const agentAccount = {
  route: 'POST /v1/agent-accounts',
  publisherRequest: '{"role":"publisher","name":"...","email":"...","domain":"...","source":"..."}',
  makerRequest: '{"role":"maker","name":"...","email":"...","source":"..."}',
  publisherResponse: '{"owner_id":"...","agent_only":true,"human_login":false,"role":"publisher","site_id":"...","api_key":"...","key_header":"X-API-Key"}',
  makerResponse: '{"owner_id":"...","agent_only":true,"human_login":false,"role":"maker","maker_id":"...","api_key":"...","key_header":"X-Maker-Key"}',
  rules: 'role is publisher or maker. name is 1 to 80 characters. email contains @, has no spaces, and is at most 255 characters. A publisher also sends domain: non-empty, no spaces, at most 255 characters. source is optional. An unrecognized source is stored as direct. 400 is invalid payload, or role, name, and email required, or domain required. 409 is {reason, message} with reason account_exists and message An account already exists for this email. 429 is {"error":"rate limit exceeded"}.',
}

export const aloneToday = [
  {
    title: 'Discovery',
    body: 'POST /v1/revenue/opportunities takes a site key. POST /v1/revenue/preview takes no key. Both take context of 10 to 12000 characters. The preview writes nothing.',
  },
  {
    title: 'Matching on English pages',
    body: 'POST /api/preview takes {"url":"..."} and scores the fetched page text by lexical token overlap. The tokenizer drops English stopwords and strips the English suffixes ing, ed, and s. It writes nothing.',
  },
  {
    title: 'Recording a sale and a refund',
    body: 'A maker key records a sale at POST /v1/maker/sales and a refund at POST /v1/maker/refunds.',
  },
  {
    title: 'Placing an offer',
    body: 'Placing an offer needs approval for now. prepare_placement (POST /v1/placements) returns a proposal and owner_approval_url. No agent tool can approve it.',
  },
]

export function llmsText() {
  const toolLines = tools.map((tool) => `- ${tool.name}: ${tool.line}`).join('\n')
  const docs = docLinks.map((href) => `- API docs: ${href}`).join('\n')
  const device = deviceSteps.map((step) => `- ${step.title}. ${step.body}`).join('\n')
  const alone = aloneToday.map((item) => `- ${item.title}. ${item.body}`).join('\n')
  const install = installs.map((item) => `### ${item.label}\n\n${item.snippet}`).join('\n\n')
  return `# Offer Rails

Offer Rails is an MCP server. An agent finds affiliate offers for a publisher's own article, prepares a disclosed placement, and a person approves it before it is published. A chat answer is never itself an ad.

MCP server: ${mcpUrl}

## Tell your agent

${agentPrompt}

## Install

${signInNote}

${install}

Add to Cursor: ${cursorInstallUrl}

## Ordered agent steps

Reserved. These steps are not written here.

## What an agent can do alone today

${alone}

## No-browser account

${agentAccount.route}

Publisher request: ${agentAccount.publisherRequest}

Maker request: ${agentAccount.makerRequest}

Publisher 201: ${agentAccount.publisherResponse}

Maker 201: ${agentAccount.makerResponse}

${agentAccount.rules}

## Device code

${device}

## Tools

${toolLines}

A maker grant adds the maker verbs (register_product through submit_offer, and get_earnings).

## Pages

- Agents page: ${pagePath}
${docs}
`
}
