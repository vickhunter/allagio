const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: '.',
  testMatch: ['offer-rail.spec.js', 'owner-walk.spec.js', 'landing-a11y.spec.js', 'dashboard-escape.spec.js', 'dashboard-status.spec.js'],
  timeout: 120000,
  workers: 1,
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: { headless: true },
});
