import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { joinPrompt } from '../components/join-prompt.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = [process.env.CHROME_BIN, '/usr/local/bin/google-chrome', '/usr/bin/google-chrome'].find((path) => path && existsSync(path))

test('the home first screen connects the agent with a setup link and the public copy control', { timeout: 60_000 }, async () => {
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
      { width: 320, height: 720 },
      { width: 390, height: 844 },
      { width: 1440, height: 900 },
    ]) {
      const context = await browser.newContext({ viewport, reducedMotion: 'reduce' })
      const page = await context.newPage()
      await page.goto(base, { waitUntil: 'networkidle' })
      const hero = page.locator('#hero')
      assert.equal(await hero.getByRole('link', { name: 'For makers', exact: true }).count(), 1)
      assert.equal(await hero.getByRole('link', { name: 'For publishers', exact: true }).count(), 1)
      assert.equal(await page.getByRole('link', { name: 'I run a site' }).count(), 0)
      const heading = page.getByRole('heading', { level: 1 })
      const headingBox = await heading.boundingBox()
      assert.equal(await heading.innerText(), 'Agents find and close deals autonomously.')
      assert.ok(headingBox && headingBox.y + headingBox.height < viewport.height)
      const setup = page.getByRole('link', { name: 'Set up your agent', exact: true }).first()
      const setupBox = await setup.boundingBox()
      assert.ok(setupBox && setupBox.y + setupBox.height < viewport.height, `${viewport.width} setup link`)
      assert.equal(await setup.getAttribute('href'), '/landing-v9/agents/')
      // The copy control lives in the Connection card below the fold now.
      const copy = page.getByRole('button', { name: 'Copy setup prompt', exact: true })
      assert.equal(await copy.count(), 1)
      await page.locator('.setup-details summary').click()
      const details = page.locator('.setup-details')
      assert.match(await details.innerText(), new RegExp(joinPrompt.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
      assert.equal(await details.locator('a[href="/llms.txt"]').count(), 1)
      const coming = page.locator('#publishers').getByText('Offer suggestions while you write: Works today.')
      assert.equal(await coming.count(), 1)
      assert.equal(await page.locator('header a[href^="/you"]').count(), 0)
      const rules = page.getByRole('link', { name: 'See approval rules', exact: true })
      await rules.click()
      assert.equal(new URL(page.url()).hash, '#approval-rules')
      assert.equal(await page.locator('#approval-rules a').getAttribute('href'), '/you/#approvals')
      const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
      assert.ok(scrollWidth <= viewport.width, `home scroll width ${scrollWidth}px at ${viewport.width}px`)
      assert.equal(await page.getByText('Chapter A').count(), 0)
      assert.equal(await page.getByText('Path A').count(), 0)
      await context.close()
    }
    for (const viewport of [
      { width: 320, height: 720 },
      { width: 390, height: 844 },
      { width: 768, height: 1024 },
      { width: 1440, height: 600 },
    ]) {
      const context = await browser.newContext({ viewport, reducedMotion: 'reduce' })
      const page = await context.newPage()
      const headingIsClear = (id) => {
        const heading = document.getElementById(id)
        const { top, left, bottom } = heading.getBoundingClientRect()
        return top >= document.querySelector('header').getBoundingClientRect().bottom
          && bottom < innerHeight
          && heading.contains(document.elementFromPoint(left + 5, top + 5))
      }
      await page.goto(`${base}agents/#steps`)
      await page.waitForFunction(headingIsClear, 'steps-h')
      const contents = page.getByRole('navigation', { name: 'On this page' })
      if (viewport.width < 768) {
        const box = await contents.boundingBox()
        assert.ok(box && box.y + box.height < 0, 'mobile contents should scroll offscreen')
      }
      await contents.getByRole('link', { name: 'Install', exact: true }).click()
      await page.waitForFunction(headingIsClear, 'install-h')
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `guide overflows at ${viewport.width}px`)
      await context.close()
    }
  } finally {
    await browser.close()
    await server.close()
  }
})
