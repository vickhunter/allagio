const { test, expect } = require('@playwright/test');
const fs = require('fs');
const path = require('path');

const css = fs.readFileSync(path.join(__dirname, '../admin/css/affilio-admin.css'), 'utf8');
const dashboardJs = path.resolve(__dirname, '../admin/build/dashboard.js');
const onboardingJs = path.resolve(__dirname, '../admin/build/onboarding.js');

// Core admin buttons.css at max-width 782px. Loaded after the plugin sheet, as in wp-admin.
const wpAdminButtons = `
.wp-core-ui .button, .wp-core-ui .button-primary, .wp-core-ui .button-secondary {
  display: inline-block;
  font-size: 13px;
  line-height: 2.15384615;
  min-height: 30px;
  margin: 0;
  padding: 0 10px;
  border-width: 1px;
  border-style: solid;
  box-sizing: border-box;
}
.wp-core-ui .button.button-large {
  min-height: 40px;
  line-height: 38px;
  font-size: 14px;
  padding: 0 12px;
}
@media screen and (max-width: 782px) {
  .wp-core-ui .button,
  .wp-core-ui .button.button-large,
  .wp-core-ui .button.button-small {
    padding: 0 14px;
    line-height: 2.71428571;
    font-size: 14px;
    vertical-align: middle;
    min-height: 40px;
    margin-bottom: 4px;
  }
}`;

async function noHorizontalOverflow(page) {
  const box = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    client: document.documentElement.clientWidth,
  }));
  expect(box.scroll).toBeLessThanOrEqual(box.client);
}

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
});

async function bootOnboarding(page, { locale, connected, copy, phase = 'not_started', scanned = 0, summary = '' }) {
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><div class="wrap affilio-admin-page"><div id="affilio-onboarding-root"></div></div>`);
  await page.evaluate((setup) => {
    window.AffilioSetup = setup;
  }, {
    locale,
    restBase: 'https://blog.example/wp-json/affilio/v1',
    nonce: 'n',
    serviceUrl: 'https://api.example.test',
    copy,
  });
  await page.route('**/affilio/v1/setup', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      phase,
      scanned,
      summary,
      skipped: [],
      connection: { connected },
      dashboard_url: 'https://blog.example/wp-admin/admin.php?page=affilio',
    }),
  }));
  await page.addScriptTag({ path: onboardingJs });
}

async function bootDashboard(page, data, routes) {
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><div class="wrap affilio-admin-page"><div id="affilio-dashboard-root"></div></div>`);
  await page.evaluate((value) => {
    window.AffilioData = value;
  }, data);
  await page.route('**/affilio/v1/proxy', async (route) => {
    const body = route.request().postDataJSON();
    const payload = (routes && body && routes[body.path]) || {};
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(payload),
    });
  });
  await page.addScriptTag({ path: dashboardJs });
}

test('connected settings offer start analysis', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'en',
    connected: true,
    copy: { startAnalysis: 'Start analysis', connectedStatus: 'Your site is connected.' },
  });
  const button = page.getByRole('button', { name: 'Start analysis', exact: true });
  await expect(button).toBeEnabled();
  await expect(page.locator('[data-progress]')).toHaveText('Your site is connected.');
  await expect(page.getByRole('button', { name: 'Connect and find opportunities' })).toHaveCount(0);
  const box = await button.boundingBox();
  expect(box.height).toBeGreaterThanOrEqual(44);
  await button.focus();
  const outline = await button.evaluate((el) => getComputedStyle(el).outlineStyle);
  expect(outline).not.toBe('none');
  await noHorizontalOverflow(page);
});

test('disconnected settings keep the connect prompt', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'en',
    connected: false,
    copy: { startAnalysis: 'Start analysis', connectedStatus: 'Your site is connected.' },
  });
  const button = page.getByRole('button', { name: 'Connect and find opportunities', exact: true });
  await expect(button).toBeEnabled();
  await expect(page.locator('[data-progress]')).toHaveText('Ready to connect. No API key to copy.');
  await expect(page.getByRole('button', { name: 'Start analysis' })).toHaveCount(0);
  await expect(page.locator('[data-progress]')).not.toContainText('Your site is connected.');
  const box = await button.boundingBox();
  expect(box.height).toBeGreaterThanOrEqual(44);
  await noHorizontalOverflow(page);
});

test('italian connected settings use the catalog strings', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'it',
    connected: true,
    copy: { startAnalysis: 'Avvia analisi', connectedStatus: 'Il sito è collegato.' },
  });
  await expect(page.getByRole('button', { name: 'Avvia analisi', exact: true })).toBeVisible();
  await expect(page.locator('[data-progress]')).toHaveText('Il sito è collegato.');
  await expect(page.getByRole('button', { name: 'Collega e trova opportunità' })).toHaveCount(0);
  await expect(page.locator('[data-progress]')).not.toContainText('Avvia la prima analisi');
  await noHorizontalOverflow(page);
});

test('a running scan replaces the start label', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'it',
    connected: true,
    phase: 'scanning',
    scanned: 2,
    copy: {
      startAnalysis: 'Avvia analisi',
      connectedStatus: 'Il sito è collegato.',
      analysisInProgress: 'Analisi in corso',
      analysisComplete: 'Analisi completata',
    },
  });
  const button = page.getByRole('button', { name: 'Analisi in corso', exact: true });
  await expect(button).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Avvia analisi' })).toHaveCount(0);
  await noHorizontalOverflow(page);
});

test('a finished scan replaces the start label', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'it',
    connected: true,
    phase: 'complete',
    scanned: 4,
    summary: 'Analisi completata: 4 articoli analizzati',
    copy: {
      startAnalysis: 'Avvia analisi',
      connectedStatus: 'Il sito è collegato.',
      analysisInProgress: 'Analisi in corso',
      analysisComplete: 'Analisi completata',
    },
  });
  const button = page.getByRole('button', { name: 'Analisi completata', exact: true });
  await expect(button).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Avvia analisi' })).toHaveCount(0);
  await expect(page.locator('[data-progress]')).toHaveText('Analisi completata: 4 articoli analizzati');
  await noHorizontalOverflow(page);
});

test('starting analysis from a connected site switches to the in-progress label', async ({ page }) => {
  await bootOnboarding(page, {
    locale: 'en',
    connected: true,
    copy: {
      startAnalysis: 'Start analysis',
      connectedStatus: 'Your site is connected.',
      analysisInProgress: 'Analysis in progress',
      analysisComplete: 'Analysis complete',
    },
  });
  const button = page.getByRole('button', { name: 'Start analysis', exact: true });
  await expect(button).toBeEnabled();
  await page.unroute('**/affilio/v1/setup');
  await page.route('**/affilio/v1/setup', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      phase: 'scanning',
      scanned: 0,
      skipped: [],
      connection: { connected: true },
      dashboard_url: 'https://blog.example/wp-admin/admin.php?page=affilio',
    }),
  }));
  await button.click();
  await expect(page.getByRole('button', { name: 'Analysis in progress', exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Start analysis' })).toHaveCount(0);
});

test('connected dashboard shows the settings badge', async ({ page }) => {
  await bootDashboard(page, {
    locale: 'en',
    nonce: 'n',
    restBase: 'https://blog.example/wp-json/affilio/v1',
    settingsUrl: 'https://blog.example/wp-admin/admin.php?page=affilio-settings',
    connectionStatus: { connected: true, state: 'connected' },
    i18n: {
      connected: 'Connected',
      notConnected: 'Not connected.',
      connectSite: 'Connect your site in Settings.',
    },
  }, {
    '/articles': { articles: [] },
    '/user-programs': { programs: [] },
    '/recommendations': { recommendations: [] },
  });
  const badge = page.locator('.affilio-status-pill.affilio-status-connected');
  await expect(badge).toHaveText('Connected');
  await expect(page.locator('.affilio-next-step')).toHaveCount(0);
  await noHorizontalOverflow(page);
});

test('disconnected dashboard names the settings step', async ({ page }) => {
  const settings = 'https://blog.example/wp-admin/admin.php?page=affilio-settings';
  await bootDashboard(page, {
    locale: 'en',
    nonce: 'n',
    restBase: 'https://blog.example/wp-json/affilio/v1',
    settingsUrl: settings,
    connectionStatus: { connected: false, state: 'missing_key', message: 'Configure your API key.' },
    i18n: {
      connected: 'Connected',
      notConnected: 'Not connected.',
      connectSite: 'Connect your site in Settings.',
    },
  });
  const link = page.getByRole('link', { name: 'Connect your site in Settings.' });
  await expect(page.locator('.affilio-status-message')).toContainText('Not connected.');
  await expect(link).toHaveAttribute('href', settings);
  await expect(page.locator('.affilio-status-pill')).toHaveCount(0);
  const box = await link.boundingBox();
  expect(box.height).toBeGreaterThanOrEqual(44);
  const open = page.getByRole('link', { name: 'Open settings' });
  const openBox = await open.boundingBox();
  expect(openBox.height).toBeGreaterThanOrEqual(44);
  await link.focus();
  const outline = await link.evaluate((el) => getComputedStyle(el).outlineStyle);
  expect(outline).not.toBe('none');
  await noHorizontalOverflow(page);
});

test('italian dashboard status uses the catalog', async ({ page }) => {
  await bootDashboard(page, {
    locale: 'it',
    nonce: 'n',
    restBase: 'https://blog.example/wp-json/affilio/v1',
    settingsUrl: 'https://blog.example/wp-admin/admin.php?page=affilio-settings',
    connectionStatus: { connected: true, state: 'connected' },
    i18n: {
      connected: 'Connesso',
      notConnected: 'Non connesso.',
      connectSite: 'Collega il sito in Impostazioni.',
    },
  }, {
    '/articles': { articles: [] },
    '/user-programs': { programs: [] },
    '/recommendations': { recommendations: [] },
  });
  await expect(page.locator('.affilio-status-pill.affilio-status-connected')).toHaveText('Connesso');
  await noHorizontalOverflow(page);
});

test('dashboard text fields are escaped', async ({ page }) => {
  let dialogs = 0;
  page.on('dialog', async (dialog) => {
    dialogs += 1;
    await dialog.dismiss();
  });
  await bootDashboard(page, {
    locale: 'en',
    nonce: 'n',
    restBase: 'https://blog.example/wp-json/affilio/v1',
    settingsUrl: 'https://blog.example/wp-admin/admin.php?page=affilio-settings',
    connectionStatus: { connected: true, state: 'connected' },
    i18n: { connected: 'Connected', notConnected: 'Not connected.', connectSite: 'Connect your site in Settings.' },
  }, {
    '/articles': {
      articles: [{
        id: 'id"><img src=z>',
        title: '<img src=x onerror=alert(1)>',
        wp_post_id: '9',
        monetization_score: 80,
      }],
    },
    '/user-programs': {
      programs: [{ name: '<svg/onload=alert(1)>', domain: 'evil<img>.test' }],
    },
    '/recommendations': { recommendations: [] },
  });
  const root = page.locator('#affilio-dashboard-root');
  await expect(root.locator('img, svg, script')).toHaveCount(0);
  const html = await root.innerHTML();
  expect(html).not.toContain('<img');
  expect(html).not.toContain('<svg');
  await expect(root).toContainText('<img src=x onerror=alert(1)>');
  await expect(root).toContainText('<svg/onload=alert(1)>');
  await expect(root).toContainText('evil<img>.test');
  expect(dialogs).toBe(0);
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.getByRole('button', { name: 'Review now', exact: true }).click();
  await expect(page.locator('tr.affilio-row-focus')).toHaveCount(1);
  expect(errors).toEqual([]);
  await noHorizontalOverflow(page);
});

test('the dashboard scan button is at least 44px at 390 and 1440', async ({ page }) => {
  const html = `<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><style>${wpAdminButtons}</style><body class="wp-admin wp-core-ui"><div class="wrap affilio-admin-page"><form id="affilio-scan-all" method="post" action="https://blog.example/wp-admin/admin-post.php"><p data-scan-status role="status"></p><button type="submit" class="button button-primary button-large" data-scan>Scansiona tutti gli articoli</button></form></div></body>`;
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.setContent(html);
    const box = await page.getByRole('button', { name: 'Scansiona tutti gli articoli', exact: true }).boundingBox();
    expect(box.height).toBeGreaterThanOrEqual(44);
    await noHorizontalOverflow(page);
  }
});

test('the dashboard scan stays on the page and shows progress', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><div class="wrap affilio-admin-page"><div id="affilio-dashboard-root"></div><form id="affilio-scan-all" method="post" action="https://blog.example/wp-admin/admin-post.php?affilio_scan_continue=1"><p data-scan-status role="status"></p><button type="submit" class="button button-primary button-large" data-scan>Scansiona tutti gli articoli</button></form></div>`);
  await page.evaluate(() => {
    window.AffilioData = {
      locale: 'it',
      nonce: 'n',
      restBase: 'https://blog.example/wp-json/affilio/v1',
      settingsUrl: 'https://blog.example/wp-admin/admin.php?page=affilio-settings',
      connectionStatus: { connected: true, state: 'connected' },
      i18n: {
        connected: 'Connesso',
        notConnected: 'Non connesso.',
        connectSite: 'Collega il sito in Impostazioni.',
        analysisInProgress: 'Analisi in corso',
        analysisComplete: 'Analisi completata',
      },
    };
  });
  let releaseScan;
  const scanHeld = new Promise((resolve) => {
    releaseScan = resolve;
  });
  await page.route('**/affilio/v1/setup', async (route) => {
    const scanning = route.request().method() === 'POST';
    if (!scanning) await scanHeld;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        phase: scanning ? 'scanning' : 'complete',
        scanned: scanning ? 1 : 3,
        connection: { connected: true },
      }),
    });
  });
  await page.route('**/affilio/v1/proxy', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ articles: [], programs: [], recommendations: [] }),
  }));
  let continued = false;
  page.on('request', (request) => {
    if (request.url().includes('admin-post.php') || request.url().includes('affilio_scan_continue')) continued = true;
  });
  await page.addScriptTag({ path: dashboardJs });
  await page.getByRole('button', { name: 'Scansiona tutti gli articoli', exact: true }).click();
  await expect(page.locator('[data-scan-status]')).toHaveText('Analisi in corso');
  releaseScan();
  await expect(page.locator('[data-scan-status]')).toHaveText('Analisi completata');
  await expect(page.getByRole('button', { name: 'Analisi completata', exact: true })).toBeDisabled();
  expect(page.url()).not.toContain('affilio_scan_continue');
  expect(continued).toBe(false);
});

test('settings and dashboard buttons are at least 44px at 390px', async ({ page }) => {
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><style>${wpAdminButtons}</style><body class="wp-admin wp-core-ui"><p><button type="button" class="button button-primary">Posts</button></p><div class="wrap affilio-admin-page"><div id="affilio-onboarding-root"><button type="button" class="button button-primary">Collega e trova opportunità</button><button type="button" class="button button-primary button-large">Avvia analisi</button></div><form class="affilio-settings-form"><p class="submit"><input type="submit" class="button button-primary button-large" value="Salva impostazioni"></p></form><div id="affilio-dashboard-root"><button type="button" class="button button-primary">Rivedi ora</button><button type="button" class="button button-secondary">Aggiungi</button></div></div></body>`);
  for (const name of ['Collega e trova opportunità', 'Avvia analisi', 'Salva impostazioni', 'Rivedi ora', 'Aggiungi']) {
    const box = await page.getByRole('button', { name, exact: true }).boundingBox();
    expect(box.height).toBeGreaterThanOrEqual(44);
  }
  const outside = await page.getByRole('button', { name: 'Posts', exact: true }).boundingBox();
  expect(outside.height).toBeLessThan(44);
  await noHorizontalOverflow(page);
});

test('server-rendered dashboard status is not duplicated', async ({ page }) => {
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><div class="wrap affilio-admin-page"><div id="affilio-dashboard-connection" class="affilio-status-row affilio-dashboard-connection" data-connection-status><span class="affilio-status-pill affilio-status-connected">Connected</span></div><div id="affilio-dashboard-root"></div></div>`);
  await page.evaluate(() => {
    window.AffilioData = {
      locale: 'en',
      nonce: 'n',
      restBase: 'https://blog.example/wp-json/affilio/v1',
      settingsUrl: 'https://blog.example/wp-admin/admin.php?page=affilio-settings',
      connectionStatus: { connected: true, state: 'connected' },
      i18n: { connected: 'Connected', notConnected: 'Not connected.', connectSite: 'Connect your site in Settings.' },
    };
  });
  await page.route('**/affilio/v1/proxy', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: '{}',
  }));
  await page.addScriptTag({ path: dashboardJs });
  await expect(page.locator('.affilio-status-pill')).toHaveCount(1);
  await expect(page.locator('#affilio-dashboard-connection')).toHaveText('Connected');
});
