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

function layoutOf() {
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
    primary: document.querySelectorAll('[data-primary-action]').length,
    live: document.querySelector('[data-post-preview] [role="status"]')?.getAttribute('aria-live') || '',
  }
}

async function assertPreviewChrome(page, slug, outDir) {
  const metrics = await page.evaluate(layoutOf)
  const axe = await new AxeBuilder({ page }).analyze()
  const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
  writeFileSync(join(outDir, `axe-${slug}.json`), JSON.stringify({ violations: serious }, null, 2))
  if (!metrics.scroll) throw new Error(slug + ' scrolls sideways')
  if (metrics.small.length) throw new Error(slug + ' small targets ' + JSON.stringify(metrics.small))
  if (!metrics.brand) throw new Error(slug + ' shows a retired brand name')
  if (metrics.primary !== 1) throw new Error(slug + ' primary count ' + metrics.primary)
  if (metrics.live !== 'polite') throw new Error(slug + ' aria-live ' + metrics.live)
  if (serious.length) throw new Error(slug + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
  const stored = await page.evaluate(() => Object.keys(localStorage))
  if (stored.length) throw new Error(slug + ' localStorage ' + stored.join(','))
  return metrics
}

function offerFollowsBar(index) {
  const root = document.querySelector('[data-post-preview]')
  const bars = [...root.querySelectorAll('[data-paragraph]')]
  const offer = root.querySelector('[data-offer]')
  if (!offer || bars.length !== 4 || !bars[index]) return false
  const after = bars[index].compareDocumentPosition(offer) & Node.DOCUMENT_POSITION_FOLLOWING
  const beforeNext = index === bars.length - 1 || (offer.compareDocumentPosition(bars[index + 1]) & Node.DOCUMENT_POSITION_FOLLOWING)
  return Boolean(after && beforeNext)
}

async function showPreview(page, origin, route, fixture, width, summary, outDir, posts) {
  const name = route === '/' ? 'home' : 'setup'
  const before = posts.count
  await page.goto(origin + route + '?source=cursor&previewFixture=' + encodeURIComponent(fixture), { waitUntil: 'networkidle' })
  const field = page.getByLabel('Paste a post URL')
  if (await field.count() !== 1) throw new Error(name + ' ' + fixture + ' paste field')
  await page.getByText("Public links only. Private posts and accounts can't be read.").waitFor()
  await field.fill('https://journal.example/start')
  const button = page.getByRole('button', { name: 'Preview this post' })
  await button.click()
  await page.getByText('Reading your post, up to 30s').waitFor({ timeout: 4000 })
  if (fixture === 'success' || fixture === 'after_intro' || fixture === 'mid_article' || fixture === 'before_conclusion' || fixture === 'end_of_article') {
    const slot = fixture === 'success' ? ['After the intro', 0] : {
      after_intro: ['After the intro', 0],
      mid_article: ['Mid-article', 1],
      before_conclusion: ['Before the conclusion', 2],
      end_of_article: ['End of article', 3],
    }[fixture]
    await page.getByRole('heading', { name: 'How to start a journal' }).waitFor()
    await page.getByText('Field Notes', { exact: true }).waitFor()
    await page.getByText(slot[0], { exact: true }).waitFor()
    await page.getByText('Sponsored, via Offer Rails', { exact: true }).waitFor()
    if (!await page.evaluate(offerFollowsBar, slot[1])) throw new Error(fixture + ' offer is not in its slot')
    const approve = page.getByRole('link', { name: 'Approve your placements' })
    const href = await approve.getAttribute('href') || ''
    const back = new URL(href, origin).searchParams.get('return_to') || ''
    if (!href.includes('/auth/login') || !href.includes('source=cursor') || !back.startsWith('/publishers/setup') || !back.includes('source=cursor')) throw new Error('approve href ' + href)
    if (await approve.getAttribute('data-primary-action') !== null) throw new Error('approve is a second primary')
  } else if (fixture === 'nomatch') {
    await page.getByText('No program lines up with this post yet.', { exact: true }).waitFor()
    await page.getByRole('heading', { name: 'Closest programs' }).waitFor()
    await page.getByRole('button', { name: 'Notify me when a match joins' }).waitFor()
    await page.getByRole('button', { name: 'Try another URL' }).waitFor()
    if (await page.locator('[data-offer]').count()) throw new Error('no-match rendered an offer')
    const hrefs = await page.locator('[data-post-preview] [data-suggested]').evaluateAll((els) => els.map((el) => el.getAttribute('href') || ''))
    if (!hrefs.length || hrefs.some((href) => /\/v1\/go|[?&]via=/.test(href))) throw new Error('no-match suggested href ' + hrefs.join(','))
  } else {
    const copy = {
      400: ["That doesn't look like a post URL.", 'Try one like https://example.com/your-post.'],
      403: ['That page is private.', 'Make the post public or paste another link'],
      422: ["We couldn't read that page.", 'Make the post public or paste another link'],
      429: ['Too many tries.', 'Wait a minute, then try again.'],
    }[fixture]
    await page.getByText(copy[0], { exact: true }).waitFor()
    await page.getByText(copy[1], { exact: true }).waitFor()
    const shown = await page.locator('[data-post-preview]').innerText()
    if (/invalid url|blocked address|fetch failed|rate limit exceeded|\{"error"/.test(shown)) throw new Error(fixture + ' showed raw error JSON')
    if (fixture === '403' || fixture === '422') {
      if (/no match/i.test(shown)) throw new Error(fixture + ' said no match')
    }
    if (fixture === '429') {
      if (!await button.isDisabled()) throw new Error('429 left the button enabled')
    } else if (await button.isDisabled()) throw new Error(fixture + ' left the button disabled')
  }
  if (await field.inputValue() !== 'https://journal.example/start') throw new Error(fixture + ' cleared the URL')
  if (posts.count !== before) throw new Error(fixture + ' called /api/preview')
  if (name === 'setup') {
    if ((await page.locator('[data-primary-action]').count()) !== 1) throw new Error('setup gained a primary')
    const primaryName = await page.locator('[data-primary-action]').innerText()
    if (!primaryName.includes('Download the plugin')) throw new Error('setup primary moved ' + primaryName)
  }
  const slug = 'preview-' + name + '-' + fixture + '-' + width
  const metrics = await assertPreviewChrome(page, slug, outDir)
  const shot = name === 'home' ? page.locator('#post-preview') : page.locator('[data-post-preview]')
  await shot.screenshot({ path: join(outDir, slug + '.png') })
  summary.push({ slug, ...metrics, posts: posts.count })
}

async function walkPreview(browser, origin, width, summary, outDir) {
  const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
  const page = await context.newPage()
  const posts = { count: 0 }
  page.on('request', (req) => {
    if (req.method() === 'POST' && new URL(req.url()).pathname === '/api/preview') posts.count += 1
  })
  await page.goto(origin + '/?source=cursor&previewFixture=success', { waitUntil: 'networkidle' })
  await page.getByLabel('Paste a post URL').fill('notaurl')
  await page.getByRole('button', { name: 'Preview this post' }).click()
  await page.getByText("That doesn't look like a post URL.", { exact: true }).waitFor()
  await page.getByText('Try one like https://example.com/your-post.', { exact: true }).waitFor()
  if (posts.count !== 0) throw new Error('invalid URL called /api/preview')
  for (const route of ['/', '/publishers/setup']) {
    for (const fixture of ['success', 'nomatch', '400', '403', '422', '429']) {
      await showPreview(page, origin, route, fixture, width, summary, outDir, posts)
    }
  }
  for (const fixture of ['after_intro', 'mid_article', 'before_conclusion', 'end_of_article']) {
    await showPreview(page, origin, '/', fixture, width, summary, outDir, posts)
  }
  if (width === 390) {
    const liveBefore = posts.count
    await page.goto(origin + '/?source=cursor', { waitUntil: 'networkidle' })
    await page.getByLabel('Paste a post URL').fill('https://journal.example/missing')
    await page.getByRole('button', { name: 'Preview this post' }).click()
    await page.getByText("We couldn't read that page.", { exact: true }).waitFor({ timeout: 4000 })
    await page.getByText('Make the post public or paste another link', { exact: true }).waitFor()
    if (posts.count !== liveBefore + 1) throw new Error('missing route did not request /api/preview ' + posts.count)
    const shown = await page.locator('[data-post-preview]').innerText()
    if (/\{"error"|missing/.test(shown)) throw new Error('missing route showed a raw body')
    await assertPreviewChrome(page, 'preview-missing-' + width, outDir)
    await page.locator('#post-preview').screenshot({ path: join(outDir, 'preview-missing-' + width + '.png') })
  }
  await context.close()
}

async function capturePreviewGroups(browser, origin) {
  const dir = resolve(root, '../../proof/d072b')
  mkdirSync(dir, { recursive: true })
  const lines = []
  const states = [
    ['unreadable', 'empty', 'That page returned no text.'],
    ['groups', 'groups', 'Verified partners'],
    ['nomatch', 'nomatch', 'Notify me when a match joins'],
  ]
  for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 1200 }, reducedMotion: 'reduce' })
    const page = await context.newPage()
    for (const [name, fixture, needle] of states) {
      await page.goto(origin + '/?previewFixture=' + fixture, { waitUntil: 'networkidle' })
      await page.getByLabel('Paste a post URL').fill('https://journal.example/start')
      await page.getByRole('button', { name: 'Preview this post' }).click()
      await page.getByText(needle, { exact: true }).waitFor()
      await page.getByText("Public links only. Private posts and accounts can't be read.").waitFor()
      const shown = await page.locator('[data-post-preview]').innerText()
      if (name === 'unreadable') {
        if (/no match/i.test(shown)) throw new Error('unreadable said no match')
        await page.getByText('Make the post public or paste another link', { exact: true }).waitFor()
      }
      if (name === 'groups') {
        await page.getByRole('heading', { name: 'Suggested programs' }).waitFor()
        await page.getByText('Unverified', { exact: true }).first().waitFor()
        await page.getByText("Not on Offer Rails yet. Payouts go through the company's own affiliate program.").waitFor()
        await page.getByText('Sponsored, via Offer Rails', { exact: true }).waitFor()
        const hrefs = await page.locator('[data-suggested]').evaluateAll((els) => els.map((el) => el.getAttribute('href') || ''))
        if (hrefs.length < 2 || hrefs.some((href) => /\/v1\/go|[?&]via=|utm_/.test(href))) throw new Error('groups href ' + hrefs.join(','))
        if (/commission|earnings/i.test(shown)) throw new Error('groups showed an earnings estimate')
      }
      if (name === 'nomatch') {
        if (await page.locator('[data-offer]').count()) throw new Error('nomatch rendered a verified offer')
        await page.getByRole('heading', { name: 'Closest programs' }).waitFor()
        if (/Sponsored, via Offer Rails/.test(shown)) throw new Error('nomatch showed a verified card')
      }
      const scroll = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)
      if (!scroll) throw new Error(name + ' ' + width + ' scrolls sideways')
      const axe = await new AxeBuilder({ page }).analyze()
      const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
      const critical = serious.filter((item) => item.impact === 'critical')
      lines.push(`${name} ${width} serious=${serious.length} critical=${critical.length}${serious.length ? ' ' + serious.map((item) => item.impact + ':' + item.id).join(',') : ''}`)
      if (serious.length) throw new Error(name + ' ' + width + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
      await page.locator('#post-preview').screenshot({ path: join(dir, name + '-' + width + '.png') })
    }
    await context.close()
  }
  writeFileSync(join(dir, 'axe.txt'), lines.join('\n') + '\n')
}

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
          preview: document.querySelectorAll('[data-post-preview]').length,
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
      if (route === '/' && metrics.preview !== 1) throw new Error('home preview fields ' + metrics.preview)
      if (route !== '/' && metrics.preview) throw new Error(route + ' preview field leaked')
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
        preview: document.querySelectorAll('[data-post-preview]').length,
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
    if (setupMetrics.preview !== 1) throw new Error('setup-' + width + ' preview fields ' + setupMetrics.preview)
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
    await walkPreview(browser, base, width, summary, out)
  }
  await capturePreviewGroups(browser, base)
  writeFileSync(join(out, 'summary.json'), JSON.stringify(summary, null, 2))
  console.log(JSON.stringify(summary, null, 2))
} finally {
  await browser.close()
  server.close()
}
