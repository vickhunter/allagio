import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = ['/usr/local/bin/google-chrome', '/usr/bin/google-chrome'].find((path) => existsSync(path))

test('the home first screen is the in-write offer, the join line, and your limits', { timeout: 60_000 }, async () => {
  assert.ok(chrome, 'chrome is required to measure the fold')
  const server = await createServer({
    root,
    configFile: resolve(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 0 },
    logLevel: 'error',
  })
  await server.listen()
  const address = server.httpServer?.address()
  assert.ok(address && typeof address === 'object')
  const base = `http://127.0.0.1:${address.port}/landing-v9/`
  const browser = await chromium.launch({ headless: true, executablePath: chrome, args: ['--disable-gpu'] })
  try {
    for (const viewport of [
      { width: 390, height: 844 },
      { width: 1440, height: 900 },
    ]) {
      const context = await browser.newContext({ viewport, reducedMotion: 'reduce' })
      const page = await context.newPage()
      await page.goto(base, { waitUntil: 'networkidle' })
      assert.equal(await page.getByRole('button', { name: 'I sell a product' }).count(), 0, `${viewport.width} still has the maker role button`)
      assert.equal(await page.getByRole('link', { name: 'I run a site' }).count(), 0, `${viewport.width} still has the publisher role link`)
      const offer = page.getByRole('heading', { level: 1 })
      const offerBox = await offer.boundingBox()
      assert.ok(offerBox, `${viewport.width} offer heading has a box`)
      assert.match(await offer.innerText(), /this fits your post, want to monetize it/)
      assert.ok(offerBox.y + offerBox.height < viewport.height, `${viewport.width} offer bottom ${offerBox.y + offerBox.height}`)
      const copies = page.getByRole('button', { name: 'Copy' })
      assert.equal(await copies.count(), 1, `${viewport.width} has one Copy button`)
      const box = await copies.boundingBox()
      assert.ok(box, `${viewport.width} Copy button has a box`)
      assert.ok(box.y + box.height < viewport.height, `${viewport.width} Copy bottom ${box.y + box.height}`)
      assert.equal(await page.locator('header').getByRole('link', { name: 'Your limits' }).count(), 1, `${viewport.width} header limits link`)
      const limits = page.locator('section').filter({ has: page.locator('#hero-h') }).getByRole('link', { name: 'Your limits', exact: true })
      const limitsBox = await limits.boundingBox()
      assert.ok(limitsBox, `${viewport.width} limits link has a box`)
      assert.ok(limitsBox.y + limitsBox.height < viewport.height, `${viewport.width} limits bottom ${limitsBox.y + limitsBox.height}`)
      const href = await limits.getAttribute('href')
      assert.ok(href && href.startsWith('/you'), `${viewport.width} limits href ${href}`)
      const headerBottom = await page.locator('header').evaluate((el) => el.getBoundingClientRect().bottom)
      assert.ok(offerBox.y >= headerBottom - 1, `${viewport.width} offer overlaps the header at ${headerBottom}`)
      assert.equal(await page.getByText('Chapter A').count(), 0)
      assert.equal(await page.getByText('Path A').count(), 0)
      await context.close()
    }
  } finally {
    await browser.close()
    await server.close()
  }
})
