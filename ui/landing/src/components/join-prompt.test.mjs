import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import test from 'node:test'
import { JoinPrompt, joinHeading, joinLead, joinPrompt } from './join-prompt.mjs'

test('the join prompt and the Copy button render', () => {
  const html = renderToStaticMarkup(createElement(JoinPrompt))
  assert.match(html, new RegExp(joinPrompt))
  assert.match(html, /<button[^>]*>[\s\S]*Copy setup prompt<\/button>/)
  assert.match(html, /href="\/llms.txt"/)
  assert.match(html, new RegExp(joinLead))
  assert.equal(joinHeading, 'Tell your agent: Join with POST /v1/agent-tokens and send the bearer.')
  assert.match(html, /POST \/v1\/agent-tokens/)
  assert.equal(html.includes('one sign-in link'), false)
  assert.doesNotMatch(html, /Sell something|Earn from my posts|Your agent can connect you|on their own/)
})
