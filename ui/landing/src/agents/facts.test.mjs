import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { agentAccount, agentPrompt, aloneToday, approvalNote, deployNote, deviceSignIn, docLinks, errors, installs, llmsText, mcpUrl, modes, orderedSteps, pagePath, rates, signInNote, steps, tools } from './facts.mjs'

const banned = /grok|marketplace|\blisted\b|github\.com|distributoor|affilio|connector/i

function withoutOfferJoin(text) {
  return text.replaceAll('/v1/marketplace/', '/v1/offer-join/')
}

test('llms.txt matches the agent facts', () => {
  const file = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../public/llms.txt'), 'utf8')
  const text = llmsText()
  assert.equal(file, text)
  assert.match(text, new RegExp(mcpUrl.replaceAll('.', '\\.')))
  assert.match(text, new RegExp(pagePath))
  assert.equal(text.includes('\n- Alias:'), false)
  assert.match(text, /GET \/device/)
  assert.match(text, new RegExp(deployNote.replaceAll('.', '\\.')))
  assert.match(text, new RegExp(approvalNote))
  assert.equal(orderedSteps.map((step) => step.n).join(','), '1,2,3,4,5,6,7,8,9,10,11')
  assert.match(text, /X-Plugin-Key/)
  assert.match(text, /X-Admin-Key/)
  assert.match(text, /POST \/v1\/agent-accounts/)
  assert.match(text, /"agent_only":true/)
  assert.match(text, /"human_login":false/)
  assert.match(text, /"key_header":"X-API-Key"/)
  assert.match(text, /"key_header":"X-Maker-Key"/)
  assert.match(text, /The browser step needs a human sign-in/)
  assert.match(text, /What an agent can do alone today/)
  assert.match(text, /Placing an offer needs approval for now/)
  assert.match(text, /\/mcp takes a Bearer token/)
  assert.equal(text.includes('Google sign-in today'), false)
  assert.equal(text.includes('/mcp needs the Google sign-in'), false)
  assert.equal(text.includes('Sign in with Google when it asks'), false)
  assert.match(text, /claude mcp add --scope user --transport http offer-rails/)
  assert.match(text, /"serverUrl":"https:\/\/offer-rails\.onrender\.com\/mcp"/)
  assert.equal(text.includes('cursor.com/en/install-mcp'), false)
  assert.equal(text.includes('## No-browser account'), false)
  assert.equal(text.includes('## Device code'), false)
  assert.equal(text.includes('register_product through submit_offer'), false)
  assert.match(text, /Publisher request:/)
  assert.match(text, /urn:ietf:params:oauth:grant-type:device_code/)
  assert.match(text, /register_maker/)
  assert.match(text, /list_listings/)
  assert.match(text, /get_earnings/)
  assert.match(text, /## Tell your agent/)
  assert.match(text, /## Ordered agent steps/)
  assert.match(text, /### Device sign-in/)
  assert.equal(text.includes('YOUR_API_KEY'), false)
  assert.equal(text.includes('Reserved. These steps are not written'), false)
  for (const item of installs) {
    if (item.tested) assert.equal(item.proof.startsWith('Checked.'), true)
    else assert.match(item.proof, /^Untested\./)
    assert.equal(item.snippet.includes('Authorization'), false)
  }
  assert.deepEqual(installs.filter((item) => item.tested).map((item) => item.id), ['generic'])
  for (const tool of tools) assert.match(text, new RegExp(tool.name))
  for (const href of docLinks) assert.match(text, new RegExp(href))
  assert.equal(banned.test(withoutOfferJoin(text)), false)
})

test('ordered steps name the hosted join, prepare, receipt, and sale bodies', () => {
  const text = llmsText()
  const step = (n) => orderedSteps.find((item) => item.n === n)
  assert.equal(step(3).line, step(3).body)
  for (const n of [4, 7, 10]) assert.equal(step(n).line === step(n).body, false)
  for (const n of [3, 4, 7, 10]) {
    assert.match(text, new RegExp(step(n).body.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(step(3).body, /List offers with GET \/v1\/marketplace\/offers \(X-API-Key\)/)
  assert.match(step(3).body, /Join returns tracked_link \/v1\/go\/<tracking_code>/)
  assert.match(step(4).body, /surface_url must be on the domain you signed up with/)
  assert.match(step(4).body, /at most 140 characters/)
  assert.match(step(4).body, /422 claims_from_offer/)
  assert.match(step(7).body, /surface_url, published_text, and evidence \(10 to 2000 characters\), sent only after commit/)
  assert.match(step(7).body, /tracked link from commit \(\/v1\/go\/<placement_id>\); otherwise 409 receipt_mismatch/)
  assert.equal(step(7).body.includes('and /go/<placement_id>'), false)
  assert.match(step(10).body, /click_id \(the via value\), event_id, object_id, amount_minor, currency, occurred_at optional RFC3339/)
  assert.match(step(10).body, /amount_refunded_minor or full:true/)
  assert.match(step(10).body, /Refunds take no click_id/)
  assert.match(step(10).body, /maker who owns the clicked offer, or the server answers 404/)
  assert.match(signInNote, /send its api_key as Authorization: Bearer <api_key>/)
  assert.match(signInNote, /The plugin key \(X-Plugin-Key\) and keys from accounts made in a browser are refused with 401/)
  assert.match(text, /send its api_key as Authorization: Bearer <api_key>/)
  assert.match(agentPrompt, /POST \/v1\/agent-accounts and send the api_key as a Bearer token to \/mcp/)
  assert.equal(agentPrompt.includes('Sign in with Google'), false)
  const claude = installs.find((item) => item.id === 'claude')
  assert.equal(claude.tested, false)
  assert.equal(claude.proof, 'Untested. Claude Code adds and removes the server, but sign-in to the hosted MCP server is not working yet.')
  assert.equal(claude.snippet, 'claude mcp add --scope user --transport http offer-rails https://offer-rails.onrender.com/mcp')
  assert.equal(text.includes('Needs authentication'), false)
  assert.equal(text.includes('Tested: add and remove'), false)
  assert.equal(JSON.stringify(orderedSteps).includes('Needs authentication'), false)
})

test('only page preview and digest are coming soon', () => {
  assert.deepEqual(tools.filter((tool) => tool.soon).map((tool) => tool.name), ['preview_page', 'get_digest'])
})

test('public agent copy stays on the documented words', () => {
  const blob = JSON.stringify({ steps, modes, tools, rates, errors, deviceSignIn, agentAccount, aloneToday, installs, signInNote, agentPrompt, orderedSteps })
  assert.equal(banned.test(withoutOfferJoin(blob)), false)
  assert.equal(banned.test(withoutOfferJoin(llmsText())), false)
  for (const name of ['Manual', 'Auto', 'YOLO']) assert.match(blob, new RegExp(name))
  assert.match(blob, /rate limit exceeded/)
  assert.match(blob, /preview rate limit exceeded/)
  assert.match(blob, /invalid payload/)
  assert.match(blob, /fetch failed/)
  assert.match(blob, /claim_check_unavailable/)
  assert.match(blob, /Not checked yet/)
})
