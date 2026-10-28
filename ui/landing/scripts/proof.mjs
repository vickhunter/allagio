import { createServer } from 'node:http'
import { copyFileSync, readFileSync, mkdirSync, statSync, writeFileSync } from 'node:fs'
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

function hosted(name) {
  return readFileSync(join(build, 'landing-v9', name), 'utf8')
}

const agentsHtml = hosted('agents/index.html')
const pages = { '/': page('home.html'), '/makers': page('makers.html'), '/makers/': page('makers.html'), '/publishers': page('publishers.html'), '/publishers/': page('publishers.html'), '/publishers/setup': page('publishers-setup.html'), '/publishers/setup/': page('publishers-setup.html'), '/protocol': page('protocol.html'), '/protocol/': page('protocol.html'), '/privacy': hosted('privacy/index.html'), '/privacy/': hosted('privacy/index.html'), '/terms': hosted('terms/index.html'), '/terms/': hosted('terms/index.html'), '/agents': agentsHtml, '/agents/': agentsHtml, '/landing-v9/agents': agentsHtml, '/landing-v9/agents/': agentsHtml, '/landing-v9/agents/index.html': agentsHtml }

function typeFor(file) {
  if (file.endsWith('.css')) return 'text/css; charset=utf-8'
  if (file.endsWith('.woff2')) return 'font/woff2'
  if (file.endsWith('.svg')) return 'image/svg+xml'
  if (file.endsWith('.png')) return 'image/png'
  if (file.endsWith('.ico')) return 'image/x-icon'
  if (file.endsWith('.webp')) return 'image/webp'
  if (file.endsWith('.js')) return 'text/javascript; charset=utf-8'
  return 'application/octet-stream'
}

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  const path = url.pathname
  if (pages[path] || path === '/protocol' || path.startsWith('/protocol/')) {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'content-security-policy': "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; img-src 'self'; connect-src 'self'" })
    res.end(pages[path] || pages['/protocol/'])
    return
  }
  if (path === '/llms.txt') {
    res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' })
    res.end(readFileSync(join(build, 'landing-v9/llms.txt')))
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
  if (path === '/downloads/offer-rails.zip') {
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

async function assertAgents(browser, origin, width, summary, outDir) {
  const llms = await fetch(origin + '/llms.txt')
  const llmsType = llms.headers.get('content-type') || ''
  const llmsBody = await llms.text()
  if (llms.status !== 200 || !llmsType.includes('text/plain')) throw new Error('llms ' + llms.status + ' ' + llmsType)
  for (const needle of ['https://offer-rails.onrender.com/mcp', 'find_opportunities', 'preview_page', 'get_digest', 'register_product', 'get_earnings', 'list_listings', '/landing-v9/agents/', 'POST /v1/agent-accounts', 'What an agent can do alone today', 'The browser step needs a human sign-in', 'POST /v1/maker/sales', 'POST /v1/maker/refunds', 'Placing an offer needs approval for now', 'POST /v1/agent-tokens', 'No Google sign-in and no role', 'curl -fsS -X POST "$OFFER_RAILS_URL/v1/agent-tokens"', 'claude mcp add --scope user --transport http offer-rails', '"serverUrl":"https://offer-rails.onrender.com/mcp"', '## Tell your agent', '## Ordered agent steps', '/docs/modes.md', 'Publisher request:', 'urn:ietf:params:oauth:grant-type:device_code', 'Join returns tracked_link /v1/go/<tracking_code>', '422 claims_from_offer', '10 to 2000 characters', '409 receipt_mismatch', 'Refunds take no click_id', 'sign-in to the hosted MCP server is not working yet']) {
    if (!llmsBody.includes(needle)) throw new Error('llms missing ' + needle)
  }
  if (!/GET \/device/.test(llmsBody)) throw new Error('llms does not name GET /device')
  if (!llmsBody.includes('The hosted app uses these steps.')) throw new Error('llms missing the deploy note')
  if (!llmsBody.includes('Step 5 stays a human or an operator for now.')) throw new Error('llms missing the approval note')
  if (!llmsBody.includes('X-Plugin-Key') || !llmsBody.includes('X-Admin-Key')) throw new Error('llms missing an auth header from the ordered steps')
  if (llmsBody.includes('cursor.com/en/install-mcp') || llmsBody.includes('## No-browser account') || llmsBody.includes('## Device code') || llmsBody.includes('register_product through submit_offer') || llmsBody.includes('Needs authentication')) throw new Error('llms still repeats an old section')
  if (/grok|marketplace|\blisted\b|github\.com|distributoor|affilio|connector/i.test(llmsBody.replaceAll('/v1/marketplace/', '/v1/offer-join/'))) throw new Error('llms has forbidden wording')
  const alias = await fetch(origin + '/agents', { redirect: 'manual' })
  const aliasBody = await alias.text()
  if (alias.status !== 200 || aliasBody !== agentsHtml) throw new Error('agents page ' + alias.status)
  const staticCopy = await fetch(origin + '/landing-v9/agents/')
  if ((await staticCopy.text()) !== agentsHtml) throw new Error('agents static copy differs from /agents')
  const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce', permissions: ['clipboard-read', 'clipboard-write'] })
  const page = await context.newPage()
  await page.goto(origin + '/agents', { waitUntil: 'networkidle' })
  if (!page.url().endsWith('/agents')) throw new Error('agents landed on ' + page.url())
  const facts = await page.evaluate(() => {
    const text = document.body.innerText
    const small = [...document.querySelectorAll('a, button')].filter((el) => {
      const style = getComputedStyle(el)
      if (style.display === 'none' || style.visibility === 'hidden') return false
      const box = el.getBoundingClientRect()
      if (box.width < 2 || box.height < 2) return false
      return box.width < 44 || box.height < 44
    }).map((el) => ({ text: (el.innerText || el.getAttribute('aria-label') || '').trim().slice(0, 80), w: Math.round(el.getBoundingClientRect().width), h: Math.round(el.getBoundingClientRect().height) }))
    return {
      scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
      small,
      brand: !/distributoor|affilio/i.test(text),
      forbidden: /grok|marketplace|\blisted\b|github\.com|connector/i.test(text.replaceAll('/v1/marketplace/', '/v1/offer-join/')),
      steps: document.querySelectorAll('[data-agent-steps] > li').length,
      ordered: document.querySelectorAll('#steps ol > li').length,
      height: document.documentElement.scrollHeight,
      headings: ['No-browser account', 'Device code', 'Find offers'].filter((name) => [...document.querySelectorAll('h2, h3')].some((el) => el.textContent.trim() === name)),
      soon: [...document.querySelectorAll('[data-tool][data-soon="true"]')].map((el) => el.getAttribute('data-tool')),
      livePreview: document.querySelector('[data-tool="preview_page"]')?.innerText.includes('Coming soon') && !document.querySelector('[data-tool="find_opportunities"]')?.innerText.includes('Coming soon'),
      modes: ['Manual', 'Auto', 'YOLO'].every((name) => text.includes(name)),
      stub: text.includes('claim_check_unavailable') && text.includes('Not checked yet'),
      url: document.querySelector('[data-mcp-url]')?.textContent || '',
      alternate: document.querySelector('link[rel="alternate"]')?.getAttribute('href') || '',
      alternateType: document.querySelector('link[rel="alternate"]')?.getAttribute('type') || '',
      primary: document.querySelectorAll('[data-primary-action]').length,
      tram: getComputedStyle(document.querySelector('.tram')).animationName,
      rates: text.includes('rate limit exceeded') && text.includes('preview rate limit exceeded') && text.includes('{"error":"rate limit exceeded"}'),
      errors: ['invalid payload', 'invalid url', 'blocked address', 'origin not allowed', 'fetch failed', 'authorization_pending', 'isError: true'].every((item) => text.includes(item)),
      alone: document.querySelectorAll('[data-alone] > li').length,
      human: text.includes('The browser step needs a human sign-in'),
      signup: text.includes('POST /v1/agent-accounts') && (document.body.textContent || '').includes('"key_header":"X-API-Key"') && (document.body.textContent || '').includes('"key_header":"X-Maker-Key"'),
      deviceLinks: [...document.querySelectorAll('a')].map((el) => el.getAttribute('href') || '').filter((href) => /\/device(?!\/|[-a-z0-9])/.test(href)),
    }
  })
  const axe = await new AxeBuilder({ page }).analyze()
  const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
  writeFileSync(join(outDir, `axe-agents-${width}.json`), JSON.stringify({ violations: serious }, null, 2))
  const tabLabels = []
  for (const spec of [
    { name: 'Cursor', untested: true },
    { name: 'Claude Code', untested: true },
    { name: 'VS Code', untested: true },
    { name: 'Windsurf', untested: true },
    { name: 'Generic', untested: false },
  ]) {
    const tab = page.getByRole('tab', { name: spec.name })
    const label = (await tab.innerText()).replace(/\s+/g, ' ').trim()
    tabLabels.push(label)
    if (!label.startsWith(spec.name)) throw new Error('agents-' + width + ' tab lost its name ' + label)
    if (spec.untested && !label.includes('untested')) throw new Error('agents-' + width + ' tab missing untested badge ' + label)
    if (!spec.untested && /untested/i.test(label)) throw new Error('agents-' + width + ' tested tab badged ' + label)
  }
  if (await page.locator('[data-cursor-install]').count()) throw new Error('agents-' + width + ' still shows the untested Cursor deeplink')
  await page.getByRole('tab', { name: 'Generic' }).click()
  await page.getByRole('button', { name: 'Copy endpoint' }).click()
  await page.locator('[data-copy-status="endpoint"]').getByText('Copied', { exact: true }).waitFor()
  const copiedUrl = await page.evaluate(() => navigator.clipboard.readText())
  await page.getByRole('button', { name: 'Copy config' }).click()
  await page.locator('[data-copy-status="config"]').getByText('Copied', { exact: true }).waitFor()
  const copiedConfig = await page.evaluate(() => navigator.clipboard.readText())
  await page.screenshot({ path: join(outDir, 'agents-' + width + '.png'), fullPage: true })
  summary.push({ slug: 'agents-' + width, ...facts, serious: serious.map((item) => item.id), copiedUrl, copiedConfig, tabLabels, height: facts.height })
  if (!facts.scroll) throw new Error('agents-' + width + ' scrolls sideways')
  if (facts.small.length) throw new Error('agents-' + width + ' small targets ' + JSON.stringify(facts.small))
  if (serious.length) throw new Error('agents-' + width + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
  if (!facts.brand || facts.forbidden) throw new Error('agents-' + width + ' forbidden wording')
  if (facts.steps !== 0 || facts.ordered !== 11) throw new Error('agents-' + width + ' steps ' + facts.steps + ' ordered ' + facts.ordered)
  if (facts.headings.length) throw new Error('agents-' + width + ' duplicate headings ' + facts.headings.join(','))
  if (width === 390 && facts.height > 7100) throw new Error('agents-' + width + ' height ' + facts.height)
  if (facts.soon.join(',') !== 'preview_page,get_digest' || !facts.livePreview) throw new Error('agents-' + width + ' soon ' + facts.soon.join(','))
  if (!facts.modes || !facts.stub) throw new Error('agents-' + width + ' modes')
  if (facts.url !== 'https://offer-rails.onrender.com/mcp') throw new Error('agents-' + width + ' url ' + facts.url)
  if (facts.alternate !== '/llms.txt' || facts.alternateType !== 'text/plain') throw new Error('agents-' + width + ' alternate ' + facts.alternateType + ' ' + facts.alternate)
  if (facts.primary !== 1) throw new Error('agents-' + width + ' primary ' + facts.primary)
  if (facts.tram !== 'none') throw new Error('agents-' + width + ' motion ' + facts.tram)
  if (!facts.rates || !facts.errors) throw new Error('agents-' + width + ' limits or errors')
  if (facts.alone !== 4 || !facts.human || !facts.signup) throw new Error('agents-' + width + ' alone or signup')
  if (facts.deviceLinks.length !== 1 || facts.deviceLinks[0] !== '/device') throw new Error('agents-' + width + ' device link ' + facts.deviceLinks.join(','))
  if (copiedUrl !== 'https://offer-rails.onrender.com/mcp') throw new Error('agents-' + width + ' clipboard url ' + copiedUrl)
  if (copiedConfig !== '{"mcpServers":{"offer-rails":{"type":"http","url":"https://offer-rails.onrender.com/mcp"}}}') throw new Error('agents-' + width + ' clipboard config ' + copiedConfig)
  const reserved = await page.locator('[data-d063-steps]').innerText()
  const deviceCopy = await page.locator('#device').innerText()
  for (const needle of ['1. Signup', '11. Reporting', 'X-Plugin-Key', 'X-Admin-Key', 'X-API-Key', 'X-Maker-Key', 'The hosted app uses these steps.', 'Step 5 stays a human or an operator for now.']) {
    if (!reserved.includes(needle)) throw new Error('agents-' + width + ' d063 missing ' + needle)
  }
  for (const needle of ['Device sign-in', 'GET /device', 'The browser step needs a human sign-in']) {
    if (!deviceCopy.includes(needle)) throw new Error('agents-' + width + ' device missing ' + needle)
  }
  if (reserved.includes('not written') || reserved.includes('Reserved') || reserved.includes('Device sign-in')) throw new Error('agents-' + width + ' d063 still mixed ' + reserved.slice(0, 80))
  const genericPanel = await page.locator('#install-panel-generic').innerText()
  if (!genericPanel.includes('401 no bearer token') || genericPanel.includes('Untested')) throw new Error('agents-' + width + ' generic panel ' + genericPanel)
  await page.getByRole('tab', { name: 'Claude Code' }).click()
  const claudePanel = await page.locator('#install-panel-claude').innerText()
  if (!claudePanel.startsWith('Untested. Claude Code adds and removes the server, but sign-in to the hosted MCP server is not working yet.') || claudePanel.includes('Needs authentication') || claudePanel.includes('Tested: add and remove')) throw new Error('agents-' + width + ' claude panel ' + claudePanel)
  const pageText = await page.evaluate(() => document.body.textContent || '')
  for (const needle of ['Join returns tracked_link /v1/go/<tracking_code>', 'surface_url must be on the domain you signed up with', '422 claims_from_offer', '10 to 2000 characters', '409 receipt_mismatch', 'Refunds take no click_id', 'amount_refunded_minor or full:true', 'No Google sign-in and no role', 'POST /v1/agent-tokens']) {
    if (!pageText.includes(needle)) throw new Error('agents-' + width + ' missing ' + needle)
  }
  await page.getByRole('tab', { name: 'Windsurf' }).click()
  const windsurfPanel = await page.locator('#install-panel-windsurf').innerText()
  if (!windsurfPanel.startsWith('Untested.')) throw new Error('agents-' + width + ' windsurf panel ' + windsurfPanel)
  await page.locator('#on-this-page a[href="#tell"]').click()
  const covered = await page.evaluate(() => {
    const overlaps = (a, b) => a.bottom > b.top + 1 && a.right > b.left + 1 && a.left < b.right - 1 && a.top < b.bottom - 1
    const header = document.querySelector('header').getBoundingClientRect()
    const chips = document.querySelector('#on-this-page').getBoundingClientRect()
    const tell = document.querySelector('#tell').getBoundingClientRect()
    return {
      headerBottom: Math.round(header.bottom),
      chips: { top: Math.round(chips.top), bottom: Math.round(chips.bottom), left: Math.round(chips.left), right: Math.round(chips.right) },
      tellTop: Math.round(tell.top),
      overlap: overlaps(header, tell) || overlaps(chips, tell),
    }
  })
  await page.screenshot({ path: join(outDir, 'agents-tell-' + width + '.png') })
  if (covered.overlap) throw new Error('agents-' + width + ' header covers tell ' + JSON.stringify(covered))
  await page.getByRole('tab', { name: 'Claude Code' }).click()
  await page.locator('#on-this-page a[href="#install"]').click()
  await page.screenshot({ path: join(outDir, 'agents-install-' + width + '.png') })
  await context.close()
}

await new Promise((resolveListen) => server.listen(0, '127.0.0.1', resolveListen))
const base = 'http://127.0.0.1:' + server.address().port
mkdirSync(out, { recursive: true })

for (const path of ['/privacy', '/privacy/', '/terms', '/terms/']) {
  const res = await fetch(base + path)
  if (res.status !== 200) throw new Error(path + ' status ' + res.status)
  const type = res.headers.get('content-type') || ''
  if (!type.includes('text/html')) throw new Error(path + ' type ' + type)
  const html = await res.text()
  if (!html.includes('<meta name="robots" content="noindex, nofollow"')) throw new Error(path + ' missing noindex')
}
for (const file of ['mark.svg', 'mark-dark.svg', 'wordmark.svg', 'favicon.svg', 'favicon.ico', 'favicon-16.png', 'favicon-32.png', 'apple-touch-icon.png', 'mark-light-512.png', 'mark-light-1024.png', 'mark-transparent-512.png', 'mark-transparent-1024.png', 'mark-dark-512.png', 'mark-dark-1024.png']) {
  const res = await fetch(base + '/landing-v9/brand/' + file)
  if (res.status !== 200) throw new Error('brand ' + file + ' ' + res.status)
}
async function assertHeaderSignIn(page, slug) {
  const box = await page.locator('header').getByRole('link', { name: 'Sign in', exact: true }).boundingBox()
  if (!box || box.width < 44 || box.height < 44) throw new Error(slug + ' header Sign in ' + JSON.stringify(box))
}

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
      await page.locator('#join summary').click()
      const signIn = page.getByRole('link', { name: /Sign in with Google|Start as a maker|Start as a publisher/ }).first()
      const signHref = await signIn.getAttribute('href')
      const zip = await page.getByRole('link', { name: 'Download WordPress plugin' }).first().getAttribute('href')
      summary.push({ slug, ...metrics, serious: serious.map((item) => item.id), signHref, zip })
      await assertHeaderSignIn(page, slug)
      if (!metrics.brand) throw new Error(slug + ' still shows the old brand name')
      if (!metrics.scroll) throw new Error(slug + ' scrolls sideways')
      if (metrics.small.length) throw new Error(slug + ' small targets ' + JSON.stringify(metrics.small))
      if (serious.length) throw new Error(slug + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
      if (!signHref.includes('/auth/login') || !signHref.includes('source=cursor')) throw new Error(slug + ' sign-in href ' + signHref)
      if (zip !== '/downloads/offer-rails.zip') throw new Error(slug + ' zip ' + zip)
      if (route === '/' && metrics.how !== 0) throw new Error('home still has the how-it-works anchor')
      if (route === '/' && metrics.preview !== 1) throw new Error('home preview fields ' + metrics.preview)
      if (route !== '/' && metrics.preview) throw new Error(route + ' preview field leaked')
      const expectPrimary = route === '/' ? 0 : 1
      if (metrics.primary !== expectPrimary) throw new Error(slug + ' primary count ' + metrics.primary)
      const mcpHrefs = await page.getByRole('link', { name: 'MCP and setup' }).evaluateAll((links) => links.map((link) => link.getAttribute('href') || ''))
      if (!mcpHrefs.length || mcpHrefs.some((href) => !href.includes('/landing-v9/agents/') || !href.includes('source=cursor'))) throw new Error(slug + ' MCP href ' + mcpHrefs.join(','))
      const positioning = route === '/' ? /this fits your post, want to monetize it/ : route === '/makers' ? /Pay only for sales/ : /Earn from more of your posts/
      if (!(await page.getByRole('heading', { level: 1, name: positioning }).count())) throw new Error(slug + ' missing the positioning heading')
      const spoken = await page.locator('body').innerText()
      if (/\bads\b/i.test(spoken) || /revolution/i.test(spoken) || /\bpilot\b/i.test(spoken) || /every post/i.test(spoken)) throw new Error(slug + ' banned positioning word')
      if (route === '/') {
        if (await page.getByRole('button', { name: 'I sell a product' }).count()) throw new Error('home still asks who you are')
        if (await page.getByRole('link', { name: 'I run a site' }).count()) throw new Error('home still asks who you are')
        const limitsHref = await page.getByRole('link', { name: 'Your limits' }).first().getAttribute('href')
        if (!limitsHref || !limitsHref.includes('/you')) throw new Error('home limits href ' + limitsHref)
        await page.locator('#join summary').click()
        const google = await page.getByRole('link', { name: 'Sign in with Google' }).getAttribute('href')
        if (!google || !google.includes('/auth/login')) throw new Error('home google href ' + google)
      }
      if (route === '/makers' && await page.getByRole('link', { name: 'I run a site' }).count()) throw new Error('makers page offers the site choice')
      if (route === '/publishers') {
        await page.getByRole('link', { name: 'I run a site' }).click()
        await page.waitForURL(/\/publishers\/setup/)
        if (!page.url().includes('source=cursor')) throw new Error('publisher page site choice dropped the source ' + page.url())
        const publisherZip = await page.getByRole('link', { name: 'Download the plugin' }).getAttribute('href')
        if (publisherZip !== '/downloads/offer-rails.zip') throw new Error('publisher page step 1 zip ' + publisherZip)
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
    const zipResponse = await page.request.get(base + '/downloads/offer-rails.zip')
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
    await assertHeaderSignIn(protocolPage, 'protocol-' + width)
    if (!protocolOk.scroll) throw new Error('protocol-' + width + ' scrolls sideways')
    if (protocolOk.title !== 'Offer Rails Protocol') throw new Error('protocol heading ' + protocolOk.title)
    if (!protocolOk.nav.includes('/protocol/') || !protocolOk.nav.includes('source=cursor')) throw new Error('protocol nav ' + protocolOk.nav)
    if (protocolSerious.length) throw new Error('protocol-' + width + ' axe ' + protocolSerious.map((item) => item.id).join(' '))
    await protocolContext.close()
    for (const legal of ['privacy', 'terms']) {
      const legalContext = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
      const legalPage = await legalContext.newPage()
      const response = await legalPage.goto(base + '/' + legal + '?source=cursor', { waitUntil: 'networkidle' })
      if (!response || response.status() !== 200) throw new Error(legal + ' status ' + (response && response.status()))
      const text = await legalPage.locator('body').innerText()
      const banned = /grok|connector|marketplace|\blisted\b|github\.com|distributoor|affilio/i
      if (banned.test(text)) throw new Error(legal + '-' + width + ' banned copy')
      const need = legal === 'privacy'
        ? ['Draft. This text is not final and has not been approved yet.', '18 October 2026', '[OPERATOR NAME]', '[CONTACT EMAIL]', '10 to 12000', 'surface URL', 'not followed', 'signed-out caller', 'frozen copy', 'approved by', 'pass or fail signals', 'We never store source code, environment files or secrets.', '/24', '/48', 'daily salt', 'raw IP is never stored', '30 days', 'Render hosts', 'Neon provides Postgres', 'Cloudflare sits in front of Render', 'Stripe processes payments', 'If AI copy generation is turned on, page text you send is processed by Anthropic (Claude) to classify and draft copy.', 'no telemetry', 'train models', 'not sold', 'not transferred', 'untrusted data', 'only tighten modes', 'Until the claims check is live, every placement needs a person to approve it.', 'Not checked yet']
        : ['Draft. This text is not final and has not been approved yet.', '18 October 2026', '[OPERATOR NAME]', '[CONTACT EMAIL]', 'A maker may publish an offer', 'sale and a refund', 'does not hold the money', 'approve or refuse', 'until a person approves', 'only tighten an approval mode', 'cannot approve a placement', 'cannot turn YOLO on', 'test mode', 'not a live charge', 'no warranty', 'as is']
      for (const needle of need) {
        if (!text.includes(needle)) throw new Error(legal + '-' + width + ' missing ' + needle)
      }
      if (legal === 'privacy') {
        for (const dropped of ['does not add practices', 'claim_check_unavailable', 'reviewer demo account', 'does not say the call is on', '.env files']) {
          if (text.includes(dropped)) throw new Error(legal + '-' + width + ' still says ' + dropped)
        }
      }
      const legalMetrics = await legalPage.evaluate(() => {
        const banner = document.querySelector('[data-draft-banner]')
        const box = banner ? banner.getBoundingClientRect() : { width: 0, height: 0 }
        return {
          scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
          small: [...document.querySelectorAll('a, button')].filter((el) => {
            const style = getComputedStyle(el)
            if (style.display === 'none' || style.visibility === 'hidden') return false
            const target = el.getBoundingClientRect()
            if (target.width < 2 || target.height < 2) return false
            return target.width < 44 || target.height < 44
          }).map((el) => ({ text: (el.innerText || el.getAttribute('aria-label') || '').trim().slice(0, 80), w: Math.round(el.getBoundingClientRect().width), h: Math.round(el.getBoundingClientRect().height) })),
          banner: box.height >= 24 && box.width > 200,
          current: document.querySelector('nav[aria-label="Main"] a[aria-current="page"]')?.textContent || '',
          privacy: document.querySelector('nav[aria-label="Footer"] a[href^="/privacy"]')?.getAttribute('href') || '',
          terms: document.querySelector('nav[aria-label="Footer"] a[href^="/terms"]')?.getAttribute('href') || '',
          primary: document.querySelectorAll('[data-primary-action]').length,
        }
      })
      const legalAxe = await new AxeBuilder({ page: legalPage }).analyze()
      const legalSerious = legalAxe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
      writeFileSync(join(out, `axe-${legal}-${width}.json`), JSON.stringify({ violations: legalSerious }, null, 2))
      await legalPage.screenshot({ path: join(out, legal + '-' + width + '.png'), fullPage: true })
      summary.push({ slug: legal + '-' + width, ...legalMetrics, serious: legalSerious.map((item) => item.id) })
      await assertHeaderSignIn(legalPage, legal + '-' + width)
      if (!legalMetrics.scroll) throw new Error(legal + '-' + width + ' scrolls sideways')
      if (legalMetrics.small.length) throw new Error(legal + '-' + width + ' small targets ' + JSON.stringify(legalMetrics.small))
      if (!legalMetrics.banner) throw new Error(legal + '-' + width + ' draft banner')
      if (legalMetrics.current) throw new Error(legal + '-' + width + ' nav current ' + legalMetrics.current)
      if (!legalMetrics.privacy.startsWith('/privacy') || !legalMetrics.privacy.includes('source=cursor')) throw new Error(legal + ' privacy href ' + legalMetrics.privacy)
      if (!legalMetrics.terms.startsWith('/terms') || !legalMetrics.terms.includes('source=cursor')) throw new Error(legal + ' terms href ' + legalMetrics.terms)
      if (legalMetrics.primary !== 0) throw new Error(legal + '-' + width + ' primary count ' + legalMetrics.primary)
      if (legalSerious.length) throw new Error(legal + '-' + width + ' axe ' + legalSerious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
      await legalContext.close()
    }
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
    const manual = await setupPage.getByRole('radio', { name: /Manual/ }).isChecked()
    const setupCopy = await setupPage.locator('body').innerText()
    summary.push({ slug: 'setup-' + width, ...setupMetrics, serious: setupSerious.map((item) => item.id), connectHref, manual })
    await assertHeaderSignIn(setupPage, 'setup-' + width)
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
    if (!manual) throw new Error('setup-' + width + ' does not preselect Manual')
    if (!setupCopy.includes('A new site waits for a person to approve each placement before it is published.') || !setupCopy.includes('That starting choice stays when claim checks are on.')) throw new Error('setup-' + width + ' copy drifted')
    if (!connectHref.includes('/auth/login') || !connectHref.includes('source=cursor') || !connectTo.startsWith('/owner/connect-site') || !connectTo.includes('source=cursor')) throw new Error('setup connect href ' + connectHref)
    const setupZip = await setupPage.getByRole('link', { name: 'Download the plugin' }).getAttribute('href')
    if (setupZip !== '/downloads/offer-rails.zip') throw new Error('setup zip ' + setupZip)
    await setupContext.close()
    await assertAgents(browser, base, width, summary, out)
    await walkPreview(browser, base, width, summary, out)
  }
  await capturePreviewGroups(browser, base)
  writeFileSync(join(out, 'summary.json'), JSON.stringify(summary, null, 2))
  mkdirSync('/opt/cursor/artifacts/screenshots', { recursive: true })
  for (const name of ['privacy-390.png', 'privacy-1440.png', 'terms-390.png', 'terms-1440.png']) {
    copyFileSync(join(out, name), '/opt/cursor/artifacts/screenshots/' + name)
  }
  console.log(JSON.stringify(summary, null, 2))
} finally {
  await browser.close()
  server.close()
}
