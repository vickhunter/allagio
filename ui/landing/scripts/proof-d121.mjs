import { createServer } from 'node:http'
import { readFileSync, mkdirSync, statSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'

const outDir = process.argv[2]
if (!outDir) {
  console.error('usage: node scripts/proof-d121.mjs <out-dir>')
  process.exit(2)
}

const build = resolve('/workspace/affilio-wp/admin/build')
const routes = [
  ['home', '/'],
  ['you', '/you/'],
  ['agents', '/agents'],
  ['makers', '/makers/'],
  ['publishers', '/publishers/'],
  ['setup', '/publishers/setup'],
  ['protocol', '/protocol/'],
]
const widths = [390, 1440]

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
      if (!statSync(file).isFile()) throw new Error('dir')
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
  executablePath: process.env.CHROME_PATH || '/usr/local/bin/chrome',
  args: ['--disable-gpu'],
})
const report = []
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
      }))
      const overflow = layout.scrollWidth > layout.innerWidth + 1
      const axe = await new AxeBuilder({ page }).analyze()
      const serious = axe.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
      if (overflow || serious.length) failed = true
      report.push({ name, route, width, title: layout.title, overflow, axe: serious.map((item) => item.id) })
      console.log(`${name} ${width} overflow=${overflow} axe=${serious.map((item) => item.id).join(',') || '0'} title=${layout.title}`)
    }
    await context.close()
  }
} finally {
  await browser.close()
  server.close()
}
writeFileSync(join(outDir, 'report.json'), JSON.stringify(report, null, 2))
if (failed) process.exit(1)
