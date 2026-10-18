import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { agentAccount, agentPrompt, aloneToday, approvalNote, cursorInstallUrl, deployNote, deviceSteps, docLinks, errors, installs, llmsText, mcpUrl, modes, orderedSteps, pagePath, rates, signInNote, steps, tools } from './facts.mjs'

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
  assert.match(text, /This server needs a Google sign-in today/)
  assert.match(text, /claude mcp add --scope user --transport http offer-rails/)
  assert.match(text, /"serverUrl":"https:\/\/offer-rails\.onrender\.com\/mcp"/)
  assert.match(text, new RegExp(cursorInstallUrl.replaceAll('?', '\\?')))
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
  assert.deepEqual(installs.filter((item) => item.tested).map((item) => item.id), ['claude', 'generic'])
  for (const tool of tools) assert.match(text, new RegExp(tool.name))
  for (const href of docLinks) assert.match(text, new RegExp(href))
  assert.equal(banned.test(withoutOfferJoin(text)), false)
})

test('only page preview and digest are coming soon', () => {
  assert.deepEqual(tools.filter((tool) => tool.soon).map((tool) => tool.name), ['preview_page', 'get_digest'])
})

test('public agent copy stays on the documented words', () => {
  const blob = JSON.stringify({ steps, modes, tools, rates, errors, deviceSteps, agentAccount, aloneToday, installs, signInNote, agentPrompt, orderedSteps })
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
