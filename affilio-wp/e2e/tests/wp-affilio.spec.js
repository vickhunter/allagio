const { test, expect } = require('@playwright/test');

async function loginAsAdmin(page) {
  const user = process.env.WP_ADMIN_USER;
  const pass = process.env.WP_ADMIN_PASS;
  test.skip(!user || !pass, 'WP_ADMIN_USER and WP_ADMIN_PASS are required');

  await page.goto('/wp-login.php');
  await page.fill('#user_login', user);
  await page.fill('#user_pass', pass);
  await page.click('#wp-submit');
  await expect(page).toHaveURL(/wp-admin/);
}

test.describe('Affilio WP Admin Flows', () => {
  test('shows onboarding/settings state', async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/wp-admin/admin.php?page=affilio-settings');

    await expect(page.locator('h1')).toContainText('Affilio Settings');
    await expect(page.getByText('Status:')).toBeVisible();
    await expect(page.locator('input[name="affilio_api_key"]')).toBeVisible();
    await expect(page.getByRole('link', { name: /Create Affilio account/i })).toBeVisible();
  });

  test('disconnected flow shows banner on dashboard', async ({ page }) => {
    await loginAsAdmin(page);

    await page.goto('/wp-admin/admin.php?page=affilio-settings');
    await page.fill('input[name="affilio_api_key"]', '');
    await page.getByRole('button', { name: /Save Settings/i }).click();

    await page.goto('/wp-admin/admin.php?page=affilio');
    await expect(page.getByText(/Affilio is not connected/i)).toBeVisible();
  });

  test('connected flow renders dashboard', async ({ page }) => {
    const apiKey = process.env.AFFILIO_E2E_API_KEY;
    test.skip(!apiKey, 'AFFILIO_E2E_API_KEY is required for connected-flow test');

    await loginAsAdmin(page);

    await page.goto('/wp-admin/admin.php?page=affilio-settings');
    await page.fill('input[name="affilio_api_key"]', apiKey);
    await page.getByRole('button', { name: /Save Settings/i }).click();

    await page.goto('/wp-admin/admin.php?page=affilio');
    await expect(page.locator('#affilio-dashboard-root')).toBeVisible();
    await expect(page.getByText(/Not connected/i)).toHaveCount(0);
  });
});
