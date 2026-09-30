const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests',
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  use: {
    baseURL: process.env.WP_BASE_URL || 'http://localhost:8080',
    headless: true,
    launchOptions: { args: process.env.AFFILIO_E2E_HOSTMAP ? ['--host-resolver-rules=' + process.env.AFFILIO_E2E_HOSTMAP, '--no-proxy-server'] : [] },
    trace: 'on-first-retry',
  },
  retries: 1,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
});
