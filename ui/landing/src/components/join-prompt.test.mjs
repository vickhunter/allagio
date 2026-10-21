import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import test from 'node:test'
import { JoinPrompt, joinHeading, joinLead, joinPrompt, joinSignInNote } from './join-prompt.mjs'

test('the join prompt and the Copy button render', () => {
  const html = renderToStaticMarkup(createElement(JoinPrompt))
  assert.match(html, new RegExp(joinPrompt))
  assert.match(html, /<button[^>]*>Copy<\/button>/)
  assert.match(html, /href="\/llms.txt"/)
  assert.match(html, new RegExp(joinLead))
  assert.equal(joinHeading, 'Tell your agent: Connect my site to Offer Rails')
  assert.equal(joinSignInNote, 'Your agent will send you one sign-in link to finish.')
  assert.doesNotMatch(html, /Sell something|Earn from my posts|Your agent can connect you/)
  assert.doesNotMatch(joinSignInNote, /on their own|Your agent can connect you/)
})
