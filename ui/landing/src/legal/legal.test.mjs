import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const landing = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const repo = join(landing, '../..')
const draftLine = 'Draft. This text is not final and has not been approved yet.'
const robots = '<meta name="robots" content="noindex, nofollow" />'

for (const page of ['privacy', 'terms']) {
  test(page + ' is a final, indexable legal page', () => {
    const html = readFileSync(join(landing, page, 'index.html'), 'utf8')
    assert.equal(html.includes(robots), false, page + ' html still blocks indexing')
    assert.equal(html.includes(draftLine), false, page + ' html still shows the draft line')
    const built = readFileSync(join(repo, 'affilio-wp/admin/build/landing-v9', page, 'index.html'), 'utf8')
    assert.equal(built.includes(draftLine), false, page + ' built html still shows the draft line')
    assert.equal(built.includes(robots), false, page + ' built html still blocks indexing')
    const app = readFileSync(join(landing, 'src', page, 'App.tsx'), 'utf8')
    assert.match(app, /LegalDocument/)
    const copy = readFileSync(join(landing, 'src/legal/copy.ts'), 'utf8')
    assert.equal(copy.includes('draftBanner'), false)
    assert.equal(copy.includes('[OPERATOR NAME]'), false)
    assert.equal(copy.includes('[CONTACT EMAIL]'), false)
    assert.equal(/draft of|draft service|draft terms|early draft|not an approved|not yet approved/i.test(copy), false, page + ' copy still reads as a draft')
    const document = readFileSync(join(landing, 'src/legal/Document.tsx'), 'utf8')
    assert.equal(document.includes('data-draft-banner'), false)
  })
}
