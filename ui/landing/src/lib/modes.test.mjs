import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { approvalModes } from './modes.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const names = ['Ask me every time', 'Approve within my rules', 'Approve anything that passes the checks']
const retiredMode = 'YO' + 'LO'

test('one approval-mode name set, with no retired mode label', () => {
  assert.deepEqual(approvalModes.map((mode) => mode.name), names)
  assert.equal(new Set(approvalModes.map((mode) => mode.id)).size, approvalModes.length)
  const files = [
    'src/content.ts',
    'src/publishers/setup/App.tsx',
    'src/vision.mjs',
    'src/you/App.tsx',
    'src/legal/copy.ts',
  ]
  for (const file of files) {
    const text = readFileSync(resolve(root, file), 'utf8')
    assert.equal(text.includes(retiredMode), false, `${file} still says the retired mode name`)
  }
  const setup = readFileSync(resolve(root, 'src/publishers/setup/App.tsx'), 'utf8')
  const vision = readFileSync(resolve(root, 'src/vision.mjs'), 'utf8')
  const content = readFileSync(resolve(root, 'src/content.ts'), 'utf8')
  assert.match(setup, /approvalModes/)
  assert.match(vision, /approvalModes/)
  assert.match(content, /approvalModes/)
})

function filesUnder(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist') continue
    const path = join(dir, name)
    if (statSync(path).isDirectory()) {
      filesUnder(path, out)
      continue
    }
    out.push(path)
  }
  return out
}

test('the retired mode name does not appear in landing source except a doc that quotes the old name', () => {
  const src = resolve(root, 'src')
  const hits = []
  for (const path of filesUnder(src)) {
    const text = readFileSync(path, 'utf8')
    if (!text.includes(retiredMode)) continue
    const lines = text.split('\n')
    lines.forEach((line, index) => {
      if (!line.includes(retiredMode)) return
      if (/old name/i.test(line)) return
      hits.push(`${relative(root, path)}:${index + 1}`)
    })
  }
  assert.deepEqual(hits, [])
})
