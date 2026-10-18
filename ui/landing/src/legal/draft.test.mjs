import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const landing = join(dirname(fileURLToPath(import.meta.url)), '../..')
const repo = join(landing, '../..')
const draftLine = 'Draft. This text is not final and has not been approved yet.'
const robots = '<meta name="robots" content="noindex, nofollow" />'

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist' || name === '.git') continue
    const file = join(dir, name)
    const info = statSync(file)
    if (info.isDirectory()) walk(file, out)
    else if (/sitemap/i.test(name)) out.push(file)
  }
  return out
}

for (const page of ['privacy', 'terms']) {
  test(page + ' shows the draft line and is not indexed', () => {
    const html = readFileSync(join(landing, page, 'index.html'), 'utf8')
    assert.ok(html.includes(robots), page + ' html is missing the robots meta')
    assert.ok(html.includes(draftLine), page + ' html is missing the draft line')
    const built = readFileSync(join(repo, 'affilio-wp/admin/build/landing-v9', page, 'index.html'), 'utf8')
    assert.ok(built.includes(draftLine), page + ' built html is missing the draft line')
    assert.ok(built.includes(robots), page + ' built html is missing the robots meta')
    const app = readFileSync(join(landing, 'src', page, 'App.tsx'), 'utf8')
    assert.match(app, /LegalDocument/)
    const copy = readFileSync(join(landing, 'src/legal/copy.ts'), 'utf8')
    assert.ok(copy.includes(`export const draftBanner = '${draftLine}'`))
    const document = readFileSync(join(landing, 'src/legal/Document.tsx'), 'utf8')
    assert.match(document, /data-draft-banner/)
    assert.match(document, /\{draftBanner\}/)
    const banner = document.slice(document.indexOf('data-draft-banner') - 80, document.indexOf('{draftBanner}') + 20)
    assert.equal(/sr-only|hidden|display:\s*none|visibility:\s*hidden/.test(banner), false)
  })
}

test('no sitemap lists the draft privacy or terms pages', () => {
  const files = [...walk(landing), ...walk(join(repo, 'affilio-wp'))]
  for (const file of files) {
    const text = readFileSync(file, 'utf8')
    assert.equal(/\/privacy\b|\/terms\b/.test(text), false, file)
  }
})
