import { createServer } from 'node:http'
import { existsSync, readFileSync, mkdirSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const defaultBuild = resolve(scriptDir, '../../../affilio-wp/admin/build')

function defaultChrome() {
  const fromEnv = process.env.CHROME_PATH || process.env.CHROME
  if (fromEnv) return fromEnv
  const candidates = ['/usr/local/bin/chrome', '/opt/google/chrome/chrome', '/usr/bin/google-chrome', '/usr/bin/chromium']
  return candidates.find((candidate) => existsSync(candidate)) || 'chrome'
}

function arg(name) {
  const flag = process.argv.find((item) => item.startsWith(`--${name}=`))
  return flag ? flag.slice(name.length + 3) : ''
}

const outDir = arg('out') || process.argv[2]
const build = resolve(arg('build') || process.env.AFFILIO_BUILD || defaultBuild)
const chromePath = arg('chrome') || defaultChrome()
if (!outDir || outDir.startsWith('--')) {
  console.error('usage: node scripts/proof-d121.mjs <out-dir> [--build=DIR] [--chrome=PATH]')
  console.error('env: AFFILIO_BUILD, CHROME_PATH')
  process.exit(2)
}

const routes = [
  ['home', '/'],
  ['you', '/you/'],
  ['agents', '/agents'],
  ['makers', '/makers/'],
  ['publishers', '/publishers/'],
  ['setup', '/publishers/setup'],
  ['protocol', '/protocol/'],
]
const widths = [390, 1280, 1440]
const impacts = ['minor', 'moderate', 'serious', 'critical']

function page(name) {
  return readFileSync(join(build, name), 'utf8')
    .replaceAll('{{.InstallCommand}}', '')
    .replaceAll('{{.BaseURL}}', 'http://127.0.0.1')
    .replace(/\{\{if \.InstallerReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
    .replace(/\{\{if \.PluginReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1')
}

const agentsHtml = readFileSync(join(build, 'landing-v9/agents/index.html'), 'utf8')
const pages = {
  '/': page('home.html'),
  '/you': page('you.html'),
  '/you/': page('you.html'),
  '/makers': page('makers.html'),
  '/makers/': page('makers.html'),
  '/publishers': page('publishers.html'),
  '/publishers/': page('publishers.html'),
  '/publishers/setup': page('publishers-setup.html'),
  '/publishers/setup/': page('publishers-setup.html'),
  '/protocol': page('protocol.html'),
  '/protocol/': page('protocol.html'),
  '/agents': agentsHtml,
  '/agents/': agentsHtml,
}

function typeFor(file) {
  if (file.endsWith('.css')) return 'text/css; charset=utf-8'
  if (file.endsWith('.woff2')) return 'font/woff2'
  if (file.endsWith('.svg')) return 'image/svg+xml'
  if (file.endsWith('.png')) return 'image/png'
  if (file.endsWith('.ico')) return 'image/x-icon'
  if (file.endsWith('.js')) return 'text/javascript; charset=utf-8'
  return 'application/octet-stream'
}

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  const path = url.pathname
  if (pages[path] || path.startsWith('/protocol/')) {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
    res.end(pages[path] || pages['/protocol/'])
    return
  }
  if (path.startsWith('/landing-v9/')) {
    const file = join(build, path)
    try {
      if (!file.startsWith(build) || !statSync(file).isFile()) throw new Error('dir')
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
const origin = `http://127.0.0.1:${server.address().port}`
mkdirSync(outDir, { recursive: true })
const browser = await chromium.launch({
  headless: true,
  executablePath: chromePath,
  args: ['--disable-gpu'],
})
const report = []
const totals = Object.fromEntries(impacts.map((impact) => [impact, 0]))
let failed = false
try {
  for (const width of widths) {
    const context = await browser.newContext({
      viewport: { width, height: width === 390 ? 844 : 900 },
      reducedMotion: 'reduce',
    })
    const page = await context.newPage()
    for (const [name, route] of routes) {
      await page.goto(origin + route, { waitUntil: 'networkidle' })
      const shot = join(outDir, `${name}-${width}.png`)
      await page.screenshot({ path: shot, fullPage: false })
      const layout = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
        title: document.title,
        text: document.body.innerText,
      }))
      const overflow = layout.scrollWidth > layout.innerWidth + 1
      const offerRails = (layout.title + '\n' + layout.text).includes('Offer Rails')
      const axe = await new AxeBuilder({ page }).analyze()
      const byImpact = Object.fromEntries(impacts.map((impact) => [impact, axe.violations.filter((item) => item.impact === impact).map((item) => item.id)]))
      for (const impact of impacts) totals[impact] += byImpact[impact].length
      if (overflow || offerRails || impacts.some((impact) => byImpact[impact].length)) failed = true
      report.push({ name, route, width, title: layout.title, overflow, offerRails, axe: byImpact })
      const axeText = impacts.map((impact) => `${impact}=${byImpact[impact].length}`).join(' ')
      console.log(`${name} ${width} overflow=${overflow} offerRails=${offerRails} ${axeText} title=${layout.title}`)
    }
    await context.close()
  }
} finally {
  await browser.close()
  server.close()
}
writeFileSync(join(outDir, 'report.json'), JSON.stringify({ totals, report }, null, 2))
console.log('axe totals ' + impacts.map((impact) => `${impact}=${totals[impact]}`).join(' '))
if (failed) process.exit(1)
