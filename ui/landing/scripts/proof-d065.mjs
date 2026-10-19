// D-065 proof: how-it-works, the branded 404, demo noindex, and the client/channel list.
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repo = resolve(root, '../..')
const build = resolve(repo, 'affilio-wp/admin/build')
const shots = '/opt/cursor/artifacts/screenshots'

function typeFor(file) {
  if (file.endsWith('.css')) return 'text/css; charset=utf-8'
  if (file.endsWith('.html')) return 'text/html; charset=utf-8'
  if (file.endsWith('.js')) return 'text/javascript; charset=utf-8'
  if (file.endsWith('.svg')) return 'image/svg+xml'
  if (file.endsWith('.png')) return 'image/png'
  if (file.endsWith('.webp')) return 'image/webp'
  if (file.endsWith('.woff2')) return 'font/woff2'
  return 'application/octet-stream'
}

function fileFor(path) {
  const rel = path.endsWith('/') ? path + 'index.html' : path
  const file = join(build, rel)
  if (!file.startsWith(build)) return ''
  try {
    if (statSync(file).isFile()) return file
  } catch {
    return ''
  }
  return ''
}

const notFound = () => readFileSync(join(build, 'landing-v9/not-found/index.html'))

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  const file = fileFor(url.pathname)
  if (file) {
    const headers = { 'content-type': typeFor(file) }
    if (url.pathname.startsWith('/landing-v9/how-it-works') || url.pathname === '/not-a-real-landing-page') {
      headers['content-security-policy'] = "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; img-src 'self'; connect-src 'self'"
    }
    res.writeHead(200, headers)
    res.end(readFileSync(file))
    return
  }
  if (url.pathname.startsWith('/landing-v9/')) {
    res.writeHead(404)
    res.end('missing')
    return
  }
  res.writeHead(404, {
    'content-type': 'text/html; charset=utf-8',
    'content-security-policy': "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; img-src 'self'; connect-src 'self'",
  })
  res.end(notFound())
})

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const file = join(dir, name)
    if (statSync(file).isDirectory()) walk(file, out)
    else if (/\.(html|js|mjs|css|tsx?)$/.test(name)) out.push(file)
  }
  return out
}

function channelReport() {
  const files = [
    ...walk(resolve(root, 'src')).filter((file) => file.endsWith('.tsx') || file.endsWith('.ts')),
    resolve(root, 'public/how-it-works/index.html'),
  ]
  const names = ['Claude Code', 'Generic', 'Cursor', 'VS Code', 'Windsurf', 'ChatGPT', 'Grok', 'Shopify', 'WordPress', 'Traffic trade', 'X']
  const lines = []
  for (const file of files) {
    const text = readFileSync(file, 'utf8')
    const banned = text.split('\n').filter((row) => /marketplace|connector/i.test(row) && !/preselects YOLO for cli, cursor, grok, mcp, and connector/.test(row))
    if (banned.length) throw new Error('banned claim in ' + file + '\n' + banned.join('\n'))
    const rows = text.split('\n')
    rows.forEach((row, index) => {
      for (const name of names) {
        const hit = name === 'X' ? /\bX\b/.test(row) : row.includes(name)
        if (!hit) continue
        if (name === 'X' && !/Shopify|Coming|coming|Planned|planned/.test(row)) continue
        let status = 'named'
        if (name === 'WordPress') status = 'live plugin'
        else if (/untested/i.test(row) && (name === 'Claude Code' || name === 'Generic')) status = 'untested'
        else if (/planned/i.test(row)) status = 'planned'
        else if (/later/i.test(row)) status = 'later'
        else if (/coming/i.test(row)) status = 'coming'
        lines.push(`${name}\t${status}\t${file.replace(repo + '/', '')}:${index + 1}`)
      }
    })
  }
  const required = {
    'Claude Code': 'untested',
    Generic: 'untested',
    Cursor: 'coming',
    'VS Code': 'coming',
    Windsurf: 'coming',
    ChatGPT: 'coming',
    Grok: 'coming',
    Shopify: 'planned',
    'Traffic trade': 'later',
    X: 'planned',
  }
  for (const [name, status] of Object.entries(required)) {
    if (!lines.some((line) => line.startsWith(name + '\t' + status + '\t'))) throw new Error('missing ' + status + ' ' + name)
  }
  if (lines.some((line) => line.startsWith('Cursor\tnamed') || line.startsWith('Grok\tnamed') || line.startsWith('Shopify\tnamed') || line.startsWith('ChatGPT\tnamed'))) {
    throw new Error('unlabeled client\n' + lines.filter((line) => line.includes('\tnamed\t')).join('\n'))
  }
  const proofClaude = readFileSync(resolve(repo, 'proof/d055/clients/claude-code.txt'), 'utf8')
  const proofGeneric = readFileSync(resolve(repo, 'proof/d055/clients/generic.txt'), 'utf8')
  if (!proofClaude.includes('https://offer-rails.onrender.com/mcp') || !proofGeneric.includes('HTTP/2 401')) {
    throw new Error('saved client proof is missing')
  }
  return lines.join('\n') + '\n'
}

await new Promise((resolveListen) => server.listen(0, '127.0.0.1', resolveListen))
const base = 'http://127.0.0.1:' + server.address().port
const css = await fetch(base + '/landing-v9/how-it-works/theme.css')
if (css.status !== 200 || !(await css.text()).includes('.site-head')) throw new Error('theme.css ' + css.status)

const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/usr/local/bin/google-chrome', args: ['--disable-gpu'] })
mkdirSync(shots, { recursive: true })
try {
  for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 900 }, reducedMotion: 'reduce' })
    const page = await context.newPage()
    const cssStatus = []
    page.on('response', (res) => {
      if (res.url().includes('theme.css')) cssStatus.push(res.status())
    })
    await page.goto(base + '/landing-v9/how-it-works/', { waitUntil: 'networkidle' })
    if (!cssStatus.includes(200)) throw new Error('how-it-works theme.css ' + cssStatus.join(','))
    const how = await page.evaluate(() => ({
      scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
      nav: document.querySelector('nav[aria-label="Main"]')?.innerText || '',
      text: document.body.innerText,
      head: getComputedStyle(document.querySelector('.site-head')).display,
    }))
    if (how.head === 'none' || !how.nav.includes('Makers') || !how.nav.includes('Publishers') || !how.nav.includes('Protocol')) throw new Error('how header ' + how.nav)
    if (!how.text.includes('Nine hard checks') || !/all 9 checks|Nine hard checks/.test(how.text)) throw new Error('nine checks missing')
    for (const item of ['Disclosure label next to the link', 'rel="sponsored" on every link', 'Claims come from the offer data only', 'Copy capped at 140 characters', 'One box per post', 'Relevance score 25 or higher', 'Offer is live', 'Revoke works', 'Monthly cap respected']) {
      if (!how.text.includes(item)) throw new Error('missing check ' + item)
    }
    if (/three checks|3 checks|all three/i.test(how.text)) throw new Error('still says three checks')
    if (!how.scroll) throw new Error('how-it-works-' + width + ' scrolls sideways')
    const sign = await page.locator('header').getByRole('link', { name: 'Sign in', exact: true }).boundingBox()
    if (!sign || sign.width < 44 || sign.height < 44) throw new Error('how sign-in ' + JSON.stringify(sign))
    const logo = await page.locator('header .site-logo svg').count()
    if (!logo) throw new Error('rail logo missing')
    const hint = await page.locator('[data-corner="hint"]').innerText()
    if (width < 768) {
      if (!/tap to step/i.test(hint) || /hover/i.test(hint)) throw new Error('narrow hint ' + hint)
    } else if (!/hover/i.test(hint) || /tap to step/i.test(hint)) {
      throw new Error('wide hint ' + hint)
    }
    const axe = await new AxeBuilder({ page }).analyze()
    const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
    if (serious.length) throw new Error('how-it-works-' + width + ' axe ' + serious.map((item) => item.id + ' ' + item.nodes.map((node) => node.target.join(' ')).join('; ')).join(' | '))
    await page.screenshot({ path: join(shots, `d065-how-it-works-${width}.png`), fullPage: true })

    const response = await page.goto(base + '/not-a-real-landing-page', { waitUntil: 'networkidle' })
    if (!response || response.status() !== 404) throw new Error('404 status ' + (response && response.status()))
    const missing = await page.evaluate(() => ({
      scroll: document.documentElement.scrollWidth <= window.innerWidth + 1,
      h: document.querySelector('h1')?.textContent || '',
      nav: document.querySelector('nav[aria-label="Main"]')?.innerText || '',
      home: document.querySelector('a[href="/"], a[href^="/?"]')?.textContent || '',
      agents: [...document.querySelectorAll('a')].some((el) => el.getAttribute('href')?.startsWith('/agents')),
    }))
    if (missing.h !== 'That page is not on Offer Rails.') throw new Error('404 heading ' + missing.h)
    if (!missing.nav.includes('Makers') || !missing.agents) throw new Error('404 links ' + JSON.stringify(missing))
    if (!missing.scroll) throw new Error('404-' + width + ' scrolls sideways')
    const missingAxe = await new AxeBuilder({ page }).analyze()
    const missingSerious = missingAxe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
    if (missingSerious.length) throw new Error('404-' + width + ' axe ' + missingSerious.map((item) => item.id).join(' '))
    await page.screenshot({ path: join(shots, `d065-404-${width}.png`), fullPage: true })
    await context.close()
  }
  const report = channelReport()
  writeFileSync(join(shots, 'd065-channels.txt'), report)
  console.log(report)
} finally {
  await browser.close()
  server.close()
}

const go = spawn('go', ['test', './internal/handlers', '-count=1', '-run', 'TestDemoProductSitePassesReadiness|TestLandingUnknownPathIs404'], {
  cwd: resolve(repo, 'affilio-api'),
  env: { ...process.env, GOTOOLCHAIN: 'auto', GOCACHE: '/tmp/go-cache', GOMODCACHE: '/tmp/go-mod-cache' },
  stdio: 'inherit',
})
const code = await new Promise((done) => go.on('exit', done))
if (code !== 0) throw new Error('go test ' + code)
console.log('d065 proof ok')
