// v9 proof: system Chrome headless=new, non-localhost requests blocked, full-page 390/1440, reduced-motion, footer close-up, axe.
import { chromium } from 'playwright-core'
import AxeBuilder from '@axe-core/playwright'
import { writeFileSync } from 'node:fs'
const OUT = '/workspace/builds/offer-rails/proof/landing-v9'
const BASE = process.env.BASE || 'http://127.0.0.1:4179'
const pages = [['home', '/'], ['makers', '/makers/'], ['publishers', '/publishers/']]
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--headless=new', '--enable-unsafe-webgpu', '--enable-features=Vulkan', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] })
const results = {}
for (const [name, path] of pages) {
  for (const [w, h] of [[390, 844], [1440, 900]]) {
    for (const motion of ['no-preference', 'reduce']) {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, reducedMotion: motion, deviceScaleFactor: 1 })
      const page = await ctx.newPage()
      const blocked = [], logs = []
      await page.route('**/*', (r) => { const u = new URL(r.request().url()); if (['127.0.0.1', 'localhost'].includes(u.hostname)) r.continue(); else { blocked.push(u.href); r.abort() } })
      page.on('console', (m) => logs.push(`${m.type()}: ${m.text()}`))
      page.on('pageerror', (e) => logs.push(`pageerror: ${e.message}`))
      await page.goto(BASE + path, { waitUntil: 'networkidle' })
      await page.waitForTimeout(2500)
      await page.waitForFunction(() => document.getAnimations().every((a) => a.effect.getTiming().iterations === Infinity || a.playState !== 'running'), null, { timeout: 15000 }).catch(() => {})
      if (name === 'makers' && motion === 'no-preference') await page.waitForTimeout(9000)
      const info = await page.evaluate(() => ({
        overflow: document.documentElement.scrollWidth - innerWidth,
        gpu: 'gpu' in navigator, heroBg: document.querySelector('[data-hero-bg]')?.getAttribute('data-hero-bg') ?? null,
        canvas: !!document.querySelector('[data-hero-bg] canvas'),
        keycaps: document.querySelectorAll('.keycap').length,
        unconfirmed: document.querySelectorAll('[data-unconfirmed]').length,
        tramAnim: getComputedStyle(document.querySelector('.tram')).animationName,
      }))
      const key = `${name}-${w}-${motion}`
      if (motion === 'no-preference') await page.screenshot({ path: `${OUT}/${name}-${w}.png`, fullPage: true })
      else await page.screenshot({ path: `${OUT}/${name}-${w}-reduced-motion.png`, fullPage: w === 390 ? false : true })
      if (name === 'home' && w === 1440 && motion === 'no-preference') {
        const f = page.locator('footer')
        await f.scrollIntoViewIfNeeded(); await page.waitForTimeout(1500)
        const on = await page.evaluate(() => document.querySelector('.padova').dataset.onscreen)
        info.footerOnscreen = on
        await f.screenshot({ path: `${OUT}/footer-1440.png` })
        await page.locator('.padova').screenshot({ path: `${OUT}/footer-skyline-closeup.png` })
        await page.evaluate(() => scrollTo(0, 0)); await page.waitForTimeout(500)
        info.footerOffscreen = await page.evaluate(() => [document.querySelector('.padova').dataset.onscreen, getComputedStyle(document.querySelector('.tram')).animationPlayState])
        // accordion + tracking smoke test
        await page.locator('#faq button[aria-expanded="false"]').first().click(); await page.waitForTimeout(400)
        info.track = await page.evaluate(() => window.__track.map((t) => t.event))
      }
      if (name === 'home' && w === 390 && motion === 'no-preference') await page.locator('footer').screenshot({ path: `${OUT}/footer-390.png` })
      const axe = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa', 'best-practice']).analyze()
      results[key] = { ...info, violations: axe.violations.map((x) => ({ id: x.id, impact: x.impact, n: x.nodes.length, t: x.nodes.slice(0, 3).map((n) => n.target.join(' ') + ' :: ' + n.failureSummary?.slice(0, 160)) })), passes: axe.passes.length, blocked, errors: logs.filter((l) => /error/i.test(l)) }
      console.log(key, JSON.stringify({ ...info, v: results[key].violations.map((v) => `${v.id}:${v.impact}:${v.n}`), blocked: blocked.length, errors: results[key].errors }))
      await ctx.close()
    }
  }
}
writeFileSync(`${OUT}/axe.json`, JSON.stringify(results, null, 2))
await browser.close()
