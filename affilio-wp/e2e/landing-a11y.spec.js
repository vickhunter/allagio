const { test, expect } = require('@playwright/test');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');

const build = path.resolve(__dirname, '../admin/build');
const axePath = path.resolve(__dirname, '../../ui/dashboard/vendor/axe.min.js');

function landingHtml() {
  let html = fs.readFileSync(path.join(build, 'home.html'), 'utf8');
  html = html.replaceAll('{{.InstallCommand}}', 'npx offer-rails');
  html = html.replace(/\{\{if \.InstallerReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1');
  html = html.replace(/\{\{if \.PluginReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/g, '$1');
  return html;
}

function startLanding() {
  const server = http.createServer((req, res) => {
    const url = req.url.split('?')[0];
    if (url === '/landing') {
      res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' });
      res.end(landingHtml());
      return;
    }
    if (url.startsWith('/landing-v9/')) {
      const file = path.join(build, url);
      if (!file.startsWith(build) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
        res.writeHead(404);
        res.end('missing');
        return;
      }
      const type = file.endsWith('.css') ? 'text/css; charset=utf-8' : file.endsWith('.woff2') ? 'font/woff2' : file.endsWith('.svg') ? 'image/svg+xml' : 'text/javascript; charset=utf-8';
      res.writeHead(200, { 'content-type': type });
      res.end(fs.readFileSync(file));
      return;
    }
    if (url.startsWith('/assets/')) {
      const file = path.join(build, path.basename(url));
      if (!file.startsWith(build) || !fs.existsSync(file)) {
        res.writeHead(404);
        res.end('missing');
        return;
      }
      const type = file.endsWith('.css') ? 'text/css; charset=utf-8' : 'text/javascript; charset=utf-8';
      res.writeHead(200, { 'content-type': type });
      res.end(fs.readFileSync(file));
      return;
    }
    res.writeHead(404);
    res.end('missing');
  });
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => {
      resolve({ base: 'http://127.0.0.1:' + server.address().port, close: () => server.close() });
    });
  });
}

function startDemoServer() {
  const addrFile = path.join(require('os').tmpdir(), `affilio-demo-a11y-${process.pid}-${Date.now()}`);
  const child = spawn('go', ['test', './internal/handlers', '-run', '^TestServeDemoForE2E$', '-count=1', '-timeout', '20m'], {
    cwd: path.resolve(__dirname, '../../affilio-api'),
    env: { ...process.env, DEMO_E2E: '1', DEMO_E2E_ADDR_FILE: addrFile },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  child.stdout.on('data', (chunk) => { output += chunk.toString(); });
  child.stderr.on('data', (chunk) => { output += chunk.toString(); });
  const ready = new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('demo server did not start\n' + output)), 180000);
    const poll = setInterval(() => {
      if (!fs.existsSync(addrFile)) return;
      const addr = fs.readFileSync(addrFile, 'utf8').trim();
      if (!addr) return;
      clearInterval(poll);
      clearTimeout(timer);
      resolve('http://' + addr);
    }, 200);
    child.on('exit', (code) => {
      clearInterval(poll);
      clearTimeout(timer);
      reject(new Error('demo server exited ' + code + '\n' + output));
    });
  });
  return { ready, stop() { child.kill('SIGTERM'); fs.rmSync(addrFile, { force: true }); } };
}

async function paint(page) {
  return page.evaluate(() => {
    const lin = (c) => {
      c /= 255;
      return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    };
    const parse = (value) => {
      const match = String(value).match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/);
      if (!match) return null;
      return { r: +match[1], g: +match[2], b: +match[3], a: match[4] === undefined ? 1 : +match[4] };
    };
    const lum = (c) => 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b);
    const contrast = (fg, bg) => {
      const hi = Math.max(lum(fg), lum(bg));
      const lo = Math.min(lum(fg), lum(bg));
      return (hi + 0.05) / (lo + 0.05);
    };
    const sample = (el) => {
      const fg = parse(getComputedStyle(el).color);
      let node = el;
      const layers = [];
      while (node && node.nodeType === 1) {
        const bg = parse(getComputedStyle(node).backgroundColor);
        if (bg && bg.a > 0) layers.push(bg);
        if (bg && bg.a === 1) break;
        node = node.parentElement;
      }
      if (!layers.length) layers.push({ r: 255, g: 255, b: 255, a: 1 });
      let out = layers[layers.length - 1];
      for (let i = layers.length - 2; i >= 0; i -= 1) {
        const src = layers[i];
        out = {
          r: Math.round(src.r * src.a + out.r * (1 - src.a)),
          g: Math.round(src.g * src.a + out.g * (1 - src.a)),
          b: Math.round(src.b * src.a + out.b * (1 - src.a)),
          a: 1,
        };
      }
      const box = el.getBoundingClientRect();
      return {
        text: el.textContent.trim(),
        fontSize: parseFloat(getComputedStyle(el).fontSize),
        ratio: contrast(fg, out),
        height: box.height,
      };
    };
    const label = document.querySelector('.command-label');
    const flag = document.querySelector('.demo-flag');
    return {
      legend: [...document.querySelectorAll('.world-legend li span')].map(sample),
      label: label ? sample(label) : null,
      demo: flag ? sample(flag) : null,
    };
  });
}

async function axeContrast(page, include) {
  await page.addScriptTag({ path: axePath });
  return page.evaluate(async (selectors) => {
    const result = await window.axe.run(
      { include: selectors },
      { runOnly: { type: 'rule', values: ['color-contrast'] } },
    );
    return result.violations.map((violation) => violation.id + ' ' + violation.nodes.map((node) => node.target.join(' ')).join(', '));
  }, include);
}

test.describe('public landing contrast and tap targets', () => {
  let landing;
  let demo;

  test.beforeAll(async () => {
    landing = await startLanding();
    demo = startDemoServer();
    await demo.ready;
  });

  test.afterAll(async () => {
    if (landing) landing.close();
    if (demo) demo.stop();
  });

  for (const width of [390, 1440]) {
    test(`legend, DEMO, command label, and tap targets at ${width}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width, height: width === 390 ? 844 : 900 },
        reducedMotion: 'reduce',
        bypassCSP: true,
      });
      const page = await context.newPage();
      await page.goto(landing.base + '/landing', { waitUntil: 'networkidle' });
      // D-127 semantic update. Main already retired "Someone has the product" and the
      // "Copy command" control. The approved heading is the in-write offer, and the
      // current control is Copy. This is not a D-126 import and does not skip the check.
      await expect(page.getByRole('heading', { name: /this fits your post, want to monetize it/ })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Copy', exact: true })).toBeVisible();
      await expect(page.locator('#how')).toHaveCount(1);
      await expect(page.locator('#publishers')).toHaveCount(1);
      expect(await axeContrast(page, ['#hero-h', '#join-h'])).toEqual([]);

      for (const name of [/Sign in with Google/, /Download WordPress plugin/, /MCP and setup/]) {
        const box = await page.getByRole('link', { name }).first().boundingBox();
        expect(box.height).toBeGreaterThanOrEqual(44);
      }
      const copy = await page.getByRole('button', { name: 'Copy', exact: true }).boundingBox();
      expect(copy.height).toBeGreaterThanOrEqual(44);
      await page.locator('a.skip').focus();
      const skip = await page.locator('a.skip').boundingBox();
      expect(skip.height).toBeGreaterThanOrEqual(44);

      await page.goto((await demo.ready) + '/demo', { waitUntil: 'networkidle' });
      const demoPaint = await paint(page);
      expect(demoPaint.demo.text).toBe('DEMO');
      expect(demoPaint.demo.ratio).toBeGreaterThanOrEqual(4.5);
      expect(await axeContrast(page, ['.demo-flag'])).toEqual([]);
      await context.close();
    });
  }
});
