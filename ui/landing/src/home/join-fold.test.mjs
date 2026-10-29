import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = ['/usr/local/bin/google-chrome', '/usr/bin/google-chrome'].find((path) => existsSync(path))

test('the home role buttons sit above the fold and the agent line sits under them', { timeout: 60_000 }, async () => {
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
      const sell = page.getByRole('button', { name: 'I sell a product' })
      const run = page.getByRole('link', { name: 'I run a site' })
      const sellBox = await sell.boundingBox()
      const runBox = await run.boundingBox()
      assert.ok(sellBox, `${viewport.width} maker button has a box`)
      assert.ok(runBox, `${viewport.width} publisher link has a box`)
      assert.ok(sellBox.y + sellBox.height < viewport.height, `${viewport.width} maker button bottom ${sellBox.y + sellBox.height}`)
      assert.ok(runBox.y + runBox.height < viewport.height, `${viewport.width} publisher link bottom ${runBox.y + runBox.height}`)
      const story = page.getByText('Agents join with no sign-in. People sign in with Google for the dashboard.')
      const storyBox = await story.boundingBox()
      assert.ok(storyBox && storyBox.y + storyBox.height < viewport.height, `${viewport.width} sign-in story is below the fold`)
      const copies = page.getByRole('button', { name: 'Copy' })
      assert.equal(await copies.count(), 1, `${viewport.width} has one Copy button`)
      const box = await copies.boundingBox()
      assert.ok(box, `${viewport.width} Copy button has a box`)
      const choiceBottom = Math.max(sellBox.y + sellBox.height, runBox.y + runBox.height)
      assert.ok(box.y >= choiceBottom - 1, `${viewport.width} Copy top ${box.y} is above the role choice at ${choiceBottom}`)
      const headerBottom = await page.locator('header').evaluate((el) => el.getBoundingClientRect().bottom)
      assert.ok(sellBox.y >= headerBottom, `${viewport.width} maker button overlaps the header at ${headerBottom}`)
      await context.close()
    }
  } finally {
    await browser.close()
    await server.close()
  }
})
