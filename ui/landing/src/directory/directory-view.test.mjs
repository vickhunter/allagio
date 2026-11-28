import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = [process.env.CHROME_BIN, '/usr/local/bin/google-chrome', '/usr/bin/google-chrome'].find((path) => path && existsSync(path))
const emptyText = 'No makers listed yet. Yours can be first. Add your product below. That step needs an MCP client.'
const errorText = 'The list could not load.'

test('a failed directory load is an error, and an empty list stays empty', { timeout: 60_000 }, async () => {
  assert.ok(chrome, 'chrome is required to render the directory')
  const server = await createServer({
    root,
    configFile: resolve(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 0 },
    logLevel: 'error',
  })
  await server.listen()
  const address = server.httpServer?.address()
  assert.ok(address && typeof address === 'object')
  const base = `http://127.0.0.1:${address.port}/landing-v9/makers/`
  const browser = await chromium.launch({ headless: true, executablePath: chrome, args: ['--no-sandbox', '--disable-gpu'] })
  let calls = 0
  let fail = true
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, reducedMotion: 'reduce' })
    const page = await context.newPage()
    await page.route('**/v1/directory/makers**', async (route) => {
      calls += 1
      if (fail) {
        await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"unavailable"}' })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], next_cursor: null, total: 0 }),
      })
    })
    await page.goto(base, { waitUntil: 'networkidle' })
    const alert = page.getByRole('alert')
    await alert.waitFor()
    assert.equal(await alert.innerText(), errorText)
    assert.equal(await page.getByText(emptyText).count(), 0)
    const opened = calls
    assert.ok(opened >= 1)
    const retried = page.waitForResponse((res) => res.url().includes('/v1/directory/makers') && res.status() === 503)
    await page.getByRole('button', { name: 'Retry' }).click()
    await retried
    await page.getByRole('alert').waitFor()
    assert.equal(calls, opened + 1)
    assert.equal(await page.getByText(emptyText).count(), 0)
    fail = false
    const emptied = page.waitForResponse((res) => res.url().includes('/v1/directory/makers') && res.status() === 200)
    await page.getByRole('button', { name: 'Retry' }).click()
    await emptied
    await page.getByText(emptyText).waitFor()
    assert.equal(calls, opened + 2)
    assert.equal(await page.getByRole('alert').count(), 0)
    await context.close()
  } finally {
    await browser.close()
    await server.close()
  }
})
