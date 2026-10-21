import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const banned = /\bads?\b|fully automatic|\bpilot\b|\brevolutionary\b|best-converting/gi

function filesUnder(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist' || name === 'fonts') continue
    const path = join(dir, name)
    if (statSync(path).isDirectory()) {
      filesUnder(path, out)
      continue
    }
    if (name.includes('.test.') || name.endsWith('.d.ts')) continue
    if (/\.(tsx|ts|mjs|html|txt|css)$/.test(name)) out.push(path)
  }
  return out
}

function visibleHits(text) {
  const hits = []
  for (const match of text.matchAll(banned)) {
    const before = match.index === 0 ? '' : text[match.index - 1]
    const after = text[match.index + match[0].length] || ''
    const starts = before === '' || /\s/.test(before)
    const ends = after === '' || /[\s.,;:!?]/.test(after)
    if (!starts || !ends) continue
    const lineStart = text.lastIndexOf('\n', match.index - 1) + 1
    const lineEnd = text.indexOf('\n', match.index)
    const line = text.slice(lineStart, lineEnd === -1 ? undefined : lineEnd)
    if (match[0].length <= 3 && !line.trim().includes(' ')) continue
    hits.push(match[0])
  }
  return hits
}

test('rendered public landing copy has no ad, ads, fully automatic, pilot, revolutionary, or best-converting', () => {
  const hits = []
  for (const path of filesUnder(root)) {
    const text = readFileSync(path, 'utf8').replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, ' ').replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, ' ')
    for (const word of visibleHits(text)) {
      hits.push(`${relative(root, path)} contains ${word}`)
    }
  }
  assert.deepEqual(hits, [])
})
