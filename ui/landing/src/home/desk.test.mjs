import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { pagePath } from '../agents/facts.mjs'
import { joinLead, joinPrompt } from '../components/join-prompt.mjs'
import { comingLine, heroH1, heroSupport, manualSetup, noSignIn, pathCaption, promptLine, setupHref, setupLabel, setupSteps } from './hero-copy.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

test('setup copy stays the public join line and the navigation target is the agents page', () => {
  assert.equal(joinPrompt, 'Join with POST /v1/agent-tokens and send the bearer.')
  assert.equal(heroH1, 'Offers, ready for your agent.')
  assert.equal(heroSupport, 'Your agent can find a labelled offer for a post. You keep the limits and the approval.')
  assert.equal(manualSetup, 'Setup is manual. You send the bearer yourself.')
  assert.equal(comingLine, 'Offer suggestions while you write: Coming.')
  assert.equal(noSignIn, 'No sign-in link.')
  assert.deepEqual(setupSteps, [
    'POST /v1/agent-tokens with {}.',
    'Send the access_token as Authorization: Bearer to /mcp.',
    'POST /orp/join with {}.',
    'Read tools/list on /mcp.',
  ])
  assert.equal(setupHref, pagePath)
  assert.equal(setupHref, '/landing-v9/agents/')
  assert.equal(setupLabel, 'Set up your agent')
  assert.match(promptLine, new RegExp(joinPrompt.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  assert.equal(pathCaption.includes('No token is issued'), true)
  const prompt = readFileSync(resolve(root, 'src/components/join-prompt.mjs'), 'utf8')
  assert.match(prompt, /navigator\.clipboard\.writeText\(joinPrompt\)/)
  assert.match(prompt, /Copy setup prompt/)
  assert.match(prompt, /href: '\/llms\.txt'/)
  assert.match(prompt, /setCopied\(false\)/)
})

test('the homepage keeps Copy and adds a separate setup link', () => {
  const app = readFileSync(resolve(root, 'src/home/App.tsx'), 'utf8')
  const sequence = readFileSync(resolve(root, 'src/home/SetupSequence.tsx'), 'utf8')
  const header = readFileSync(resolve(root, 'src/site/Header.tsx'), 'utf8')
  assert.match(app, /heroH1/)
  assert.match(app, /<JoinPrompt line=\{false\} \/>/)
  assert.match(app, /setupHref/)
  assert.match(app, /setupLabel/)
  assert.match(app, /<SetupSequence \/>/)
  assert.match(app, /<Orbit \/>/)
  assert.match(header, /Menu/)
  assert.match(header, /aria-controls="site-menu"/)
  assert.equal(header.includes('mask-image'), false)
  assert.equal(sequence.includes('Coming'), false)
  assert.match(app, /comingLine/)
  assert.equal(app.includes('Someone has the product'), false)
})

test('the no-JS home shell keeps both setup destinations and the public prompt', () => {
  const stamp = readFileSync(resolve(root, 'scripts/stamp-hosted.mjs'), 'utf8')
  const md = readFileSync(resolve(root, '../../affilio-wp/admin/build/landing-v9/home.md'), 'utf8')
  assert.match(stamp, /return_to=%2Fowner%3Fsetup%3Dmaker/)
  assert.match(stamp, /href="\/owner\/settings"/)
  assert.match(stamp, /setupHref/)
  for (const line of [heroH1, heroSupport, manualSetup, setupLabel, joinLead, joinPrompt, comingLine, pathCaption, ...setupSteps, noSignIn]) {
    assert.equal(md.includes(line), true, `home.md dropped ${line}`)
  }
})
