import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createServer } from 'vite'
import { chromium } from 'playwright-core'
import test from 'node:test'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { joinPrompt } from '../components/join-prompt.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const chrome = [
  process.env.CHROME_PATH,
  '/usr/local/bin/google-chrome',
  '/usr/bin/google-chrome',
  '/home/ubuntu/.local/bin/google-chrome',
].filter(Boolean).find((path) => existsSync(path))

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
      { width: 390, height: 844 },
      { width: 1440, height: 900 },
    ]) {
      const context = await browser.newContext({ viewport, reducedMotion: 'reduce' })
      const page = await context.newPage()
      await page.goto(base, { waitUntil: 'networkidle' })
      assert.equal(await page.getByRole('link', { name: 'I sell a product', exact: true }).count(), 1)
      assert.equal(await page.getByRole('link', { name: 'I publish content', exact: true }).count(), 1)
      assert.equal(await page.getByRole('link', { name: 'I run a site' }).count(), 0)
      const heading = page.getByRole('heading', { level: 1 })
      const headingBox = await heading.boundingBox()
      assert.equal(await heading.innerText(), 'Agents do the deal. You keep the approval.')
      assert.ok(headingBox && headingBox.y + headingBox.height < viewport.height)
      const setup = page.getByRole('link', { name: 'Set up your agent', exact: true }).first()
      const setupBox = await setup.boundingBox()
      assert.ok(setupBox && setupBox.y + setupBox.height < viewport.height, `${viewport.width} setup link`)
      assert.equal(await setup.getAttribute('href'), '/agents')
      // Switch to Agent tab in hero to verify quickstart copy & llms.txt.
      await page.getByRole('tab', { name: 'Agent', exact: true }).click()
      const copy = page.getByRole('button', { name: 'Copy setup prompt', exact: true })
      assert.equal(await copy.count(), 1)
      const agentPanel = page.locator('#hero-panel-agent')
      assert.match(await agentPanel.innerText(), new RegExp(joinPrompt.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
      assert.ok((await agentPanel.locator('a[href="/llms.txt"]').count()) >= 1)
      // Verify simulator slot and subtle light editorial bands exist.
      assert.equal(await page.locator('#simulator-slot').count(), 1)
      assert.ok((await page.locator('.band-subtle').count()) >= 2)
      if (viewport.width < 1280) {
        await page.getByRole('button', { name: 'Menu', exact: true }).click()
      }
      const templatesLink = page.locator('header').getByRole('link', { name: 'Templates', exact: true })
      assert.equal(await templatesLink.count(), 1)
      const templatesBox = await templatesLink.boundingBox()
      assert.ok(templatesBox && templatesBox.height >= 44)
      assert.ok((await templatesLink.getAttribute('href'))?.startsWith('/use-cases'))
      const overflow = await page.evaluate(() => ({
        inner: window.innerWidth,
        client: document.documentElement.clientWidth,
        scroll: document.documentElement.scrollWidth,
      }))
      assert.equal(overflow.scroll <= overflow.inner + 1, true, JSON.stringify(overflow))
      assert.equal(await page.getByText('Chapter A').count(), 0)
      assert.equal(await page.getByText('Path A').count(), 0)
      await context.close()
    }
  } finally {
    await browser.close()
    await server.close()
  }
})
