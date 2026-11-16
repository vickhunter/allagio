import { createServer } from 'node:http'
import { existsSync, readFileSync, mkdirSync, statSync } from 'node:fs'
import { dirname, extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const build = resolve(scriptDir, '../../../affilio-wp/admin/build')
const outDir = process.argv[2]
if (!outDir) {
  console.error('usage: node scripts/proof-phaseA.mjs <out-dir>')
  process.exit(2)
}
mkdirSync(outDir, { recursive: true })

const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.txt': 'text/plain', '.md': 'text/markdown' }
const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64')

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://x')
  if (url.pathname === '/shot-img.png') {
    res.writeHead(200, { 'Content-Type': 'image/png' })
    res.end(pixel)
    return
  }
  if (url.pathname === '/' || url.pathname === '/index.html') {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
    res.end(readFileSync(join(build, 'home.html')))
    return
  }
  if (url.pathname.startsWith('/landing-v9/')) {
    const file = join(build, url.pathname)
    if (file.startsWith(build) && existsSync(file) && statSync(file).isFile()) {
      res.writeHead(200, { 'Content-Type': mime[extname(file)] || 'application/octet-stream' })
      res.end(readFileSync(file))
      return
    }
  }
  res.writeHead(404).end('missing')
})
await new Promise((r) => server.listen(0, '127.0.0.1', r))
const origin = `http://127.0.0.1:${server.address().port}`

const browser = await chromium.launch({ headless: true, args: ['--disable-gpu'] })

async function shot(width, path, file, settle = 800) {
  const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } })
  await page.goto(origin + path, { waitUntil: 'domcontentloaded' })
  if (file.includes('trust')) {
    await page.locator('#trust').scrollIntoViewIfNeeded()
  }
  if (file.includes('showcase')) {
    await page.locator('[role="tablist"]').scrollIntoViewIfNeeded()
  }
  await page.waitForTimeout(settle)
  await page.screenshot({ path: join(outDir, file) })
  await page.close()
  return page
}

console.log('trust 1440')
await shot(1440, '/', 'trust-1440.png', 1200)
console.log('trust 390')
await shot(390, '/', 'trust-390.png', 1200)
console.log('showcase 390')
await shot(390, '/', 'showcase-390.png', 1200)

// pasted-link result card with stubbed API incl. image + demo
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
await page.route('**/api/preview', (route) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({
    title: 'How I keep a field journal',
    author: 'R. Iyer',
    excerpt: 'A pocket notebook, one rule per page, and ten quiet minutes each morning.',
    image: `${origin}/shot-img.png`,
    demo: true,
    offer: { name: 'Baron Fig', verified: true },
    slot: 'mid_article',
    suggestion: { kind: 'partner_mention', name: 'Baron Fig', line: 'Sponsored — Baron Fig: pocket notebooks for field notes.', note: 'Closest listed offer.' },
  }),
}))
await page.goto(origin + '/', { waitUntil: 'domcontentloaded' })
await page.locator('#post-preview').scrollIntoViewIfNeeded()
await page.locator('input[name="post-url"]').fill('https://journal.example/field-notes')
await page.locator('[data-post-preview] form button[type="submit"]').click()
console.log('preview submitted')
await page.waitForSelector('[data-post-preview] article', { timeout: 15000 })
await page.locator('[data-post-preview] article').scrollIntoViewIfNeeded()
await page.waitForTimeout(900)
await page.screenshot({ path: join(outDir, 'preview-1440.png') })
await page.close()

await browser.close()
server.close()
console.log('shots written to', outDir)
