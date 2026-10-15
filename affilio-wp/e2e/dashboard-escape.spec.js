const { test, expect } = require('@playwright/test');
const path = require('path');

test('disconnected dashboard message renders as text', async ({ page }) => {
  const message = '<img src=x onerror="alert(1)">';
  const settings = 'https://blog.example/wp-admin/admin.php?page=affilio-settings&x="><img src=y>';
  await page.setContent('<!doctype html><div id="affilio-dashboard-root"></div>');
  await page.evaluate(({ message, settings }) => {
    window.AffilioData = {
      locale: 'en',
      nonce: 'n',
      restBase: '/wp-json/affilio/v1',
      settingsUrl: settings,
      connectionStatus: { connected: false, message },
    };
  }, { message, settings });
  await page.addScriptTag({ path: path.resolve(__dirname, '../admin/build/dashboard.js') });

  const root = page.locator('#affilio-dashboard-root');
  await expect(root.locator('img, script')).toHaveCount(0);
  await expect(root).toContainText(message);
  await expect(root.getByRole('link', { name: 'Open settings' })).toHaveAttribute('href', settings);
});
