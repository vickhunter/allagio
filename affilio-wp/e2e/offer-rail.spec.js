const { test, expect } = require('@playwright/test');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { randomUUID } = require('node:crypto');

const build = path.resolve(__dirname, '../admin/build');
const adminCss = path.resolve(__dirname, '../admin/css/affilio-admin.css');
const artifacts = process.env.OFFER_RAIL_ARTIFACTS || '/opt/cursor/artifacts/d011';
const d019 = process.env.CI ? path.join(artifacts, 'd019') : '/opt/cursor/artifacts/d019';
const d020 = process.env.CI ? path.join(artifacts, 'd020') : '/opt/cursor/artifacts/d020';
const d023 = process.env.CI ? path.join(artifacts, 'd023') : '/opt/cursor/artifacts/d023';
const template = fs.readFileSync(path.join(build, 'workspace.html'), 'utf8');

function shell(title, body, operator) {
  return template
    .replaceAll('{{.CSRF}}', 'test-csrf')
    .replaceAll('{{.Title}}', title)
    .replaceAll('{{.Body}}', body)
    .replace(/\{\{if \.Message\}\}[\s\S]*?\{\{end\}\}/, '')
    .replace(/\{\{if \.Name\}\}[\s\S]*?\{\{end\}\}/, '')
    .replace(/\{\{if \.Operator\}\}([\s\S]*?)\{\{end\}\}/, operator ? '$1' : '');
}

const state = {
  products: [],
  offers: [],
  assessments: {},
  assessCounts: {},
  failProducts: false,
  assessDelay: 0,
  failJoin: false,
  failMarketplace: false,
  failDistribution: false,
  failFind: false,
  failEarnings: false,
  failOffer: false,
  healthMode: '',
  healthHits: 0,
  healthFails: 0,
  failProductCreate: false,
};

function reset() {
  state.products = [];
  state.offers = [];
  state.assessments = {};
  state.assessCounts = {};
  state.failProducts = false;
  state.assessDelay = 0;
  state.failJoin = false;
  state.failMarketplace = false;
  state.failDistribution = false;
  state.failFind = false;
  state.failEarnings = false;
  state.failOffer = false;
  state.healthMode = '';
  state.healthHits = 0;
  state.healthFails = 0;
  state.failProductCreate = false;
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

function send(res, status, body, type) {
  const payload = Buffer.isBuffer(body) ? body : Buffer.from(typeof body === 'string' ? body : JSON.stringify(body));
  res.writeHead(status, { 'Content-Type': type || 'application/json; charset=utf-8', 'Content-Length': payload.length, 'Cache-Control': 'no-store' });
  res.end(payload);
}

function productJSON(product) {
  return { ...product };
}

function failingChecks() {
  return [
    { key: 'site_reachable', kind: 'probe', gate: true, status: 'fail', score: 0, evidence: 'landing page returned HTTP 0', remediation: 'Publish the site so the readiness check can reach it.' },
    { key: 'checkout_present', kind: 'probe', gate: true, status: 'pass', score: 1, evidence: 'Stripe checkout marker found.' },
    { key: 'ref_param_survives', kind: 'probe', gate: true, status: 'pass', score: 1, evidence: 'Referral parameter stayed on the final URL.' },
    { key: 'pricing_page', kind: 'probe', gate: false, status: 'warn', score: 0, evidence: 'No pricing page URL.', remediation: 'Add a pricing page so publishers can see the price.' },
  ];
}

function passingChecks() {
  return [
    { key: 'site_reachable', kind: 'probe', gate: true, status: 'pass', score: 1, evidence: 'landing page returned HTTP 200' },
    { key: 'checkout_present', kind: 'probe', gate: true, status: 'pass', score: 1, evidence: 'Stripe checkout marker found.' },
    { key: 'ref_param_survives', kind: 'probe', gate: true, status: 'pass', score: 1, evidence: 'Referral parameter stayed on the final URL.' },
    { key: 'pricing_page', kind: 'probe', gate: false, status: 'pass', score: 1, evidence: 'Pricing page is reachable.' },
  ];
}

const session = {
  maker: true,
  publisher: false,
  operator: true,
  email: 'owner@example.com',
  owner_id: 'owner-1',
  install_command: 'npx distributoor-setup',
  mcp_url: 'https://example.test/mcp',
};

async function handle(req, res) {
  const url = new URL(req.url, 'http://127.0.0.1');
  const pathname = url.pathname;
  if (req.method === 'POST' && pathname === '/__test/mode') {
    Object.assign(state, await readBody(req));
    return send(res, 200, { ok: true });
  }
  if (pathname === '/owner/api/session') return send(res, 200, session);
  if (pathname === '/health') {
    state.healthHits += 1;
    if (state.healthMode === 'never') return send(res, 503, { error: 'asleep' });
    if (state.healthMode === 'slow' && state.healthHits <= state.healthFails) return send(res, 503, { error: 'asleep' });
    return send(res, 200, { ok: true });
  }
  if (pathname === '/owner/api/agents' && req.method === 'GET') return send(res, 200, { grants: [] });
  if (pathname === '/owner/api/products' && req.method === 'GET') {
    if (state.failProducts) return send(res, 503, { error: 'The product list could not be loaded.' });
    return send(res, 200, { products: state.products.map(productJSON) });
  }
  if (pathname === '/owner/api/products' && req.method === 'POST') {
    const body = await readBody(req);
    if (state.failProductCreate) {
      state.failProductCreate = false;
      return send(res, 503, { error: 'asleep' });
    }
    const product = { id: randomUUID(), name: body.name, website_url: body.website_url, tagline: body.tagline, docs_url: body.docs_url || '', pricing_url: body.pricing_url || '', primary_category: body.primary_category || '', keywords: body.keywords || [], readiness_status: 'unassessed', readiness_score: 0 };
    state.products.push(product);
    return send(res, 201, { product });
  }
  const productPatch = pathname.match(/^\/owner\/api\/products\/([^/]+)$/);
  if (productPatch && req.method === 'PATCH') {
    const body = await readBody(req);
    const product = state.products.find(item => item.id === productPatch[1]);
    if (!product) return send(res, 404, { error: 'product not found' });
    Object.assign(product, body, { id: product.id });
    return send(res, 200, { product });
  }
  const assess = pathname.match(/^\/owner\/api\/products\/([^/]+)\/assess$/);
  if (assess && req.method === 'POST') {
    if (state.assessDelay) await new Promise(resolve => setTimeout(resolve, state.assessDelay));
    const product = state.products.find(item => item.id === assess[1]);
    if (!product) return send(res, 404, { error: 'product not found' });
    state.assessCounts[product.id] = (state.assessCounts[product.id] || 0) + 1;
    const failed = state.assessCounts[product.id] === 1;
    const checks = failed ? failingChecks() : passingChecks();
    product.readiness_status = failed ? 'failing' : 'ready';
    product.readiness_score = failed ? 40 : 90;
    const record = { id: randomUUID(), checks, status: product.readiness_status, score: product.readiness_score };
    state.assessments[product.id] = [record];
    return send(res, 200, { assessment_id: record.id, product_id: product.id, readiness_status: product.readiness_status, readiness_score: product.readiness_score, checks });
  }
  const assessments = pathname.match(/^\/owner\/api\/products\/([^/]+)\/assessments$/);
  if (assessments && req.method === 'GET') return send(res, 200, { assessments: state.assessments[assessments[1]] || [] });
  if (pathname === '/owner/api/offers' && req.method === 'GET') return send(res, 200, { offers: state.offers });
  if (pathname === '/owner/api/offers' && req.method === 'POST') {
    const body = await readBody(req);
    if (state.failOffer) return send(res, 422, { error: 'commission value is below the minimum' });
    const offer = { id: randomUUID(), product_id: body.product_id, status: 'draft', commission_type: body.commission_type, commission_value: body.commission_value, commission_currency: body.commission_currency, currency: body.commission_currency, cookie_duration_days: body.cookie_duration_days, refund_window_days: body.refund_window_days, terms: body.terms || '' };
    state.offers.push(offer);
    return send(res, 201, { offer });
  }
  const submit = pathname.match(/^\/owner\/api\/offers\/([^/]+)\/submit$/);
  if (submit && req.method === 'POST') {
    const offer = state.offers.find(item => item.id === submit[1]);
    const product = state.products.find(item => item.id === offer?.product_id);
    if (!product || product.readiness_status !== 'ready') {
      return send(res, 409, { error: 'product has not passed readiness assessment', readiness_status: product?.readiness_status || 'unassessed', hint: 'run POST /v1/maker/products/:id/assess and fix the failing checks' });
    }
    offer.status = 'submitted';
    return send(res, 200, { offer });
  }
  if (pathname === '/owner/api/review/offers' && req.method === 'GET') {
    const queue = state.offers.filter(offer => offer.status === 'submitted').map(offer => {
      const product = state.products.find(item => item.id === offer.product_id);
      return { ...offer, product_name: product?.name || 'Product', product_tagline: product?.tagline || '', website_url: product?.website_url || '', readiness_status: product?.readiness_status || '' };
    });
    return send(res, 200, { queue });
  }
  const review = pathname.match(/^\/owner\/api\/review\/offers\/([^/]+)\/(approve|reject)$/);
  if (review && req.method === 'POST') {
    const offer = state.offers.find(item => item.id === review[1]);
    if (offer) offer.status = review[2] === 'approve' ? 'listed' : 'draft';
    return send(res, 200, { offer });
  }
  const connection = pathname.match(/^\/owner\/products\/([^/]+)\/connection$/);
  if (connection && req.method === 'GET') {
    return send(res, 200, shell('Stripe configuration verified', '<p>Account: acct_test · Price: price_test · Currency: usd · Live mode: false</p><p>Stripe configuration verified</p><p>The verification key was not stored.</p><p><a href="/owner/products">Return to your products</a></p>', true), 'text/html; charset=utf-8');
  }
  if (pathname === '/owner/products' || pathname === '/owner' || pathname === '/owner/review' || pathname === '/owner/settings') {
    const titles = { '/owner/products': 'Products & offers', '/owner': 'Your distribution workspace', '/owner/review': 'Offer review', '/owner/settings': 'Setup & agents' };
    return send(res, 200, shell(titles[pathname], '<div id="workspace-app"></div>', true), 'text/html; charset=utf-8');
  }
  if (pathname.startsWith('/assets/')) {
    const file = path.join(build, path.basename(pathname));
    if (!file.startsWith(build) || !fs.existsSync(file)) return send(res, 404, 'missing');
    const type = file.endsWith('.css') ? 'text/css; charset=utf-8' : 'text/javascript; charset=utf-8';
    return send(res, 200, fs.readFileSync(file), type);
  }
  if (pathname === '/admin/affilio-admin.css') return send(res, 200, fs.readFileSync(adminCss), 'text/css; charset=utf-8');
  if (pathname === '/wp/distribution') return send(res, 200, wpDistribution(), 'text/html; charset=utf-8');
  if (pathname === '/wp/marketplace') return send(res, 200, wpMarketplace(), 'text/html; charset=utf-8');
  if (pathname === '/wp/earnings-down') return send(res, 200, wpEarnings(), 'text/html; charset=utf-8');
  if (pathname === '/landing') return send(res, 200, landing(), 'text/html; charset=utf-8');
  if (pathname === '/prototype' || pathname === '/prototype/' || pathname === '/prototype/share' || pathname === '/prototype/audience' || pathname === '/prototype/payments') {
    return send(res, 200, fs.readFileSync(path.join(build, 'prototype.html')), 'text/html; charset=utf-8');
  }
  if (pathname === '/wp-json/affilio/v1/distribution' && req.method === 'GET') {
    if (state.failDistribution) return send(res, 503, { message: 'Distribution could not be loaded.' });
    return send(res, 200, { site_connected: true, native_available: true, publisher_connected: true, owner_url: 'http://127.0.0.1/owner', settings_url: '/wp-admin/admin.php?page=affilio-settings' });
  }
  if (pathname === '/wp-json/affilio/v1/distribution/articles') return send(res, 200, { articles: [{ id: 7, title: 'Email inbox developer tutorial', url: 'https://publisher.example/tutorial' }] });
  if (pathname === '/wp-json/affilio/v1/distribution/placements') return send(res, 200, { placements: [] });
  if (pathname === '/wp-json/affilio/v1/distribution/opportunities' && req.method === 'POST') {
    if (state.failFind) return send(res, 500, { message: 'Relevant offers could not be loaded. Check the connection and retry.' });
    return send(res, 200, { opportunities: [] });
  }
  if (pathname === '/v1/marketplace/offers' && req.method === 'GET') {
    if (state.failMarketplace) return send(res, 503, { error: 'down' });
    return send(res, 200, { offers: [{ offer_id: 'offer-1', product_name: 'InboxPilot', joined: false, commission_type: 'percentage', commission_value: 20, commission_currency: 'USD', cookie_duration_days: 30, refund_window_days: 14, terms: 'First purchase only.', relevance: 82, product_tagline: 'Inbox cleanup for developers.' }] });
  }
  if (pathname.startsWith('/v1/marketplace/offers/') && pathname.endsWith('/join')) {
    if (state.failJoin) return send(res, 500, { error: 'join failed' });
    return send(res, 200, { joined: true });
  }
  if (pathname === '/v1/payouts/balance' || pathname === '/v1/payouts/history') {
    if (state.failEarnings) return send(res, 503, { error: 'down' });
    if (pathname.endsWith('balance')) return send(res, 200, { balances: [] });
    return send(res, 200, { ledger: [], payouts: [] });
  }
  send(res, 404, { error: 'not found', path: pathname });
}

function wpChrome(title, rootId, extra, scripts) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${title}</title><link rel="stylesheet" href="/admin/affilio-admin.css"><link rel="stylesheet" href="/assets/distribution.css"><style>body{margin:0;background:#f0f0f1;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#1d2327}.wrap{max-width:1100px;margin:0 auto;padding:16px}h1{font-size:23px;font-weight:400}.button{display:inline-block;padding:6px 12px;border:1px solid #2271b1;background:#f6f7f7;color:#2271b1;border-radius:3px;text-decoration:none;font-size:13px;cursor:pointer}.button-primary,button[data-primary-action],a[data-primary-action]{background:#2271b1;border-color:#2271b1;color:#fff}</style></head><body class="wp-admin"><div class="wrap"><h1>${title}</h1><div id="${rootId}"${extra}></div></div>${scripts}</body></html>`;
}

function wpDistribution() {
  return wpChrome('Distribution', 'affilio-distribution-root', ' class="affilio-distribution"', '<script>window.AffilioDistribution={restBase:"/wp-json/affilio/v1",nonce:"n"};</script><script src="/assets/distribution.js"></script>');
}

function wpMarketplace() {
  return wpChrome('Marketplace', 'affilio-marketplace-root', '', '<script>window.AffilioData={locale:"en",apiUrl:"/v1",apiKey:"k",settingsUrl:"/wp-admin/admin.php?page=affilio-settings",connectionStatus:{connected:true}};</script><script src="/assets/marketplace.js"></script>');
}

function wpEarnings() {
  return wpChrome('Earnings', 'affilio-earnings-root', '', '<script>window.AffilioData={locale:"en",apiUrl:"/v1",apiKey:"k",settingsUrl:"/wp-admin/admin.php?page=affilio-settings",connectionStatus:{connected:true}};</script><script src="/assets/earnings.js"></script>');
}

function landing() {
  let html = fs.readFileSync(path.join(build, 'home.html'), 'utf8');
  html = html.replaceAll('{{.InstallCommand}}', 'npx distributoor-setup');
  html = html.replace(/\{\{if \.InstallerReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/, '$1');
  html = html.replace(/\{\{if \.PluginReady\}\}([\s\S]*?)\{\{else\}\}[\s\S]*?\{\{end\}\}/, '$1');
  return html;
}

async function onePrimary(page, pattern) {
  const primary = page.locator('[data-primary-action]');
  await expect(primary).toHaveCount(1);
  if (pattern) await expect(primary).toHaveText(pattern);
  return primary;
}

async function fits(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
}

async function shot(page, name) {
  // Full-page captures paint position:fixed at the current scroll offset. Reset first so the sidebar is not shown mid-page.
  await page.evaluate(() => window.scrollTo(0, 0));
  const sidebar = page.locator('aside.sidebar');
  if (await sidebar.count()) {
    const box = await sidebar.boundingBox();
    expect(box.y).toBeLessThanOrEqual(1);
  }
  await page.screenshot({ path: path.join(artifacts, name + '.png'), fullPage: true });
}

async function shotD020(page, name) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const sidebar = page.locator('aside.sidebar');
  if (await sidebar.count()) {
    const box = await sidebar.boundingBox();
    expect(box.y).toBeLessThanOrEqual(1);
  }
  fs.mkdirSync(d020, { recursive: true });
  await page.screenshot({ path: path.join(d020, name + '.png'), fullPage: false });
}

const workspaceNav = ['Overview', 'Products & offers', 'Placements', 'Earnings', 'Setup & agents', 'Offer review'];

async function navFits(page) {
  const viewport = page.viewportSize();
  for (const name of workspaceNav) {
    const link = page.getByRole('link', { name, exact: true });
    await expect(link).toBeVisible();
    const box = await link.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(-1);
    expect(box.y).toBeGreaterThanOrEqual(-1);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width + 1);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
  }
  const clipped = await page.locator('nav[aria-label="Workspace"]').evaluate(nav => {
    const navClipped = nav.scrollWidth > nav.clientWidth + 1 || nav.scrollHeight > nav.clientHeight + 1;
    const itemClipped = [...nav.querySelectorAll('a')].some(link => link.scrollWidth > link.clientWidth + 1 || link.scrollHeight > link.clientHeight + 1);
    return navClipped || itemClipped;
  });
  expect(clipped).toBe(false);
  await fits(page);
}

async function frame(page, name, fullPage = false, keepScroll = false) {
  if (!keepScroll) await page.evaluate(() => window.scrollTo(0, 0));
  const sidebar = page.locator('aside.sidebar');
  if (await sidebar.count()) {
    const box = await sidebar.boundingBox();
    expect(box.y).toBeLessThanOrEqual(1);
  }
  fs.mkdirSync(d019, { recursive: true });
  await page.screenshot({ path: path.join(d019, name + '.png'), fullPage });
}

async function viewportShot(page, name) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const box = await page.locator('aside.sidebar').boundingBox();
  expect(box.y).toBeLessThanOrEqual(1);
  await page.screenshot({ path: path.join(artifacts, name + '.png'), fullPage: false });
}

async function registerProduct(page, name = 'Rail Tool') {
  await page.getByLabel('Product name', { exact: true }).fill(name);
  await page.getByLabel('Website', { exact: true }).fill('https://rail-tool.example');
  await page.getByLabel('What does it help people do?').fill('Help developers ship a referral offer.');
  await page.getByLabel('Topics, separated by commas').fill('postgres, databases');
  await page.getByRole('button', { name: 'Register product', exact: true }).click();
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
}

async function createOffer(page) {
  await page.getByLabel('Commission value', { exact: true }).fill('20');
  await page.getByLabel('Referral destination', { exact: true }).fill('https://rail-tool.example');
  await page.getByRole('button', { name: 'Create offer', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Submit for review', exact: true })).toBeVisible();
}

let server;
let base;

test.beforeAll(async () => {
  fs.mkdirSync(artifacts, { recursive: true });
  server = http.createServer((req, res) => { handle(req, res).catch(error => send(res, 500, { error: error.message })); });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${server.address().port}`;
});

test.afterAll(async () => {
  await new Promise(resolve => server.close(resolve));
});

test.beforeEach(() => reset());

async function journey(page, width) {
  await page.goto(base + '/owner/products');
  await onePrimary(page, 'Register product');
  await expect(page.locator('[data-station="product"]')).toHaveAttribute('aria-current', 'step');
  await fits(page);
  await shot(page, `after-owner-rail-product-empty-${width}`);
  if (width === 1440) await viewportShot(page, 'after-owner-viewport-1440');

  await registerProduct(page);
  await onePrimary(page, 'Check readiness');
  await expect(page.locator('[data-station="product"]')).toHaveClass(/is-done/);
  await fits(page);
  await shot(page, `after-owner-rail-product-${width}`);

  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  const failure = page.locator('.check.fail .field-error').first();
  await expect(failure).toBeVisible();
  await expect(failure).toBeFocused();
  await expect(failure).toContainText('Publish the site so the readiness check can reach it.');
  const box = await failure.boundingBox();
  const viewport = page.viewportSize();
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
  await onePrimary(page, 'Check readiness');
  await fits(page);
  await shot(page, `after-owner-rail-ready-fail-${width}`);

  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  await expect(page.locator('.check.fail')).toHaveCount(0);
  await expect(page.locator('[data-station="ready"]')).toHaveClass(/is-done/);
  await onePrimary(page, 'Connect Stripe');
  await fits(page);
  await shot(page, `after-owner-rail-stripe-${width}`);

  await page.getByRole('link', { name: 'Connect Stripe', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Stripe configuration verified' })).toBeVisible();
  await page.getByRole('link', { name: 'Return to your products', exact: true }).click();
  await expect(page.locator('[data-station="stripe"]')).toHaveClass(/is-done/);
  await onePrimary(page, 'Create offer');
  await fits(page);
  await shot(page, `after-owner-rail-offer-${width}`);

  await createOffer(page);
  await onePrimary(page, 'Submit for review');
  await expect(page.locator('[data-station="offer"]')).toHaveClass(/is-done/);
  await fits(page);
  await shot(page, `after-owner-rail-submit-${width}`);

  await page.getByRole('button', { name: 'Submit for review', exact: true }).click();
  await onePrimary(page, 'Waiting for the operator to list this offer');
  await expect(page.locator('[data-station="submitted"]')).toHaveClass(/is-done/);
  await fits(page);
  await shot(page, `after-owner-rail-submitted-${width}`);

  const operator = await page.context().newPage();
  await operator.goto(base + '/owner/review');
  await operator.getByRole('button', { name: 'List offer', exact: true }).click();
  await expect(operator.getByText('No offers waiting for review.')).toBeVisible();
  await operator.close();

  await page.goto(base + '/owner/products');
  await onePrimary(page, 'Listed. Publishers can place this offer.');
  await expect(page.locator('[data-station="listed"]')).toHaveClass(/is-done/);
  await expect(page.getByRole('button', { name: 'Pause offer', exact: true })).toBeVisible();
  await fits(page);
  await shot(page, `after-owner-rail-listed-${width}`);
  if (width === 1440) await viewportShot(page, 'after-owner-viewport-listed-1440');
}

test('offer rail happy path at 1440', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    recordVideo: { dir: '/tmp/d011-video', size: { width: 1440, height: 900 } },
  });
  const page = await context.newPage();
  await journey(page, 1440);
  const video = page.video();
  await context.close();
  const source = await video.path();
  execFileSync('ffmpeg', ['-y', '-i', source, '-filter:v', 'setpts=0.25*PTS', '-an', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '30', path.join(artifacts, 'after-owner-rail-happy-path.mp4')], { stdio: 'inherit' });
});

test('offer rail happy path at 390', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  await journey(page, 390);
  await context.close();
});

test('submit before readiness keeps the error in view', async ({ browser }) => {
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    reset();
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    await page.goto(base + '/owner/products');
    await registerProduct(page, 'Early Submit');
    await createOffer(page);
    await expect(page.locator('[data-primary-action]')).toHaveText('Check readiness');
    await page.getByRole('button', { name: 'Submit for review', exact: true }).click();
    await expect(page.locator('#workspace-feedback')).toContainText('product has not passed readiness assessment');
    await expect(page.locator('#workspace-feedback')).toHaveAttribute('aria-hidden', 'true');
    const error = page.locator('.field-error').first();
    await expect(error).toBeFocused();
    await expect(error).toContainText('product has not passed readiness assessment');
    const box = await error.boundingBox();
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
    await onePrimary(page, 'Check readiness');
    await fits(page);
    await shot(page, `after-owner-submit-error-${viewport.width}`);
    await context.close();
  }
});

test('returned draft, network failure, and assessment running', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  await page.goto(base + '/owner/products');
  await registerProduct(page, 'Returned Tool');
  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  await expect(page.locator('.check.fail .field-error')).toBeVisible();
  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  await page.getByRole('link', { name: 'Connect Stripe', exact: true }).click();
  await page.getByRole('link', { name: 'Return to your products', exact: true }).click();
  await createOffer(page);
  await page.getByRole('button', { name: 'Submit for review', exact: true }).click();
  await onePrimary(page, 'Waiting for the operator to list this offer');
  const operator = await context.newPage();
  await operator.goto(base + '/owner/review');
  await operator.getByRole('button', { name: 'Return to draft', exact: true }).click();
  await operator.close();
  await page.reload();
  const returned = page.locator('[id^="offer-returned-"]');
  await expect(returned).toBeVisible();
  await expect(returned).toContainText('returned this offer to draft');
  await onePrimary(page, 'Submit for review');
  await fits(page);
  await shot(page, 'after-owner-rail-returned-1440');

  await page.evaluate(async endpoint => { await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ failProducts: true }) }); }, base + '/__test/mode');
  await page.goto(base + '/owner/products');
  await onePrimary(page, 'Retry');
  await expect(page.getByRole('alert')).toContainText('could not be loaded');
  await shot(page, 'after-owner-network-1440');

  await page.evaluate(async endpoint => { await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ failProducts: false, assessDelay: 1600, assessCounts: {} }) }); }, base + '/__test/mode');
  await page.goto(base + '/owner/products');
  await page.getByRole('button', { name: 'Check readiness', exact: true }).click();
  await expect(page.getByText('Assessment running. Reading the live pages.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Checking live pages…' })).toBeVisible();
  await shot(page, 'after-owner-assess-running-1440');
  await expect(page.getByText('Assessment running. Reading the live pages.')).toBeHidden();
  await context.close();
});

test('wordpress distribution, marketplace, and earnings states', async ({ browser }) => {
  for (const viewport of [{ width: 1440, height: 900, tag: '1440' }, { width: 390, height: 844, tag: '390' }]) {
    reset();
    const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } });
    const page = await context.newPage();
    await page.goto(base + '/wp/distribution');
    await expect(page.getByRole('heading', { name: 'Your publisher account is connected.' })).toBeVisible();
    await onePrimary(page, 'Find relevant offers');
    await fits(page);
    await shot(page, `after-wp-distribution-${viewport.tag}`);
    state.failFind = true;
    await page.getByRole('button', { name: 'Find relevant offers', exact: true }).click();
    const error = page.locator('.distribution-field-error');
    await expect(error).toBeFocused();
    await expect(error).toContainText('Relevant offers could not be loaded');
    await expect(page.locator('#distribution-message')).toContainText('Relevant offers could not be loaded');
    await expect(page.locator('#distribution-message')).toHaveAttribute('aria-hidden', 'true');
    const box = await error.boundingBox();
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
    await shot(page, `after-wp-distribution-error-${viewport.tag}`);
    state.failFind = false;

    const dialogs = [];
    page.on('dialog', dialog => { dialogs.push(dialog.message()); dialog.dismiss(); });
    await page.goto(base + '/wp/marketplace');
    await onePrimary(page, 'Join offer');
    await fits(page);
    await shot(page, `after-wp-marketplace-${viewport.tag}`);
    state.failJoin = true;
    await page.getByRole('button', { name: 'Join offer', exact: true }).click();
    const joinError = page.locator('.affilio-field-error');
    await expect(joinError).toBeFocused();
    await expect(joinError).toContainText('Could not join the offer');
    expect(dialogs).toEqual([]);
    const joinBox = await joinError.boundingBox();
    expect(joinBox.y).toBeGreaterThanOrEqual(0);
    expect(joinBox.y + joinBox.height).toBeLessThanOrEqual(viewport.height + 1);
    await shot(page, `after-wp-marketplace-error-${viewport.tag}`);
    state.failJoin = false;

    state.failMarketplace = true;
    await page.goto(base + '/wp/marketplace');
    await onePrimary(page, 'Retry');
    await shot(page, `after-wp-marketplace-failed-${viewport.tag}`);
    state.failMarketplace = false;

    state.failEarnings = true;
    await page.goto(base + '/wp/earnings-down');
    await onePrimary(page, 'Retry');
    await expect(page.getByRole('alert')).toContainText('Could not load your balance');
    await fits(page);
    await shot(page, `after-wp-earnings-failed-${viewport.tag}`);
    state.failEarnings = false;
    await context.close();
  }
});

test('new offer collapses once an offer exists', async ({ browser }) => {
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    reset();
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    await page.goto(base + '/owner/products');
    await registerProduct(page, 'Collapsed Tool');
    const productId = await page.evaluate(async () => (await (await fetch('/owner/api/products')).json()).products[0].id);
    await page.evaluate(async id => {
      await fetch('/owner/api/products/' + id, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ readiness_status: 'ready' }) });
    }, productId);
    await page.reload();
    await createOffer(page);
    await expect(page.getByRole('button', { name: 'New offer', exact: true })).toBeVisible();
    await expect(page.locator('[data-new-offer]')).toHaveCount(0);
    await onePrimary(page, 'Connect Stripe');
    await fits(page);
    await frame(page, `after-products-collapsed-${viewport.width}`);
    await page.getByRole('button', { name: 'New offer', exact: true }).focus();
    await page.keyboard.press('Enter');
    const firstField = page.locator('#new-offer select, #new-offer input, #new-offer textarea').first();
    await expect(firstField).toBeFocused();
    await expect(page.getByRole('heading', { name: 'Create a referral offer' })).toBeVisible();
    await page.locator('#new-offer').evaluate(section => section.scrollIntoView({ block: 'start' }));
    await fits(page);
    await frame(page, `after-products-expanded-${viewport.width}`, false, true);
    await page.evaluate(async endpoint => {
      await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ failOffer: true }) });
    }, base + '/__test/mode');
    await page.getByLabel('Commission value', { exact: true }).fill('1');
    await page.getByLabel('Referral destination', { exact: true }).fill('https://rail-tool.example');
    await page.getByRole('button', { name: 'Create offer', exact: true }).click();
    const error = page.locator('#new-offer .field-error');
    await expect(error).toBeFocused();
    await expect(error).toContainText('commission value is below the minimum');
    const box = await error.boundingBox();
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
    await onePrimary(page, 'Connect Stripe');
    await context.close();
  }
});

test('setup and agents, landing, and overview', async ({ browser }) => {
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    reset();
    const productId = randomUUID();
    state.products = [{ id: productId, name: 'Rail Tool', website_url: 'https://rail-tool.example', tagline: 'Help developers ship a referral offer.', docs_url: '', pricing_url: '', primary_category: '', keywords: [], readiness_status: 'ready', readiness_score: 90 }];
    state.offers = [{ id: randomUUID(), product_id: productId, status: 'draft', commission_type: 'percentage', commission_value: 20, commission_currency: 'USD', currency: 'USD', cookie_duration_days: 30, refund_window_days: 30, terms: 'First purchase.' }];
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.goto(base + '/landing');
    await expect(page.getByRole('heading', { name: /You built it/ })).toBeVisible();
    await expect(page.locator('[data-primary-action]')).toHaveCount(1);
    await fits(page);
    await frame(page, `after-landing-${viewport.width}`);
    await page.goto(base + '/owner');
    await expect(page.locator('[data-station="stripe"]')).toHaveAttribute('aria-current', 'step');
    await onePrimary(page, 'Open products ↗');
    await fits(page);
    await frame(page, `after-overview-${viewport.width}`);
    await page.goto(base + '/owner/settings');
    await expect(page.getByRole('heading', { name: 'Setup & agents', level: 1 })).toBeVisible();
    await expect(page.locator('.agent-line pre')).toHaveText(`Read ${base}/SKILL.md and connect this site to Offer Rails`);
    await expect(page.locator('[data-skill-status]')).toContainText('Coming soon');
    await expect(page.locator('[data-rail-plain]')).toHaveText('Your product is registered. Next: connect Stripe test mode.');
    const humans = page.locator('.task-list');
    await expect(humans).toContainText('Connect Stripe in test mode');
    await expect(humans).toContainText('An operator approves the offer before it is listed.');
    await expect(page.getByRole('heading', { name: 'Connect your existing agent' })).toBeVisible();
    await expect(page.locator('.setup-command pre')).toHaveCount(1);
    const copy = page.getByRole('button', { name: 'Copy line', exact: true });
    await expect(copy).toBeEnabled();
    await expect(copy).toHaveClass(/secondary/);
    await expect(copy).not.toHaveAttribute('data-primary-action');
    await copy.click();
    await expect(page.locator('.agent-line .copy-status')).toHaveText('Copied. Paste it to your agent.');
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(`Read ${base}/SKILL.md and connect this site to Offer Rails`);
    await fits(page);
    await frame(page, `after-setup-${viewport.width}`);
    await shotD020(page, `after-setup-${viewport.width}`);
    if (viewport.width === 1440) await frame(page, 'after-setup-1440-page', true);
    await context.close();
  }
  reset();
  const productId = randomUUID();
  state.products = [{ id: productId, name: 'Rail Tool', website_url: 'https://rail-tool.example', tagline: 'Help developers ship a referral offer.', docs_url: '', pricing_url: '', primary_category: '', keywords: [], readiness_status: 'ready', readiness_score: 90 }];
  state.offers = [{ id: randomUUID(), product_id: productId, status: 'draft', commission_type: 'percentage', commission_value: 20, commission_currency: 'USD', currency: 'USD', cookie_duration_days: 30, refund_window_days: 30, terms: 'First purchase.' }];
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, recordVideo: { dir: '/tmp/d019-video', size: { width: 1440, height: 900 } } });
  const page = await context.newPage();
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto(base + '/owner/settings');
  await expect(page.locator('[data-rail-plain]')).toHaveText('Your product is registered. Next: connect Stripe test mode.');
  await page.getByRole('button', { name: 'Copy line', exact: true }).click();
  await expect(page.locator('.agent-line .copy-status')).toContainText('Copied');
  await page.waitForTimeout(4000);
  const video = page.video();
  await context.close();
  const source = await video.path();
  execFileSync('ffmpeg', ['-y', '-i', source, '-filter:v', 'setpts=0.25*PTS', '-an', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '30', path.join(d019, 'after-setup-agents.mp4')], { stdio: 'inherit' });
});

test('workspace nav is reachable at 390 and 360', async ({ browser }) => {
  for (const viewport of [
    { width: 390, height: 844, shot: 'after-nav-390' },
    { width: 360, height: 740, shot: 'after-nav-360' },
    { width: 1440, height: 900, shot: '' },
  ]) {
    reset();
    const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } });
    const page = await context.newPage();
    await page.goto(base + '/owner');
    await expect(page.getByRole('link', { name: 'Open products ↗', exact: true })).toBeVisible();
    await navFits(page);
    if (viewport.shot) await shotD020(page, viewport.shot);
    await context.close();
  }
});

test('a slow health check wakes the workspace and keeps typed input', async ({ browser }) => {
  test.setTimeout(90000);
  fs.mkdirSync('/tmp/d020-video', { recursive: true });
  reset();
  state.healthMode = 'slow';
  state.healthFails = 3;
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, recordVideo: { dir: '/tmp/d020-video', size: { width: 1440, height: 900 } } });
  const page = await context.newPage();
  await page.goto(base + '/owner');
  await expect(page.getByRole('heading', { name: 'Waking the server, about 30 seconds' })).toBeVisible();
  await expect(page.locator('#workspace-feedback')).toHaveText('');
  await shotD020(page, 'after-waking-1440');
  await expect(page.getByRole('link', { name: 'Open products ↗', exact: true })).toBeVisible({ timeout: 25000 });
  await shotD020(page, 'after-recovered-1440');
  const video = page.video();
  await context.close();
  const source = await video.path();
  fs.mkdirSync(d020, { recursive: true });
  execFileSync('ffmpeg', ['-y', '-i', source, '-filter:v', 'setpts=0.25*PTS', '-an', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '30', path.join(d020, 'after-waking-recovered.mp4')], { stdio: 'inherit' });

  reset();
  state.healthMode = 'slow';
  state.healthFails = 3;
  const narrow = await browser.newContext({ viewport: { width: 390, height: 844 }, reducedMotion: 'reduce' });
  const phone = await narrow.newPage();
  await phone.goto(base + '/owner');
  await expect(phone.getByRole('heading', { name: 'Waking the server, about 30 seconds' })).toBeVisible();
  await expect(phone.locator('.wake-bar')).toHaveCSS('transition-duration', '0s');
  await shotD020(phone, 'after-waking-390');
  await expect(phone.getByRole('link', { name: 'Open products ↗', exact: true })).toBeVisible({ timeout: 25000 });
  await shotD020(phone, 'after-recovered-390');
  await narrow.close();

  reset();
  const kept = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const formPage = await kept.newPage();
  await formPage.goto(base + '/owner/products');
  await formPage.getByLabel('Product name', { exact: true }).fill('Kept Name');
  await formPage.getByLabel('Website', { exact: true }).fill('https://rail-tool.example');
  await formPage.getByLabel('What does it help people do?').fill('Help developers ship a referral offer.');
  state.healthMode = 'slow';
  state.healthHits = 0;
  state.healthFails = 2;
  state.failProductCreate = true;
  await formPage.getByRole('button', { name: 'Register product', exact: true }).click();
  await expect(formPage.getByRole('heading', { name: 'Waking the server, about 30 seconds' })).toBeVisible();
  await expect(formPage.getByLabel('Product name', { exact: true })).toHaveValue('Kept Name');
  await expect(formPage.getByRole('heading', { name: 'Kept Name', exact: true })).toBeVisible({ timeout: 25000 });
  await kept.close();
});

test('a server that stays down offers try again', async ({ browser }) => {
  test.setTimeout(180000);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    reset();
    state.healthMode = 'never';
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    await page.goto(base + '/owner');
    await expect(page.getByRole('heading', { name: 'The server did not wake.' })).toBeVisible({ timeout: 90000 });
    await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
    await expect(page.locator('#workspace-app')).not.toContainText('503');
    await expect(page.locator('#workspace-feedback')).toHaveText('');
    await onePrimary(page, 'Try again');
    await shotD020(page, `after-giveup-${viewport.width}`);
    if (viewport.width === 1440) {
      await page.getByRole('button', { name: 'Try again', exact: true }).click();
      await expect(page.getByRole('heading', { name: 'Waking the server, about 30 seconds' })).toBeVisible();
    }
    await context.close();
  }
});

async function shotD023(page, name) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const sidebar = page.locator('aside.sidebar');
  if (await sidebar.count()) {
    const box = await sidebar.boundingBox();
    expect(box.y).toBeLessThanOrEqual(1);
  }
  fs.mkdirSync(d023, { recursive: true });
  await page.screenshot({ path: path.join(d023, name + '.png'), fullPage: false });
}

function watchApi(page, bag) {
  page.on('request', request => bag.push(request.url()));
}

function assertNoApi(bag) {
  const banned = bag.filter(url => {
    const parsed = new URL(url);
    if (parsed.origin !== new URL(base).origin) return true;
    return ['/owner', '/v1', '/health', '/wp-json', '/mcp', '/auth'].some(prefix => parsed.pathname === prefix || parsed.pathname.startsWith(prefix + '/'));
  });
  expect(banned).toEqual([]);
}

test('owner prototype screens at 390 and 1440', async ({ browser }) => {
  fs.mkdirSync(d023, { recursive: true });
  fs.copyFileSync(path.resolve(__dirname, '../../docs/prototype/d023-fields.md'), path.join(d023, 'd023-fields.md'));
  const screens = [
    { path: '/prototype/share', heading: 'What my agent shares', shot: 'after-share' },
    { path: '/prototype/audience', heading: 'Where my audience lives', shot: 'after-audience' },
    { path: '/prototype/payments', heading: 'Who pays whom', shot: 'after-payments' },
  ];
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    const calls = [];
    watchApi(page, calls);
    for (const screen of screens) {
      await page.goto(base + screen.path);
      await expect(page.getByRole('heading', { level: 1, name: screen.heading })).toBeVisible();
      await expect(page.getByText('Example data', { exact: true })).toBeVisible();
      await expect(page.locator('[data-primary-action]')).toHaveCount(1);
      const action = await page.locator('[data-primary-action]').boundingBox();
      expect(action.y).toBeGreaterThanOrEqual(0);
      expect(action.y + action.height).toBeLessThanOrEqual(viewport.height + 1);
      await fits(page);
      await shotD023(page, `${screen.shot}-${viewport.width}`);
    }
    await page.goto(base + '/prototype/share');
    const shareSwitch = page.getByRole('switch', { name: 'Sharing on/off' });
    await expect(shareSwitch).toHaveAttribute('aria-checked', 'true');
    await expect(page.getByText('Being sent', { exact: true })).toHaveCount(2);
    await shareSwitch.click();
    await expect(shareSwitch).toHaveAttribute('aria-checked', 'false');
    await expect(page.getByText('Not being sent', { exact: true })).toHaveCount(2);
    await expect(page.locator('[data-primary-action]')).toHaveCount(1);
    if (viewport.width === 390) await shotD023(page, 'after-sharing-off-390');
    await page.goto(base + '/prototype/audience');
    const card = page.getByRole('region', { name: 'Audience card' });
    await page.getByRole('button', { name: 'Pinterest', exact: true }).click();
    await page.getByLabel('Topic for Pinterest').fill('home offices');
    await page.getByLabel('Size for Pinterest').selectOption('100k+');
    await expect(card).toContainText('Pinterest');
    await expect(card).toContainText('home offices');
    await expect(card).toContainText('100k+');
    await fits(page);
    await expect(card).toContainText('Not being sent');
    await page.getByRole('button', { name: 'Show empty example', exact: true }).click();
    await page.goto(base + '/prototype/share');
    await expect(page.getByRole('region', { name: 'Offer card' })).toContainText('No offer yet');
    await page.goto(base + '/prototype/payments');
    await expect(page.getByText('No sales in this example.')).toBeVisible();
    await expect(page.getByText('We never hold money. Payouts go from seller to publisher.')).toBeVisible();
    assertNoApi(calls);
    await context.close();
  }

  fs.mkdirSync('/tmp/d023-video', { recursive: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, recordVideo: { dir: '/tmp/d023-video', size: { width: 1440, height: 900 } } });
  const page = await context.newPage();
  const calls = [];
  watchApi(page, calls);
  await page.goto(base + '/prototype/share');
  await expect(page.getByRole('heading', { name: 'What my agent shares' })).toBeVisible();
  await page.waitForTimeout(900);
  await page.getByRole('link', { name: 'See where the audience lives', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Where my audience lives' })).toBeVisible();
  await page.waitForTimeout(900);
  await page.getByRole('link', { name: 'See who pays whom', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Who pays whom' })).toBeVisible();
  await page.waitForTimeout(900);
  assertNoApi(calls);
  const video = page.video();
  await context.close();
  const source = await video.path();
  execFileSync('ffmpeg', ['-y', '-i', source, '-filter:v', 'setpts=0.5*PTS', '-an', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '30', path.join(d023, 'after-prototype-walk.mp4')], { stdio: 'inherit' });
});

test('landing still renders at both widths', async ({ browser }) => {
  for (const viewport of [{ width: 1440, height: 900, tag: '1440' }, { width: 390, height: 844, tag: '390' }]) {
    const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } });
    const page = await context.newPage();
    await page.goto(base + '/landing');
    await expect(page.getByRole('heading', { name: /You built it/ })).toBeVisible();
    await fits(page);
    await shot(page, `audit-landing-${viewport.tag}`);
    await context.close();
  }
});
