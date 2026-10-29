import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { modes as agentModes } from '../agents/facts.mjs'
import { approvalModes } from './modes.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const names = ['Ask me every time', 'Approve within my rules', 'Approve anything that passes the checks']

test('one approval-mode name set, with no YOLO label', () => {
  assert.deepEqual(approvalModes.map((mode) => mode.name), names)
  assert.equal(new Set(approvalModes.map((mode) => mode.id)).size, approvalModes.length)
  const files = [
    'src/content.ts',
    'src/publishers/setup/App.tsx',
    'src/vision.mjs',
    'src/you/App.tsx',
    'src/legal/copy.ts',
    'src/agents/facts.mjs',
    'src/agents/App.tsx',
  ]
  for (const file of files) {
    const text = readFileSync(resolve(root, file), 'utf8')
    assert.equal(/\bYOLO\b/.test(text), false, `${file} still says YOLO`)
  }
  const setup = readFileSync(resolve(root, 'src/publishers/setup/App.tsx'), 'utf8')
  const vision = readFileSync(resolve(root, 'src/vision.mjs'), 'utf8')
  const content = readFileSync(resolve(root, 'src/content.ts'), 'utf8')
  assert.match(setup, /approvalModes/)
  assert.match(content, /approvalModes/)
  for (const name of names) assert.match(vision, new RegExp(name))
  const facts = readFileSync(resolve(root, 'src/agents/facts.mjs'), 'utf8')
  const agents = readFileSync(resolve(root, 'src/agents/App.tsx'), 'utf8')
  assert.match(facts, /approvalModes/)
  assert.match(agents, /modes\.map/)
  assert.deepEqual(agentModes.map((mode) => mode.name), names)
})
