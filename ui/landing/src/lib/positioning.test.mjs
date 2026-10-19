import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const files = [
  'src/home/App.tsx',
  'src/home/HowItWorks.tsx',
  'src/makers/App.tsx',
  'src/publishers/App.tsx',
  'src/content.ts',
  'src/site/Header.tsx',
  'src/site/Footer.tsx',
  'src/site/bits.tsx',
  'src/components/JoinForm.tsx',
  'src/components/SignInChoice.tsx',
  'src/components/commands.ts',
  'src/components/PostPreview.tsx',
  'index.html',
  'makers/index.html',
  'publishers/index.html',
]

const banned = [
  ['ads', /\bads\b/i],
  ['revolution', /revolution/i],
  ['pilot', /\bpilot\b/i],
  ['every post', /every post/i],
]

test('home, makers and publishers copy avoids ads, revolution, pilot and every post', () => {
  const hits = []
  for (const file of files) {
    const text = readFileSync(resolve(root, file), 'utf8')
    for (const [name, pattern] of banned) {
      if (pattern.test(text)) hits.push(`${file} contains ${name}`)
    }
  }
  assert.deepEqual(hits, [])
})
