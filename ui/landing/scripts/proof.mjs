import { createServer } from 'node:http'
import { readFileSync, mkdirSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const build = resolve(root, '../../affilio-wp/admin/build')
const out = resolve(root, '../../affilio-wp/e2e/landing-v9-proof')
const install = 'npx --yes --package=https://offer-rails.example/downloads/offer-rails-0.1.0.tgz offer-rails --api-url https://offer-rails.example'

function page(name) {
  return readFileSync(join(build, name), 'utf8')
    .replaceAll('{{.InstallCommand}}', install)
    .replaceAll('{{.BaseURL}}', 'http://127.0.0.1')
    .replace(/\{\{if \.InstallerReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
    .replace(/\{\{if \.PluginReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
}

const pages = { '/': page('home.html'), '/makers': page('makers.html'), '/makers/': page('makers.html'), '/publishers': page('publishers.html'), '/publishers/': page('publishers.html'), '/publishers/setup': page('publishers-setup.html'), '/publishers/setup/': page('publishers-setup.html'), '/protocol': page('protocol.html'), '/protocol/': page('protocol.html') }

function typeFor(file) {
  if (file.endsWith('.css')) return 'text/css; charset=utf-8'
  if (file.endsWith('.woff2')) return 'font/woff2'
  if (file.endsWith('.svg')) return 'image/svg+xml'
  if (file.endsWith('.webp')) return 'image/webp'
  if (file.endsWith('.js')) return 'text/javascript; charset=utf-8'
  return 'application/octet-stream'
}

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  const path = url.pathname
  if (pages[path]) {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'content-security-policy': "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; img-src 'self'; connect-src 'self'" })
    res.end(pages[path])
    return
  }
  if (path === '/auth/login') {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
    res.end('<!doctype html><title>Offer Rails · Sign in</title><main><h1>Sign in</h1><a href="https://accounts.google.com/o/oauth2/v2/auth">Continue with Google</a><p>Return to <a href="/owner">your workspace</a>.</p></main>')
    return
  }
  if (path === '/owner' || path.startsWith('/owner/')) {
    res.writeHead(302, { location: '/auth/login?return_to=' + encodeURIComponent(path + url.search) })
    res.end()
    return
  }
  if (path === '/downloads/affilio.zip') {
    res.writeHead(200, { 'content-type': 'application/zip' })
    res.end('zip')
    return
  }
  if (path.startsWith('/landing-v9/')) {
    const file = join(build, path)
    if (!file.startsWith(build)) {
      res.writeHead(404)
      res.end('missing')
      return
    }
    try {
      const info = statSync(file)
      if (!info.isFile()) throw new Error('not a file')
      res.writeHead(200, { 'content-type': typeFor(file) })
      res.end(readFileSync(file))
    } catch {
      res.writeHead(404)
      res.end('missing')
    }
    return
  }
  res.writeHead(404)
  res.end('missing')
})

await new Promise((resolveListen) => server.listen(0, '127.0.0.1', resolveListen))
const base = 'http://127.0.0.1:' + server.address().port
mkdirSync(out, { recursive: true })

const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/usr/local/bin/google-chrome', args: ['--disable-gpu'] })
const summary = []
try {
  for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
    const page = await context.newPage()
    for (const route of ['/', '/makers', '/publishers']) {
      const slug = (route === '/' ? 'home' : route.slice(1)) + '-' + width
      await page.goto(base + route + '?source=cursor', { waitUntil: 'networkidle' })
      const metrics = await page.evaluate(() => {
        const scroll = document.documentElement.scrollWidth <= window.innerWidth + 1
        const small = [...document.querySelectorAll('a, button')].filter((el) => {
          const style = getComputedStyle(el)
          if (style.display === 'none' || style.visibility === 'hidden') return false
          const box = el.getBoundingClientRect()
          if (box.width < 2 || box.height < 2) return false
          return box.width < 44 || box.height < 44
        }).map((el) => ({ text: (el.innerText || el.getAttribute('aria-label') || '').trim().slice(0, 80), w: Math.round(el.getBoundingClientRect().width), h: Math.round(el.getBoundingClientRect().height) }))
        return {
          scroll,
          small,
          brand: !/distributoor|affilio/i.test(document.body.innerText),
          how: document.querySelectorAll('#how').length,
          publishers: document.querySelectorAll('#publishers').length,
          primary: document.querySelectorAll('[data-primary-action]').length,
          shader: document.querySelectorAll('canvas').length,
          hero: document.querySelector('[data-hero-bg]')?.getAttribute('data-hero-bg') || '',
        }
      })
      const axe = await new AxeBuilder({ page }).analyze()
      const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
      writeFileSync(join(out, `axe-${slug}.json`), JSON.stringify({ violations: serious, incomplete: axe.incomplete.map((item) => item.id) }, null, 2))
      await page.screenshot({ path: join(out, `${slug}.png`), fullPage: true })
      const signIn = page.getByRole('link', { name: /Sign in with Google|Start as a maker|Start as a publisher/ }).first()
      const signHref = await signIn.getAttribute('href')
      const zip = await page.getByRole('link', { name: 'Download WordPress plugin' }).first().getAttribute('href')
      summary.push({ slug, ...metrics, serious: serious.map((item) => item.id), signHref, zip })
      if (!metrics.brand) throw new Error(slug + ' still shows the old brand name')
      if (!metrics.scroll) throw new Error(slug + ' scrolls sideways')
      if (metrics.small.length) throw new Error(slug + ' small targets ' + JSON.stringify(metrics.small))
      if (serious.length) throw new Error(slug + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
      if (!signHref.includes('/auth/login') || !signHref.includes('source=cursor')) throw new Error(slug + ' sign-in href ' + signHref)
      if (zip !== '/downloads/affilio.zip') throw new Error(slug + ' zip ' + zip)
      if (route === '/' && (metrics.how !== 1 || metrics.publishers !== 1)) throw new Error('home anchors ' + metrics.how + '/' + metrics.publishers)
      if (metrics.primary !== 1) throw new Error(slug + ' primary count ' + metrics.primary)
      if (!(await page.getByText('Offer Rails is in an early pilot with a small group of makers and sites.').count())) throw new Error(slug + ' missing the plain pilot line')
      if (route === '/') {
        if (!(await page.getByRole('heading', { name: 'From a product to a paid sale, in five steps.' }).count())) throw new Error('how it works bento missing')
        const folded = (await page.locator('main').innerText()).toLowerCase()
        const howAt = folded.indexOf('from a product to a paid sale')
        const chapterAt = folded.indexOf('chapter a')
        if (howAt < 0 || chapterAt < 0 || howAt > chapterAt) throw new Error('how it works is not before Chapter A ' + howAt + '/' + chapterAt)
        await page.getByRole('link', { name: 'I run a site' }).click()
        await page.waitForURL(/\/publishers\/setup/)
        if (!page.url().includes('source=cursor')) throw new Error('I run a site dropped the source ' + page.url())
        const stepZip = await page.getByRole('link', { name: 'Download the plugin' }).getAttribute('href')
        if (stepZip !== '/downloads/affilio.zip') throw new Error('I run a site step 1 zip ' + stepZip)
        await page.goto(base + '/?source=cursor', { waitUntil: 'networkidle' })
        await page.getByRole('button', { name: 'I sell a product' }).click()
        const sellHref = await page.getByRole('link', { name: 'Continue with Google' }).getAttribute('href')
        const sellTo = new URL(sellHref, base).searchParams.get('return_to') || ''
        if (!sellHref.includes('source=cursor') || !sellTo.includes('setup=maker')) throw new Error('I sell a product href ' + sellHref)
        await page.keyboard.press('Escape')
      }
      if (route === '/makers' && await page.getByRole('link', { name: 'I run a site' }).count()) throw new Error('makers page offers the site choice')
      if (route === '/publishers') {
        await page.getByRole('link', { name: 'I run a site' }).click()
        await page.waitForURL(/\/publishers\/setup/)
        if (!page.url().includes('source=cursor')) throw new Error('publisher page site choice dropped the source ' + page.url())
        const publisherZip = await page.getByRole('link', { name: 'Download the plugin' }).getAttribute('href')
        if (publisherZip !== '/downloads/affilio.zip') throw new Error('publisher page step 1 zip ' + publisherZip)
      }
      if (route === '/' && metrics.shader !== 0) throw new Error('reduced motion still mounted a shader canvas')
      if (route === '/' && metrics.hero !== 'fallback') throw new Error('reduced motion hero is ' + metrics.hero)
    }
    await page.goto(base + '/')
    await page.getByRole('link', { name: 'Sign in with Google' }).first().click()
    await page.waitForURL(/\/auth\/login/)
    if (!page.url().includes('accounts.google.com') && !page.url().includes('/auth/login')) throw new Error('sign-in entry missing')
    const google = await page.getByRole('link', { name: 'Continue with Google' }).getAttribute('href')
    if (!google.includes('accounts.google.com')) throw new Error('google entry ' + google)
    const zipResponse = await page.request.get(base + '/downloads/affilio.zip')
    if (zipResponse.status() !== 200) throw new Error('zip status ' + zipResponse.status())
    const owner = await page.request.get(base + '/owner', { maxRedirects: 0 })
    if (owner.status() !== 302 || !owner.headers().location.includes('/auth/login')) throw new Error('owner did not require sign-in')
    await context.close()
    const protocolContext = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
    const protocolPage = await protocolContext.newPage()
    await protocolPage.goto(base + '/protocol?source=cursor', { waitUntil: 'networkidle' })
    const protocolOk = await protocolPage.evaluate(() => ({
      scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
      title: document.querySelector('h1')?.textContent || '',
      nav: document.querySelector('nav[aria-label="Main"] a[aria-current="page"]')?.getAttribute('href') || '',
    }))
    const protocolAxe = await new AxeBuilder({ page: protocolPage }).analyze()
    const protocolSerious = protocolAxe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
    await protocolPage.screenshot({ path: join(out, 'protocol-' + width + '.png'), fullPage: true })
    summary.push({ slug: 'protocol-' + width, ...protocolOk, serious: protocolSerious.map((item) => item.id) })
    if (!protocolOk.scroll) throw new Error('protocol-' + width + ' scrolls sideways')
    if (protocolOk.title !== 'An open way for agents to share offers.') throw new Error('protocol heading ' + protocolOk.title)
    if (!protocolOk.nav.includes('/protocol/') || !protocolOk.nav.includes('source=cursor')) throw new Error('protocol nav ' + protocolOk.nav)
    if (protocolSerious.length) throw new Error('protocol-' + width + ' axe ' + protocolSerious.map((item) => item.id).join(' '))
    await protocolContext.close()
    const setupContext = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
    const setupPage = await setupContext.newPage()
    await setupPage.goto(base + '/publishers/setup?source=cursor', { waitUntil: 'networkidle' })
    const setupMetrics = await setupPage.evaluate(() => {
      const text = document.body.innerText
      const steps = [...document.querySelectorAll('ol > li')]
      return {
        scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
        small: [...document.querySelectorAll('a, button')].filter((el) => {
          const style = getComputedStyle(el)
          if (style.display === 'none' || style.visibility === 'hidden') return false
          const box = el.getBoundingClientRect()
          if (box.width < 2 || box.height < 2) return false
          return box.width < 44 || box.height < 44
        }).map((el) => ({ text: (el.innerText || el.getAttribute('aria-label') || '').trim().slice(0, 80), w: Math.round(el.getBoundingClientRect().width), h: Math.round(el.getBoundingClientRect().height) })),
        brand: !/affilio|distributoor/i.test(text),
        steps: steps.length,
        actions: steps.map((step) => step.querySelectorAll('[data-step-action]').length),
        todo: (text.match(/To do/g) || []).length,
        nav: document.querySelector('nav[aria-label="Main"] a[aria-current="page"]')?.textContent?.trim() || '',
        primary: document.querySelectorAll('[data-primary-action]').length,
        pills: [...document.querySelectorAll('[data-step-status]')].map((el) => {
          const step = el.closest('li')
          const label = step.querySelector('[data-step-label]').getBoundingClientRect()
          const title = step.querySelector('h2').getBoundingClientRect()
          const box = el.getBoundingClientRect()
          return {
            size: parseFloat(getComputedStyle(el).fontSize),
            sameLine: Math.abs((box.top + box.bottom) / 2 - (label.top + label.bottom) / 2) < 8,
            aboveTitle: box.bottom <= title.top + 1,
          }
        }),
      }
    })
    const setupAxe = await new AxeBuilder({ page: setupPage }).analyze()
    const setupSerious = setupAxe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
    writeFileSync(join(out, `axe-setup-${width}.json`), JSON.stringify({ violations: setupSerious, incomplete: setupAxe.incomplete.map((item) => item.id) }, null, 2))
    await setupPage.screenshot({ path: join(out, 'setup-' + width + '.png'), fullPage: true })
    const connectHref = await setupPage.getByRole('link', { name: 'Connect your site' }).getAttribute('href')
    const connectTo = new URL(connectHref, base).searchParams.get('return_to') || ''
    const yolo = await setupPage.getByRole('radio', { name: /YOLO/ }).isChecked()
    summary.push({ slug: 'setup-' + width, ...setupMetrics, serious: setupSerious.map((item) => item.id), connectHref, yolo })
    if (!setupMetrics.scroll) throw new Error('setup-' + width + ' scrolls sideways')
    if (setupMetrics.small.length) throw new Error('setup-' + width + ' small targets ' + JSON.stringify(setupMetrics.small))
    if (setupSerious.length) throw new Error('setup-' + width + ' axe ' + setupSerious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
    if (!setupMetrics.brand) throw new Error('setup-' + width + ' shows a retired brand name')
    if (setupMetrics.steps !== 3 || setupMetrics.actions.some((count) => count !== 1)) throw new Error('setup-' + width + ' steps ' + setupMetrics.steps + ' actions ' + setupMetrics.actions.join(','))
    if (setupMetrics.todo !== 3) throw new Error('setup-' + width + ' to-do count ' + setupMetrics.todo)
    if (setupMetrics.nav !== 'Publishers') throw new Error('setup-' + width + ' nav ' + setupMetrics.nav)
    if (setupMetrics.primary !== 1) throw new Error('setup-' + width + ' primary count ' + setupMetrics.primary)
    const primaryName = await setupPage.locator('[data-primary-action]').innerText()
    if (!primaryName.includes('Download the plugin')) throw new Error('setup-' + width + ' primary ' + primaryName)
    if (setupMetrics.pills.length !== 3 || setupMetrics.pills.some((pill) => pill.size < 12 || !pill.sameLine || !pill.aboveTitle)) throw new Error('setup-' + width + ' status pills ' + JSON.stringify(setupMetrics.pills))
    if (!yolo) throw new Error('setup-' + width + ' does not preselect YOLO')
    if (!connectHref.includes('/auth/login') || !connectHref.includes('source=cursor') || !connectTo.startsWith('/owner/connect-site') || !connectTo.includes('source=cursor')) throw new Error('setup connect href ' + connectHref)
    const setupZip = await setupPage.getByRole('link', { name: 'Download the plugin' }).getAttribute('href')
    if (setupZip !== '/downloads/affilio.zip') throw new Error('setup zip ' + setupZip)
    await setupContext.close()
  }
  writeFileSync(join(out, 'summary.json'), JSON.stringify(summary, null, 2))
  console.log(JSON.stringify(summary, null, 2))
} finally {
  await browser.close()
  server.close()
}
