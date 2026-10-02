const { test, expect } = require('@playwright/test');

async function loginAsAdmin(page) {
  const user = process.env.WP_ADMIN_USER;
  const pass = process.env.WP_ADMIN_PASS;
  test.skip(!user || !pass, 'WP_ADMIN_USER and WP_ADMIN_PASS are required');

  await page.goto('/wp-login.php');
  await expect(page.locator('#user_login')).toBeFocused();
  await page.fill('#user_login', user);
  await page.fill('#user_pass', pass);
  await page.click('#wp-submit');
  await expect(page).toHaveURL(/wp-admin/);
}

test.describe('Affilio WP Admin Flows', () => {
  test.describe.configure({ mode: 'serial' });

  test('connects a fresh installation and scans without copying a key', async ({ page }) => {
    test.skip(process.env.AFFILIO_E2E_SETUP !== '1', 'Run only on an isolated fresh WordPress fixture');
    await loginAsAdmin(page);
    await page.goto('/wp-admin/admin.php?page=affilio-settings');
    const connect = page.getByRole('button', { name: /Connect and find opportunities/i });
    await expect(connect).toBeEnabled();
    await connect.click();
    await expect(page.locator('[data-progress]')).toContainText(/Scanning in the background/i);
    if (process.env.AFFILIO_E2E_CRON_COMPOSE) {
      const { execFileSync } = require('node:child_process');
      execFileSync('docker', ['compose', '-p', 'affilio-onboarding-qa', '-f', process.env.AFFILIO_E2E_CRON_COMPOSE, 'run', '--rm', 'cli', 'wp', 'cron', 'event', 'run', 'affilio_setup_scan'], { stdio: 'pipe' });
    }
    await expect(page.locator('[data-progress]')).toContainText(/published posts analyzed/i, { timeout: 45_000 });
    await expect(page.getByRole('link', { name: /Review opportunities/i })).toBeVisible();
    await expect(page.locator('[data-error]')).toBeEmpty();
    await page.getByRole('link', { name: /Review opportunities/i }).click();
    await expect(page.locator('#affilio-dashboard-root')).toContainText(/Best developer hosting tools/i);
  });

  test('shows onboarding/settings state', async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/wp-admin/admin.php?page=affilio-settings');

    await expect(page.locator('h1')).toContainText('Affilio Settings');
    await expect(page.locator('.affilio-status-pill')).toBeVisible();
    await expect(page.locator('#affilio-onboarding-root')).toBeVisible();
    await page.locator('summary').click();
    await expect(page.locator('input[name="affilio_api_key"]')).toBeVisible();
  });

  test('disconnected flow shows banner on dashboard', async ({ page }) => {
    await loginAsAdmin(page);

    await page.goto('/wp-admin/admin.php?page=affilio-settings');
    await page.locator('summary').click();
    await page.fill('input[name="affilio_api_key"]', '');
    await page.getByRole('button', { name: /Save Settings/i }).click();

    await page.goto('/wp-admin/admin.php?page=affilio');
    await expect(page.getByText(/Not connected\./i).first()).toBeVisible();
  });

  test('connected flow renders dashboard', async ({ page }) => {
    const apiKey = process.env.AFFILIO_E2E_API_KEY;
    test.skip(!apiKey, 'AFFILIO_E2E_API_KEY is required for connected-flow test');

    await loginAsAdmin(page);

    await page.goto('/wp-admin/admin.php?page=affilio-settings');
    await page.locator('summary').click();
    await page.fill('input[name="affilio_api_key"]', apiKey);
    await page.getByRole('button', { name: /Save Settings/i }).click();

    await page.goto('/wp-admin/admin.php?page=affilio');
    await expect(page.locator('#affilio-dashboard-root')).toBeVisible();
    await expect(page.getByText(/Not connected/i)).toHaveCount(0);
  });
});
