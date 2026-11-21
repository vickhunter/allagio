const { test, expect } = require('@playwright/test');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');

// Scenario: maker owner walk
//   Given a signed-in maker with no payment provider connected
//   When they register a product whose site is /demo
//   And readiness reports ready
//   And they create an offer and submit it
//   Then the offer lists immediately — no operator gate

const build = path.resolve(__dirname, '../admin/build');
const template = fs.readFileSync(path.join(build, 'workspace.html'), 'utf8');

function shell(title) {
  return template
    .replaceAll('{{.CSRF}}', 'test-csrf')
    .replaceAll('{{.Title}}', title)
    .replace(/\{\{if \.Message\}\}[\s\S]*?\{\{end\}\}/, '')
    .replace(/\{\{if \.Name\}\}[\s\S]*?\{\{end\}\}/, '')
    .replace(/\{\{if \.Operator\}\}([\s\S]*?)\{\{end\}\}/, '$1')
    .replace('{{.Body}}', '<div id="workspace-app"></div>');
}

const state = { products: [], offers: [], checks: {} };

function send(res, status, body, type) {
  const payload = Buffer.isBuffer(body) ? body : Buffer.from(typeof body === 'string' ? body : JSON.stringify(body));
  res.writeHead(status, { 'Content-Type': type || 'application/json; charset=utf-8', 'Content-Length': payload.length, 'Cache-Control': 'no-store' });
  res.end(payload);
}

function readBody(req) {
  return new Promise(resolve => {
    const chunks = [];
    req.on('data', chunk => chunks.push(chunk));
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8');
      if (!raw) return resolve({});
      try { resolve(JSON.parse(raw)); } catch { resolve({}); }
    });
  });
}

const demo = {
  '/demo': '<html><body><a href="/demo/start">Get started</a><a href="/demo/privacy">Privacy</a><a href="/demo/terms">Terms</a></body></html>',
  '/demo/pricing': '<html><body>Pro plan: $19/month</body></html>',
  '/demo/docs': '<html><body>' + 'Offer Rails demo documentation. '.repeat(40) + '</body></html>',
  '/demo/privacy': '<html><body>Privacy policy for the demo product.</body></html>',
  '/demo/terms': '<html><body>Terms for the demo product.</body></html>',
};

async function handle(req, res) {
  const url = new URL(req.url, 'http://127.0.0.1');
  const pathname = url.pathname;
  if (demo[pathname] && !pathname.endsWith('.js')) return send(res, 200, demo[pathname], 'text/html; charset=utf-8');
  if (pathname === '/health') return send(res, 200, { ok: true });
  if (pathname === '/owner/api/session') {
    return send(res, 200, { maker: true, publisher: false, operator: true, email: 'owner@example.com', owner_id: 'owner-1', install_command: 'npx offer-rails', mcp_url: 'https://example.test/mcp' });
  }
  if (pathname === '/owner/api/products' && req.method === 'GET') return send(res, 200, { products: state.products });
  if (pathname === '/owner/api/products' && req.method === 'POST') {
    const body = await readBody(req);
    const product = { id: randomUUID(), name: body.name, website_url: body.website_url, tagline: body.tagline, docs_url: body.docs_url || '', pricing_url: body.pricing_url || '', keywords: body.keywords || [], readiness_status: 'unassessed', readiness_score: 0 };
    state.products.push(product);
    return send(res, 201, { product });
  }
  const assess = pathname.match(/^\/owner\/api\/products\/([^/]+)\/assess$/);
  if (assess && req.method === 'POST') {
    const product = state.products.find(item => item.id === assess[1]);
    if (!product || !String(product.website_url).endsWith('/demo')) return send(res, 409, { error: 'demo site required' });
    product.readiness_status = 'ready';
    product.readiness_score = 90;
    const checks = [
      { key: 'site_reachable', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'landing page returned HTTP 200' },
      { key: 'checkout_present', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'Get started link found.' },
      { key: 'payment_provider', kind: 'probe', gate: false, status: 'warn', score: 60, evidence: 'no known payment provider' },
      { key: 'pricing_page', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'pricing page resolves and states a price' },
      { key: 'docs_present', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'docs resolve' },
      { key: 'privacy_policy', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'Privacy link found.' },
      { key: 'terms', kind: 'probe', gate: true, status: 'pass', score: 100, evidence: 'Terms link found.' },
    ];
    state.checks[product.id] = checks;
    return send(res, 200, { readiness_status: 'ready', readiness_score: 90, checks });
  }
  const assessments = pathname.match(/^\/owner\/api\/products\/([^/]+)\/assessments$/);
  if (assessments) return send(res, 200, { assessments: state.checks[assessments[1]] ? [{ checks: state.checks[assessments[1]] }] : [] });
  if (pathname === '/owner/api/offers' && req.method === 'GET') return send(res, 200, { offers: state.offers });
  if (pathname === '/owner/api/offers' && req.method === 'POST') {
    const body = await readBody(req);
    const offer = { id: randomUUID(), product_id: body.product_id, status: 'draft', commission_type: body.commission_type, commission_value: body.commission_value, commission_currency: body.commission_currency, cookie_duration_days: body.cookie_duration_days, refund_window_days: body.refund_window_days, terms: body.terms || '' };
    state.offers.push(offer);
    return send(res, 201, { offer });
  }
  const submit = pathname.match(/^\/owner\/api\/offers\/([^/]+)\/submit$/);
  if (submit && req.method === 'POST') {
    const offer = state.offers.find(item => item.id === submit[1]);
    const product = state.products.find(item => item.id === offer.product_id);
    if (product.readiness_status !== 'ready') return send(res, 409, { error: 'product has not passed readiness assessment', hint: 'run POST /v1/maker/products/:id/assess' });
    offer.status = 'listed';
    return send(res, 200, { id: offer.id, status: 'listed', program_id: 'prog-' + offer.id, review_reason: null, message: null });
  }
  if (pathname === '/owner/api/review/offers' && req.method === 'GET') {
    const queue = state.offers.filter(offer => offer.status === 'submitted').map(offer => {
      const product = state.products.find(item => item.id === offer.product_id);
      return { ...offer, product_name: product.name, product_tagline: product.tagline, website_url: product.website_url, readiness_status: product.readiness_status };
    });
    return send(res, 200, { queue });
  }
  const review = pathname.match(/^\/owner\/api\/review\/offers\/([^/]+)\/approve$/);
  if (review && req.method === 'POST') {
    const offer = state.offers.find(item => item.id === review[1]);
    offer.status = 'listed';
    return send(res, 200, { status: 'listed' });
  }
  if (pathname === '/owner/products' || pathname === '/owner/review') {
    const title = pathname === '/owner/review' ? 'Offer review' : 'Products & offers';
    return send(res, 200, shell(title), 'text/html; charset=utf-8');
  }
  if (pathname.startsWith('/assets/')) {
    const file = path.join(build, path.basename(pathname));
    if (!file.startsWith(build) || !fs.existsSync(file)) return send(res, 404, 'missing');
    const type = file.endsWith('.css') ? 'text/css; charset=utf-8' : 'text/javascript; charset=utf-8';
    return send(res, 200, fs.readFileSync(file), type);
  }
  return send(res, 404, { error: 'not found' });
}

let server;
let base;

test.beforeAll(async () => {
  server = http.createServer((req, res) => { handle(req, res).catch(error => send(res, 500, { error: error.message })); });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${server.address().port}`;
});

test.afterAll(async () => {
  await new Promise(resolve => server.close(resolve));
});

test('owner walk registers /demo, submits, and the offer lists immediately', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(base + '/owner/products');
  await expect(page.getByRole('link', { name: 'Offer review' })).toHaveAttribute('href', '/owner/review');
  await page.getByLabel('Product name', { exact: true }).fill('Demo Product');
  await page.getByLabel('Website', { exact: true }).fill(base + '/demo');
  await page.getByLabel('Pricing URL', { exact: true }).fill(base + '/demo/pricing');
  await page.getByLabel('Documentation URL', { exact: true }).fill(base + '/demo/docs');
  await page.getByLabel('What does it help people do?').fill('Show a maker how Offer Rails readiness works.');
  await page.getByRole('button', { name: 'Register product', exact: true }).click();
  await expect(page.locator('#workspace-feedback')).toContainText('Product registered');
  await expect(page.getByRole('heading', { name: 'Demo Product', exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  await expect(page.locator('[data-station="ready"]')).toHaveClass(/is-done/);
  await expect(page.getByText('no known payment provider')).toBeVisible();

  await page.getByLabel('Commission value', { exact: true }).fill('15');
  await page.getByLabel('Referral destination', { exact: true }).fill(base + '/demo');
  await page.getByRole('button', { name: 'Create offer', exact: true }).click();
  await page.getByRole('button', { name: 'Submit and list', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause offer', exact: true })).toBeVisible();
  await expect(page.locator('body')).not.toContainText('/v1/');

  await page.goto(base + '/owner/review');
  await expect(page.getByText('No offers waiting for review.')).toBeVisible();
  await page.goto(base + '/owner/products');
  await expect(page.locator('[data-station="listed"]')).toHaveClass(/is-done/);
});
