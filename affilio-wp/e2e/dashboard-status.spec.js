const { test, expect } = require('@playwright/test');
const fs = require('fs');
const path = require('path');

const css = fs.readFileSync(path.join(__dirname, '../admin/css/affilio-admin.css'), 'utf8');
const dashboardJs = path.resolve(__dirname, '../admin/build/dashboard.js');
const onboardingJs = path.resolve(__dirname, '../admin/build/onboarding.js');

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

async function bootOnboarding(page, { locale, connected, copy }) {
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
      phase: 'not_started',
      scanned: 0,
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
