// D-060 proof: directory under the heroes, profiles, unknown-slug 404, copy button.
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'
import { fixtureList, fixtureMember } from '../src/directory/fixtures.mjs'
import { agentPrompt } from '../src/directory/prompt.mjs'
import { memberRoute } from '../src/directory/client.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repo = resolve(root, '../..')
const build = resolve(repo, 'affilio-wp/admin/build')
const shots = '/opt/cursor/artifacts/screenshots'
const install = 'npx --yes --package=https://offer-rails.example/downloads/offer-rails-0.1.0.tgz offer-rails --api-url https://offer-rails.example'

function page(name) {
  return readFileSync(join(build, name), 'utf8')
    .replaceAll('{{.InstallCommand}}', install)
    .replaceAll('{{.BaseURL}}', 'http://127.0.0.1')
    .replace(/\{\{if \.InstallerReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
    .replace(/\{\{if \.PluginReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
}

const pages = {
  '/makers': page('makers.html'),
  '/makers/': page('makers.html'),
  '/publishers': page('publishers.html'),
  '/publishers/': page('publishers.html'),
}
const profile = page('landing-v9/profile/index.html')
const csp = "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; img-src 'self'; connect-src 'self'"

function typeFor(file) {
  if (file.endsWith('.css')) return 'text/css; charset=utf-8'
  if (file.endsWith('.js')) return 'text/javascript; charset=utf-8'
  if (file.endsWith('.svg')) return 'image/svg+xml'
  if (file.endsWith('.woff2')) return 'font/woff2'
  if (file.endsWith('.webp')) return 'image/webp'
  return 'application/octet-stream'
}

function sendDirectory(req, res, url) {
  const fixtures = req.headers['x-directory-fixtures'] === '1'
  if (!fixtures) {
    res.writeHead(404, { 'content-type': 'application/json' })
    res.end('{"error":"not found"}')
    return
  }
  const parts = url.pathname.split('/').filter(Boolean)
  const query = Object.fromEntries(url.searchParams.entries())
  if (parts.length === 3 && (parts[2] === 'makers' || parts[2] === 'publishers')) {
    const kind = parts[2] === 'makers' ? 'maker' : 'publisher'
    res.writeHead(200, { 'content-type': 'application/json' })
    res.end(JSON.stringify(fixtureList(kind, query)))
    return
  }
  if (parts.length === 4 && (parts[2] === 'maker' || parts[2] === 'publisher')) {
    const item = fixtureMember(parts[2], decodeURIComponent(parts[3]))
    if (!item) {
      res.writeHead(404, { 'content-type': 'application/json' })
      res.end('{"error":"not found"}')
      return
    }
    res.writeHead(200, { 'content-type': 'application/json' })
    res.end(JSON.stringify(item))
    return
  }
  res.writeHead(404, { 'content-type': 'application/json' })
  res.end('{"error":"not found"}')
}

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  if (url.pathname.startsWith('/v1/directory/')) {
    sendDirectory(req, res, url)
    return
  }
  if (pages[url.pathname]) {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'content-security-policy': csp })
    res.end(pages[url.pathname])
    return
  }
  const route = memberRoute(url.pathname)
  if (route) {
    const known = url.searchParams.get('directoryFixtures') === '1' && fixtureMember(route.kind, route.slug)
    res.writeHead(known ? 200 : 404, { 'content-type': 'text/html; charset=utf-8', 'content-security-policy': csp })
    res.end(profile)
    return
  }
  if (url.pathname.startsWith('/landing-v9/')) {
    const file = join(build, url.pathname)
    if (!file.startsWith(build)) {
      res.writeHead(404)
      res.end('missing')
      return
    }
    try {
      if (!statSync(file).isFile()) throw new Error('not a file')
      res.writeHead(200, { 'content-type': typeFor(file) })
      res.end(readFileSync(file))
    } catch {
      res.writeHead(404)
      res.end('missing')
    }
    return
  }
  res.writeHead(404, { 'content-type': 'text/html; charset=utf-8' })
  res.end(profile)
})

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const file = join(dir, name)
    if (statSync(file).isDirectory()) walk(file, out)
    else out.push(file)
  }
  return out
}

await new Promise((resolveListen) => server.listen(0, '127.0.0.1', resolveListen))
const base = 'http://127.0.0.1:' + server.address().port
const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/usr/local/bin/google-chrome', args: ['--disable-gpu'] })
mkdirSync(shots, { recursive: true })

const axeLines = []

async function check(page, label) {
  const scroll = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)
  if (!scroll) throw new Error(label + ' scrolls sideways')
  const axe = await new AxeBuilder({ page }).analyze()
  const impacts = ['critical', 'serious', 'moderate', 'minor']
  const counts = Object.fromEntries(impacts.map((impact) => [impact, axe.violations.filter((item) => item.impact === impact).length]))
  const rules = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical').map((item) => `${item.impact}:${item.id}`)
  axeLines.push(`${label} ${impacts.map((impact) => impact + '=' + counts[impact]).join(' ')} ${rules.length ? 'rules=' + rules.join(',') : 'rules=none'}`)
  const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
  if (serious.length) throw new Error(label + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.html.slice(0, 80)).join('; ')).join(' | '))
  const primary = await page.locator('#directory [data-primary-action]').count()
  if (primary) throw new Error(label + ' directory primary ' + primary)
}

try {
  const shipped = walk(join(build, 'landing-v9')).filter((file) => file.endsWith('.js'))
  for (const file of shipped) {
    if (readFileSync(file, 'utf8').includes('TEST Vibe')) throw new Error('fixture leaked into ' + file)
  }
  for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce', permissions: ['clipboard-read', 'clipboard-write'] })
    const page = await context.newPage()

    await page.goto(base + '/makers/?directoryFixtures=1', { waitUntil: 'networkidle' })
    await page.locator('#directory').getByRole('heading', { name: 'Makers on Offer Rails' }).waitFor()
    await page.locator('#directory').getByText('TEST Vibe Sales').waitFor()
    const placed = await page.evaluate(() => {
      const hero = document.querySelector('#hero-h')?.closest('section')
      return hero?.nextElementSibling?.id === 'directory'
    })
    if (!placed) throw new Error('makers directory is not under the hero')
    if (!/verified/i.test(await page.locator('#directory [data-member]').first().innerText())) throw new Error('verified badge missing')
    await page.locator('#directory').getByRole('button', { name: 'Load more' }).click()
    await page.locator('#directory').getByText('TEST Field Kit').waitFor()
    await check(page, 'makers-list-' + width)
    await page.locator('#directory').screenshot({ path: join(shots, `d060-makers-list-${width}.png`) })

    await page.goto(base + '/makers/', { waitUntil: 'networkidle' })
    await page.locator('#directory').getByText('No makers listed yet. Yours can be first.').waitFor()
    const intact = await page.evaluate(() => {
      const pre = document.querySelector('#directory pre')
      if (!pre) return false
      const whole = (word) => {
        const walker = document.createTreeWalker(pre, NodeFilter.SHOW_TEXT)
        let node
        while ((node = walker.nextNode())) {
          const index = node.textContent.indexOf(word)
          if (index < 0) continue
          const range = document.createRange()
          range.setStart(node, index)
          range.setEnd(node, index + word.length)
          return range.getClientRects().length === 1
        }
        return false
      }
      return whole('directory') && whole('transport')
    })
    if (!intact) throw new Error('prompt breaks a word at ' + width)
    if (await page.locator('#directory [data-member]').count()) throw new Error('empty makers showed a member')
    const directory = page.locator('#directory')
    await directory.getByRole('button', { name: 'Copy prompt' }).click()
    await directory.getByText('Copied').waitFor()
    const copied = await page.evaluate(() => navigator.clipboard.readText())
    if (copied !== agentPrompt) throw new Error('copy ' + copied)
    const agents = await directory.getByRole('link', { name: 'Agents' }).getAttribute('href')
    if (!agents || !agents.startsWith('/agents')) throw new Error('agents link ' + agents)
    await check(page, 'makers-empty-' + width)
    await directory.screenshot({ path: join(shots, `d060-makers-empty-${width}.png`) })

    await page.goto(base + '/makers/?directoryFixtures=1&q=zzznomatch', { waitUntil: 'networkidle' })
    await page.locator('#directory').getByText('No sites match this search.').waitFor()
    if (await page.locator('#directory [data-member]').count()) throw new Error('filter showed a member')
    await check(page, 'makers-filter-' + width)
    await page.locator('#directory').screenshot({ path: join(shots, `d060-makers-filter-${width}.png`) })

    const profileResponse = await page.goto(base + '/makers/test-vibesales?directoryFixtures=1', { waitUntil: 'networkidle' })
    if (!profileResponse || profileResponse.status() !== 200) throw new Error('profile status ' + (profileResponse && profileResponse.status()))
    await page.getByRole('heading', { name: 'TEST Vibe Sales' }).waitFor()
    await check(page, 'makers-profile-' + width)
    await page.screenshot({ path: join(shots, `d060-profile-${width}.png`), fullPage: true })

    const missing = await page.goto(base + '/makers/not-a-real-site', { waitUntil: 'networkidle' })
    if (!missing || missing.status() !== 404) throw new Error('404 status ' + (missing && missing.status()))
    await page.getByRole('heading', { name: 'That page is not on Offer Rails.' }).waitFor()
    const nav = await page.locator('nav[aria-label="Main"]').innerText()
    if (!nav.includes('Makers') || !nav.includes('Publishers')) throw new Error('404 nav ' + nav)
    const home = await page.locator('#main').getByRole('link', { name: 'Home', exact: true }).getAttribute('href')
    const missingAgents = await page.locator('#main').getByRole('link', { name: 'Agents', exact: true }).getAttribute('href')
    if (!home || !home.startsWith('/') || !missingAgents || !missingAgents.startsWith('/agents')) throw new Error('404 links ' + home + ' ' + missingAgents)
    await check(page, '404-' + width)
    await page.screenshot({ path: join(shots, `d060-404-${width}.png`), fullPage: true })

    await page.goto(base + '/publishers/', { waitUntil: 'networkidle' })
    await page.locator('#directory').getByText('No publishers listed yet. Yours can be first.').waitFor()
    await check(page, 'publishers-empty-' + width)
    await page.goto(base + '/publishers/?directoryFixtures=1', { waitUntil: 'networkidle' })
    await page.locator('#directory').getByRole('heading', { name: 'Publishers on Offer Rails' }).waitFor()
    await page.locator('#directory').getByText('TEST Field Journal').waitFor()
    const publishersPlaced = await page.evaluate(() => {
      const hero = document.querySelector('#hero-h')?.closest('section')
      return hero?.nextElementSibling?.id === 'directory'
    })
    if (!publishersPlaced) throw new Error('publishers directory is not under the hero')
    await check(page, 'publishers-list-' + width)
    await page.locator('#directory').screenshot({ path: join(shots, `d060-publishers-list-${width}.png`) })

    const publisherProfile = await page.goto(base + '/publishers/test-journal?directoryFixtures=1', { waitUntil: 'networkidle' })
    if (!publisherProfile || publisherProfile.status() !== 200) throw new Error('publisher profile ' + (publisherProfile && publisherProfile.status()))
    await page.getByRole('heading', { name: 'TEST Field Journal' }).waitFor()
    await check(page, 'publishers-profile-' + width)

    const publisherMiss = await page.goto(base + '/publishers/not-a-real-site', { waitUntil: 'networkidle' })
    if (!publisherMiss || publisherMiss.status() !== 404) throw new Error('publisher 404 ' + (publisherMiss && publisherMiss.status()))
    await page.getByRole('heading', { name: 'That page is not on Offer Rails.' }).waitFor()
    await check(page, 'publishers-404-' + width)
    await context.close()
  }
  mkdirSync(resolve(repo, 'proof/d060'), { recursive: true })
  writeFileSync(resolve(repo, 'proof/d060/axe.txt'), [
    'D-060 axe-core, viewports 390 and 1440.',
    'Pages this branch changes: /makers, /publishers, member profiles, and the unknown-slug 404.',
    ...axeLines,
    '',
    'serious+critical rules are listed on each line. A line with rules=none has none.',
    '',
  ].join('\n'))
} finally {
  await browser.close()
  server.close()
}

const go = spawn('go', ['test', './internal/handlers', '-count=1', '-run', 'TestDirectoryUnknownSlugIs404'], {
  cwd: resolve(repo, 'affilio-api'),
  env: { ...process.env, GOTOOLCHAIN: 'auto', GOCACHE: '/tmp/go-cache', GOMODCACHE: '/tmp/go-mod-cache' },
  stdio: 'inherit',
})
const code = await new Promise((done) => go.on('exit', done))
if (code !== 0) throw new Error('go test ' + code)
console.log('d060 proof ok')
