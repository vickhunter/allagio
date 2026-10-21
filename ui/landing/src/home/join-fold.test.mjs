import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = ['/usr/local/bin/google-chrome', '/usr/bin/google-chrome'].find((path) => existsSync(path))

test('the home Copy button sits above the fold', { timeout: 60_000 }, async () => {
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
      const copies = page.getByRole('button', { name: 'Copy' })
      assert.equal(await copies.count(), 1, `${viewport.width} has one Copy button`)
      const box = await copies.boundingBox()
      assert.ok(box, `${viewport.width} Copy button has a box`)
      const bottom = box.y + box.height
      assert.ok(bottom < viewport.height, `${viewport.width} Copy bottom ${bottom} is below ${viewport.height}`)
      const headerBottom = await page.locator('header').evaluate((el) => el.getBoundingClientRect().bottom)
      assert.ok(box.y >= headerBottom, `${viewport.width} Copy top ${box.y} overlaps the header at ${headerBottom}`)
      await context.close()
    }
  } finally {
    await browser.close()
    await server.close()
  }
})
