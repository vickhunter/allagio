const { test, expect } = require('@playwright/test');
const fs = require('fs');
const path = require('path');

test('Vedi il prodotto is at least 44px tall at 390px width', async ({ page }) => {
  const css = fs.readFileSync(path.join(__dirname, '../../public/css/placement.css'), 'utf8');
  await page.setViewportSize({ width: 390, height: 844 });
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style>
    <p class="affilio-placement-abc"><a class="affilio-product-link" href="/go/abc" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a></p>`);
  const box = await page.locator('a.affilio-product-link').boundingBox();
  expect(box).not.toBeNull();
  expect(box.height).toBeGreaterThanOrEqual(44);
});
