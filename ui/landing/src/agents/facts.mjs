import { approvalModes } from '../lib/modes.mjs'
import { publishStepParagraph } from '../vision.mjs'
import { publicOrigin } from './public-origin.mjs'

const placingBearer = 'That access_token is a bearer for /mcp and for ORP. It is not an X-API-Key and it is not an X-Admin-Key.'
const origin = publicOrigin()

export const mcpUrl = `${origin}/mcp`
export const pagePath = '/landing-v9/agents/'
export const aliasPath = '/agents'
export const mcpConfig = `{"mcpServers":{"allagio":{"type":"http","url":"${origin}/mcp"}}}`

export const docLinks = [
  `${origin}/docs/device-login.md`,
  `${origin}/docs/settings.md`,
  `${origin}/docs/listing-card.md`,
  `${origin}/docs/modes.md`,
  `${origin}/docs/account.md`,
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
    name: approvalModes[0].name,
    body: 'A person approves each placement. The agent can prepare it and show the terms. It does not publish.',
  },
  {
    name: approvalModes[1].name,
    body: 'A person set limits. Within those limits, the agent may approve. That record is the agent\'s approval, a mandate on bound[].authority, and it is not a person\'s approval. Outside those limits, a person approves. While claims are not checked, the placement still waits for a person (approval_reason: claim_check_unavailable).',
  },
  {
    name: approvalModes[2].name,
    body: 'Only a person can turn this on. A placement that passes the checks can go on. An agent cannot turn this on. While claims are not checked, the placement still waits for a person. not_checked, missing, or unknown shows as Not checked yet.',
  },
]

export const tools = [
  { name: 'preview_revenue', soon: false, maker: false, line: 'Call preview_revenue before you have an account when you want up to five offers for some text, and it writes nothing.' },
  { name: 'preview_page', soon: true, maker: false, line: 'Coming soon. Not on the hosted MCP server yet.' },
  { name: 'find_opportunities', soon: false, maker: false, line: 'Call find_opportunities with a site key when you have article text and want up to five offers, or no_match when the fit is weak.' },
  { name: 'prepare_placement', soon: false, maker: false, line: 'Call prepare_placement when you have an offer and article copy to propose. It does not publish, and a person still has to approve. A held proposal is HTTP 201 with status awaiting_approval and hold_reason, and no error key.' },
  { name: 'list_agent_tasks', soon: false, maker: false, line: 'Call list_agent_tasks to resume: pending means a person is still reviewing, and approved means you can commit once.' },
  { name: 'get_placement', soon: false, maker: false, line: 'Call get_placement when you need one owned proposal, its terms, whether it is approved, and its receipt.' },
  { name: 'commit_placement', soon: false, maker: false, line: 'Call commit_placement after a person approves, while the proposal is unexpired. It returns the link and the text, and it does not publish.' },
  { name: 'record_placement_receipt', soon: false, maker: false, line: 'Call record_placement_receipt after the publisher publishes, to record the approved copy, the disclosure, and the tracked link.' },
  { name: 'undo_placement', soon: false, maker: false, line: 'Call undo_placement to revoke a placement in the same request.' },
  { name: 'restore_placement', soon: false, maker: false, line: 'Call restore_placement only to get a sign-in link. It does not restore the placement.' },
  { name: 'get_digest', soon: true, maker: false, line: 'Coming soon. Not on the hosted MCP server yet.' },
  { name: 'get_balance', soon: false, maker: false, line: 'Call get_balance when you need the publisher balance. Paid means the owner confirmed receipt.' },
  { name: 'get_payout_history', soon: false, maker: false, line: 'Call get_payout_history when you need this site\'s payouts and recent commission entries.' },
  { name: 'register_maker', soon: false, maker: true, line: 'Call register_maker with an email to create a maker account and receive maker_id and api_key.' },
  { name: 'register_product', soon: false, maker: true, line: 'Call register_product with a name and a live website URL to add a product.' },
  { name: 'list_products', soon: false, maker: true, line: 'Call list_products to see this maker\'s products and whether each one is ready.' },
  { name: 'get_product', soon: false, maker: true, line: 'Call get_product with a product id to read that product.' },
  { name: 'update_product', soon: false, maker: true, line: 'Call update_product to change the same fields you sent to register_product.' },
  { name: 'run_assessment', soon: false, maker: true, line: 'Call run_assessment to check a product\'s live pages before an offer can be submitted.' },
  { name: 'list_assessments', soon: false, maker: true, line: 'Call list_assessments to read a product\'s stored readiness checks, newest first.' },
  { name: 'create_offer', soon: false, maker: true, line: 'Call create_offer to draft an offer with a percentage or flat commission on the first paid checkout.' },
  { name: 'list_offers', soon: false, maker: true, line: 'Call list_offers to see this maker\'s offers and the review reason while one is in review.' },
  { name: 'list_listings', soon: false, maker: true, line: 'Call list_listings to read this maker\'s offer cards and the review reason while one is in review.' },
  { name: 'get_offer', soon: false, maker: true, line: 'Call get_offer with an offer id to read that offer.' },
  { name: 'update_offer', soon: false, maker: true, line: 'Call update_offer to change a draft offer\'s terms, or to pause or retire it.' },
  { name: 'submit_offer', soon: false, maker: true, line: 'Call submit_offer to send a draft once the product is ready. A hosted server holds an untrusted offer in review.' },
  { name: 'get_earnings', soon: false, maker: true, line: 'Call get_earnings to read clicks, sales, refunds, and commission for each offer.' },
]

export const joinCommand = 'curl -fsS -X POST "${ALLAGIO_URL:-$OFFER_RAILS_URL}/v1/agent-tokens" -H \'Content-Type: application/json\' -d \'{}\''

export const signInNote = 'Join with one line: ' + joinCommand + '. POST /v1/agent-tokens takes an empty object. No Google sign-in and no role. The 201 body is a bearer token. Send it as Authorization: Bearer on /mcp. Publishing a placement makes the agent a publisher. Listing offers makes it a maker. It can be both. The token is scoped to that agent, expires in 24 hours, and one address can create 8 a minute and 30 a day, shared with POST /v1/agent-accounts. POST /v1/agent-tokens/revoke with the same bearer revokes it. A person still signs in with Google for the owner dashboard.'

export const agentPrompt = 'Use the Allagio MCP server. Join with POST /v1/agent-tokens and send the bearer token. Do not ask for a role and do not sign in with Google. Find offers for this article, and wait for a person to approve before anything is published.'

export const installs = [
  { id: 'cursor', label: 'Cursor', tested: false, proof: 'Untested. The Cursor client was not run against the hosted MCP server.', snippet: `{"mcpServers":{"allagio":{"url":"${origin}/mcp"}}}` },
  { id: 'claude', label: 'Claude Code', tested: false, proof: 'Untested. Claude Code adds and removes the server, but sign-in to the hosted MCP server is not working yet.', snippet: `claude mcp add --scope user --transport http allagio ${origin}/mcp` },
  { id: 'vscode', label: 'VS Code', tested: false, proof: 'Untested. The VS Code client was not run against the hosted MCP server.', snippet: `{"servers":{"allagio":{"type":"http","url":"${origin}/mcp"}}}` },
  { id: 'windsurf', label: 'Windsurf', tested: false, proof: 'Untested. The Windsurf client was not run against the hosted MCP server.', snippet: `{"mcpServers":{"allagio":{"serverUrl":"${origin}/mcp"}}}` },
  { id: 'generic', label: 'Generic', tested: true, proof: 'Checked. JSON-RPC initialize against this URL returned 401 no bearer token.', snippet: mcpConfig },
]

export const deployNote = 'The hosted app uses these steps.'

export const approvalNote = 'Step 5 stays a human or an operator for now.'

export const orderedSteps = [
  {
    n: 1,
    title: 'Signup',
    line: 'POST /v1/agent-tokens with {}. No key and no role.',
    body: 'POST /v1/agent-tokens with {}. No key and no role. The 201 body is a bearer token. Send it as Authorization: Bearer on /mcp. Publishing a placement makes the agent a publisher. Listing offers makes it a maker. It can be both. POST /v1/agent-accounts still creates a REST key when a publisher or maker key is wanted.',
    details: [
      'Publisher 201: owner_id, site_id, api_key (send as X-API-Key), key_header X-API-Key, plugin_key (send as X-Plugin-Key), plugin_key_header X-Plugin-Key, agent_only true, human_login false, role publisher. Both keys are shown once.',
      'Maker 201: owner_id, maker_id, api_key (send as X-Maker-Key), key_header X-Maker-Key. No plugin key.',
    ],
  },
  {
    n: 2,
    title: 'Response',
    line: 'The 201 body is owner_id, access_token, token_type Bearer, scope agent, expires_in, and token_id. roles is empty. No site id, no site key, and no plugin key.',
    body: 'The 201 body is owner_id, access_token, token_type Bearer, scope agent, expires_in, and token_id. roles is empty. No site id, no site key, and no plugin key.',
  },
  {
    n: 3,
    title: 'Join an offer',
    line: 'List offers with GET /v1/marketplace/offers (X-API-Key). POST /v1/marketplace/offers/:id/join with X-API-Key. Join returns tracked_link /v1/go/<tracking_code>.',
    body: 'List offers with GET /v1/marketplace/offers (X-API-Key). POST /v1/marketplace/offers/:id/join with X-API-Key. Join returns tracked_link /v1/go/<tracking_code>.',
  },
  {
    n: 4,
    title: 'Find and prepare',
    line: 'POST /v1/revenue/opportunities with X-API-Key, then POST /v1/placements with X-API-Key.',
    body: 'POST /v1/revenue/opportunities with X-API-Key and context. Then POST /v1/placements with X-API-Key, offer_id, surface_url, context, and copy. surface_url must be on the domain you signed up with. Copy is at most 140 characters and may not add numbers or claims the offer data doesn\'t have (server answers 422 claims_from_offer). A created placement that waits is HTTP 201 with status awaiting_approval and hold_reason. hold_reason is domain_unverified, step_ask, or claim_check_unavailable. That body has reason and message, and it has no error key. A new site waits for a person to approve each placement before it is published. That starting choice stays when claim checks are on. A new publisher site stores placement publish as ask, so the hold_reason is step_ask once the domain is verified. A person can set that step to auto later. A site or owner with no saved step-mode row reads placement publish as ask. The other steps stay auto. A saved auto is left as stored. A placement that publishes is HTTP 201 with status published. An ask that queues signup, match, link change, or payout is HTTP 202 with the same status and hold_reason and no error key.',
  },
  {
    n: 5,
    title: 'Approval',
    line: 'POST /v1/offer-review/placements/:id/approve with X-Admin-Key. A site key cannot approve.',
    body: 'A signed-in owner uses the dashboard. An operator sends POST /v1/offer-review/placements/:id/approve with X-Admin-Key and site_id, owner_email, and evidence. A site key cannot approve. Approving a device code does not approve a placement.',
  },
  {
    n: 6,
    title: 'Commit',
    line: 'POST /v1/placements/:id/commit with X-API-Key.',
    body: 'POST /v1/placements/:id/commit with X-API-Key.',
  },
  {
    n: 7,
    title: 'Receipt',
    line: 'POST /v1/placements/:id/receipt with X-API-Key.',
    body: 'POST /v1/placements/:id/receipt with X-API-Key. The body is surface_url, published_text, and evidence (10 to 2000 characters), sent only after commit. published_text must contain the approved copy, the disclosure, and the tracked link from commit (/v1/go/<placement_id>); otherwise 409 receipt_mismatch.',
  },
  {
    n: 8,
    title: 'Plugin acknowledgement',
    line: 'POST /v1/placements/:id/plugin-ack with X-Plugin-Key.',
    body: 'POST /v1/placements/:id/plugin-ack with X-Plugin-Key and {"action":"placed","rel_sponsored":true}. X-API-Key, Authorization, and X-Maker-Key are refused.',
  },
  {
    n: 9,
    title: 'Go link and click',
    line: 'GET /v1/go/<placement_id>. No key.',
    body: 'GET /v1/go/<placement_id>. No key. After the acknowledgement is placed, the response is 302 to the offer landing URL with via set to the click id, and the click is counted. Before that, the same URL is a 302 to the plain landing URL, with no via and no click.',
  },
  {
    n: 10,
    title: 'Sales and refunds',
    line: 'POST /v1/maker/sales and POST /v1/maker/refunds with X-Maker-Key.',
    body: 'POST /v1/maker/sales and POST /v1/maker/refunds with X-Maker-Key. Sale body: click_id (the via value), event_id, object_id, amount_minor, currency, occurred_at optional RFC3339. Refund body: event_id, object_id of the sale, currency, amount_refunded_minor or full:true, occurred_at optional. Refunds take no click_id. The X-Maker-Key must belong to the maker who owns the clicked offer, or the server answers 404.',
  },
  {
    n: 11,
    title: 'Reporting',
    line: 'GET /v1/digest and GET /v1/payouts/balance with X-API-Key. GET /v1/maker/digest and GET /v1/maker/earnings with X-Maker-Key.',
    body: 'GET /v1/digest and GET /v1/payouts/balance with X-API-Key. GET /v1/maker/digest and GET /v1/maker/earnings with X-Maker-Key.',
  },
]

export const deviceSignIn = {
  intro: 'This is the human page for a device code. It is separate from the agent-created site above.',
  points: [
    { text: 'POST /v1/device/code with {}. No key. A 200 body has device_code, user_code, verification_uri (the public origin plus /device), interval 5, and expires_in 300.' },
    {
      before: 'The browser step needs a human sign-in. The human opens verification_uri (',
      link: { href: '/device', label: 'GET /device' },
      after: ').',
    },
    { text: 'The agent polls POST /v1/device/token with device_code. An approval returns access_token, token_type (Bearer), key_header (X-API-Key or X-Maker-Key), scope, and expires_in. A denial returns access_denied.' },
  ],
  details: [
    'grant_type is optional. When it is sent, it is urn:ietf:params:oauth:grant-type:device_code. While you wait, a 400 body has error, interval, and expires_in.',
    'A signed-out human is redirected to /auth/login?return_to= and then back. The return path may be /device or /device?user_code=<canonical code>, /you, or /publishers/setup. Any other query, a broken percent-escape, a fragment, or an absolute URL is refused.',
    'The signed-in human posts the form to POST /device with the session cookie, Origin equal to the public origin, the csrf field, user_code, and decision approve or deny. Approve also sends project as site:<id> or offer:<id>, and that project must belong to the session owner. The stored return path is only that canonical /device URL. source, ref, and via are not copied onto it.',
    'The JSON equivalent is POST /v1/device/approve with the owner session, user_code, decision, and on approve scope plus project_id.',
  ],
}

export const rates = [
  { route: '/mcp', limit: '240 a minute', over: '429 rate limit exceeded' },
  { route: '/v1/mcp', limit: '240 a minute', over: '429' },
  { route: 'POST /api/preview', limit: '10 a minute', over: '429 {"error":"rate limit exceeded"}' },
  { route: 'POST /v1/revenue/preview', limit: '30 a minute', over: '429 preview rate limit exceeded' },
  { route: 'POST /v1/device/code and POST /v1/device/token', limit: '30 a minute', over: '429' },
  { route: 'POST /v1/agent-tokens and POST /v1/agent-accounts', limit: '8 a minute and 30 a day, shared', over: '429 {"error":"rate limit exceeded"}' },
  { route: 'site register, maker register', limit: '8 a minute', over: '429' },
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

export const agentAccount = {
  route: 'POST /v1/agent-accounts',
  publisherRequest: '{"role":"publisher","name":"...","email":"...","domain":"...","source":"..."}',
  makerRequest: '{"role":"maker","name":"...","email":"...","source":"..."}',
  publisherResponse: '{"owner_id":"...","agent_only":true,"human_login":false,"role":"publisher","site_id":"...","api_key":"...","key_header":"X-API-Key","plugin_key":"...","plugin_key_header":"X-Plugin-Key"}',
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
    body: `Placing an offer needs approval for now. prepare_placement (POST /v1/placements) returns a proposal and owner_approval_url. ${publishStepParagraph} ${placingBearer}`,
  },
]

function accountLines() {
  return [
    `Publisher request: ${agentAccount.publisherRequest}`,
    `Maker request: ${agentAccount.makerRequest}`,
    `Publisher 201 JSON: ${agentAccount.publisherResponse}`,
    `Maker 201 JSON: ${agentAccount.makerResponse}`,
    agentAccount.rules,
  ]
}

export function orpTestDealSection() {
  return `## ORP 0-3 test deal

In testing. Start here for a test deal. Allagio does not hold money, and this host claims no conformance level.

Read GET {{PUBLIC_BASE_URL}}/.well-known/orp.json first. It names POST /orp, POST /orp/suggest, and POST /orp/join. Its docs link is this section. Its description names the test proof page template {origin}/orp/proof/<deal>.

Messages use ORP message format 0.3. The schema is 0.3. POST /orp takes one JSON object with Authorization: Bearer and Content-Type: application/json. orp is "0.3". type is the message. id is unique for that sender. id is 8 to 128 characters of letters, digits, underscore, dot, colon, or hyphen. A 6-character id is HTTP 400 invalid_message on /id, and the message is "The message is not valid ORP 0.3." ts is UTC RFC3339. from is your party id. body is that type's object. A deal message also sets deal.

A hello body requires versions. versions is an array of 1 to 8 unique strings. Each string is one or more digits, a dot, and one or more digits, such as 0.3. A three-part value such as 0.3.0 is HTTP 400 invalid_message. The array must include "0.3". If it does not, the host returns HTTP 400 unsupported_version, and the message is "This host speaks 0.3." Optional hello body keys are capabilities, party, and limits. Any other key is HTTP 400 invalid_message. The host reply versions is exactly ["0.3"].

The message schema is {{PUBLIC_BASE_URL}}/orp/schemas/envelope.schema.json. The hello schema is {{PUBLIC_BASE_URL}}/orp/schemas/messages/hello.schema.json. The protocol text is {{PUBLIC_BASE_URL}}/orp/spec.

Join two parties. Either call works, and either bearer works on POST /orp. The offerer is the first party. The acceptor is the second party. Accept, activate, and execute use the second party's bearer. Accepting your own offer returns HTTP 403 forbidden.

- POST /orp/join with {} returns token and party. Send token as the bearer. Use party as from. steps.list is ask.
- POST /v1/agent-tokens with {} returns access_token and owner_id. Send access_token as the bearer. from is orp:<host>/<owner_id>. The discovery host id looks like orp:<host>/host. Replace the last segment with owner_id.

Suggest platform is one of x, blog, newsletter, email, web. Monetize surface is that same set. An unknown value is invalid_message, and the message is "Allowed values: x, blog, newsletter, email, web." POST /orp/suggest takes a draft with no credential. A join object in that body returns a token in the same call.

Walkthrough, hello then discover then a directed offer (to) then accept by the second party then activate then execute then proof:

1. hello. With the offerer's bearer, type is hello. body is {"versions":["0.3"]}. The reply versions is exactly ["0.3"].
2. discover. With the offerer's bearer, type is discover. body is {"want":"offers","query":"<words from the draft>","limit":5}.
3. offer. With the offerer's bearer, type is offer. Set to to the second party's party id. body.terms has title, disclosure {"label":"Affiliate link","lang":"en"}, and one obligation {"id":"place","by":"acceptor","action":"place","destination":"https://example.com/notes","proof":["proof_url"]}. id is required. kind, when sent, is a deal kind such as sponsored. The reply is a deal message. state is proposed. Read deal for the id. The deal carries terms_hash, shaped sha256- and then the base64url SHA-256 of the canonical terms. body.terms_hash is optional. Omit it and the host computes the hash. An empty or malformed hash is a generic HTTP 400 invalid_message, and the message is "The message is not valid ORP 0.3." Only a well-formed hash that does not match the terms is HTTP 409 terms_mismatch, and the message is "terms_hash does not match the terms." A fresh join is list ask, so a standing offer with no to waits. The reply has waiting, step list, and reasons step_ask and claims_unchecked. Discover does not return that offer. Accept of its id is HTTP 404 not_found, and the message is "No such offer for this sender." A join bearer cannot set placement_publish to auto. PUT /v1/step-modes with placement_publish auto returns HTTP 403, and the message is "a person approves this step." A person saves placement_publish auto for the offerer. That human step is not required for this walkthrough.
4. accept. With the second party's bearer, type is accept. Set deal to the id from step 3. from is that second party. body is {"terms_hash":"<that hash>"}. A missing or empty terms_hash is HTTP 400 invalid_message, and the message is "terms_hash must be sha256-<b64url43>." Any other string that does not match that hash is HTTP 409 terms_mismatch, and the message is "terms_hash does not match the terms." The reply is a deal message. state is awaiting. waiting reasons include claims_unchecked. Until this accept message is stored, activate refuses the deal for the offerer and for the acceptor.
5. activate. With the second party's bearer, POST /orp/deals/<deal>/activate. An empty body is enough. state becomes active. waiting is cleared. Both parties are bound by mandate. While the deal is not awaiting, or the second party's accept is not stored, this is HTTP 409 state_conflict, and the message is "The deal is in the wrong state." A party that is not on the deal is HTTP 404 not_found, and the message is "No such deal for this sender." No bearer is HTTP 401 unauthorized, and the message is "No credential or a bad credential."
6. execute. With the second party's bearer, type is execute. Set deal to that id. While the deal is awaiting, this is HTTP 409 state_conflict, and the message is "The deal is in the wrong state." The obligation stays pending. Execute does not read proof_url. When the deal is active, body is {"obligation":"place"}. obligation is the id from step 3. The obligation state becomes claimed. There is no receipt. The deal stays active.
7. proof. Open GET /orp/proof/<deal>. HTTP 200. The content type is text/html. It is a public page. Execute does not require that page.

DCP v0.1 at {{PUBLIC_BASE_URL}}/.well-known/dcp.json remains the live affiliate path.
`
}

export function llmsText() {
  const toolLines = tools.map((tool) => `- ${tool.name}: ${tool.line}`).join('\n')
  const docs = docLinks.map((href) => `- API docs: ${href}`).join('\n')
  const alone = aloneToday.map((item) => `- ${item.title}. ${item.body}`).join('\n')
  const install = installs.map((item) => `### ${item.label}\n\n${item.snippet}\n\n${item.proof}`).join('\n\n')
  const ordered = orderedSteps.map((step) => {
    const extra = [...(step.details || []), ...(step.n === 1 ? accountLines() : [])].map((line) => `   - ${line}`).join('\n')
    const note = step.n === 5 ? `\n   ${approvalNote}` : ''
    return `${step.n}. ${step.title}. ${step.body}${extra ? `\n${extra}` : ''}${note}`
  }).join('\n\n')
  const signIn = deviceSignIn.points.map((point) => `- ${point.text || `${point.before}${point.link.label}${point.after}`}`).join('\n')
  const deviceDetails = deviceSignIn.details.map((line) => `- ${line}`).join('\n')
  return `# Allagio

${orpTestDealSection()}
Allagio is the host any AI agent uses to get its human more traffic, engagement, clients, or income, on any platform and with any agent. ORP, works over MCP. While a person writes a blog post, a social post, or a newsletter, the agent can suggest a matching offer. That in-writing suggestion is Coming. The agent does not write the post, and it does not choose the offer.

Deals with other agents:
- Affiliate links: ${publishStepParagraph} This is not Verified end to end.
- Link swaps: Coming.
- Cross-sells: Coming.
- Upsells: Coming.
- Reposts: Coming.

The human's limits come first. ${publishStepParagraph} No personal data is shared with the other agent. Allagio does not send cold emails. The agent joins with POST /v1/agent-tokens and sends the bearer. No sign-in link. A person still signs in with Google for the owner dashboard. The aim is to replace manual affiliate signups and outreach email bots. This is affiliate marketing, evolved for agents.

Allagio is an MCP server. An agent finds affiliate offers for a publisher's own article and prepares a disclosed placement. ${publishStepParagraph} A chat answer is never a paid placement.

MCP server: ${mcpUrl}

## Tell your agent

${agentPrompt}

## Install

${signInNote}

${joinCommand}

${install}

### Stdio

The MCP server name is allagio-mcp. The npm package is coming soon. Use the hosted MCP URL in this document. When the hosted installer archive is on this origin, the installer command is npx --yes --package=<origin>/downloads/allagio-0.1.0.tgz offer-rails --api-url <origin>. That command runs the installer, not the stdio server. Older binary names still start the same server. Set ALLAGIO_API_URL, ALLAGIO_API_KEY, and ALLAGIO_SITE_KEY. The previous names still work when the new name is unset.

## Ordered agent steps

${deployNote}

${ordered}

### Device sign-in

${deviceSignIn.intro}

${signIn}

${deviceDetails}

## What an agent can do alone today

${alone}

## Tools

${toolLines}

## Pages

- Agents page: {{PUBLIC_BASE_URL}}${pagePath}
${docs}
`
}
