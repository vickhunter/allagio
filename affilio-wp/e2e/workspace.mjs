import { chromium, expect } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';

let input = '';
for await (const chunk of process.stdin) input += chunk;
const fixture = JSON.parse(input);
const browser = await chromium.launch({ headless: true, args: ['--enable-unsafe-swiftshader'] });
const errors = [];
const contexts = [];
async function owner(cookie) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
    contexts.push(context);
    await context.addCookies([{ name: cookie.Name, value: cookie.Value, url: fixture.base, httpOnly: true, sameSite: 'Lax' }]);
    const page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    return page;
}
async function screenshot(page, name) {
    if (!fixture.screenshots) return;
    await mkdir(fixture.screenshots, { recursive: true });
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: join(fixture.screenshots, name + '.png'), fullPage: true });
}
try {
    const builder = await owner(fixture.builder);
    await builder.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await builder.goto(fixture.base + '/');
    await expect(builder.getByRole('heading', { name: 'You built it. Give it a distribution channel.' })).toBeVisible();
    if (await builder.locator('#install').count()) {
        await builder.getByRole('button', { name: 'Copy command', exact: true }).click();
        await expect(builder.locator('.copy-status')).toContainText('Copied.');
        expect(await builder.evaluate(() => navigator.clipboard.readText())).toBe(await builder.locator('#install').inputValue());
        await builder.evaluate(() => window.scrollTo(0, 0));
    }
    await expect(builder.locator('.world-stage')).toHaveClass(/scene-ready/);
    await builder.getByRole('button', { name: 'Pause illustration', exact: true }).click();
    await expect(builder.getByRole('button', { name: 'Play illustration', exact: true })).toBeVisible();
    if (fixture.screenshots) {
        await mkdir(fixture.screenshots, { recursive: true });
        await builder.screenshot({ path: join(fixture.screenshots, 'hero.png') });
    }
    await screenshot(builder, 'home');
    await expect(builder.locator('.world-stage')).toHaveClass(/scene-ready/);
    await builder.emulateMedia({ reducedMotion: 'reduce' });
    await expect(builder.locator('.world-stage canvas')).toHaveCount(0);
    await expect(builder.getByRole('button', { name: /illustration$/ })).toBeHidden();
    await expect(builder.getByRole('link', { name: 'Get customers ↗', exact: true })).toBeVisible();
    await builder.setViewportSize({ width: 390, height: 844 });
    expect(await builder.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await screenshot(builder, 'home-mobile');
    await builder.setViewportSize({ width: 1440, height: 1000 });
    await builder.emulateMedia({ reducedMotion: 'no-preference' });
    await expect(builder.getByRole('button', { name: 'Play illustration', exact: true })).toBeVisible();
    const fallbackContext = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
    contexts.push(fallbackContext);
    const fallbackPage = await fallbackContext.newPage();
    await fallbackPage.goto(fixture.base + '/');
    await expect(fallbackPage.getByRole('link', { name: 'Get customers ↗', exact: true })).toBeVisible();
    await expect(fallbackPage.locator('.world-fallback')).toBeVisible();
    expect(await fallbackPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const noGPUContext = await browser.newContext();
    contexts.push(noGPUContext);
    await noGPUContext.addInitScript(() => {
        const original = HTMLCanvasElement.prototype.getContext;
        HTMLCanvasElement.prototype.getContext = function(type, ...args) {
            return type === 'webgl2' ? null : original.call(this, type, ...args);
        };
    });
    const noGPUPage = await noGPUContext.newPage();
    noGPUPage.on('pageerror', error => errors.push(error.message));
    await Promise.all([
        noGPUPage.waitForResponse(response => response.url().endsWith('/landing-scene.bundle.js')),
        noGPUPage.goto(fixture.base + '/'),
    ]);
    await expect(noGPUPage.locator('.world-stage canvas')).toHaveCount(0);
    await expect(noGPUPage.locator('.world-fallback')).toBeVisible();
    await expect(noGPUPage.getByRole('link', { name: 'Get customers ↗', exact: true })).toBeVisible();
    await builder.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await builder.goto(fixture.base + '/owner/settings');
    const command = await builder.locator('.setup-command pre').textContent();
    await builder.getByRole('button', { name: 'Copy command', exact: true }).click();
    await expect(builder.locator('.setup-command .copy-status')).toContainText('Copied.');
    expect(await builder.evaluate(() => navigator.clipboard.readText())).toBe(command);
    await builder.context().clearPermissions();
    await builder.addInitScript(() => {
        Object.defineProperty(navigator, 'clipboard', { value: { writeText: async () => { throw new Error('Clipboard unavailable'); } } });
    });
    await builder.reload();
    await builder.getByRole('button', { name: 'Copy command', exact: true }).click();
    await expect(builder.locator('.setup-command .copy-status')).toContainText('Select and copy');
    expect(await builder.evaluate(() => window.getSelection().toString())).toBe(command);
    await builder.goto(fixture.base + '/owner/products');
    await builder.getByLabel('Product name', { exact: true }).fill('Browser Tool <script>');
    await builder.getByLabel('Website', { exact: true }).fill('https://browser-tool.example');
    await builder.getByLabel('What does it help people do?').fill('Help developers understand their databases.');
    await builder.getByLabel('Topics, separated by commas').fill('postgres, databases');
    await builder.getByRole('button', { name: 'Register product', exact: true }).click();
    await expect(builder.getByRole('heading', { name: 'Browser Tool <script>', exact: true })).toBeVisible();
    await builder.getByRole('button', { name: 'Edit', exact: true }).click();
    await builder.getByLabel('Product name', { exact: true }).fill('Browser Tool');
    await builder.getByRole('button', { name: 'Save changes', exact: true }).click();
    await expect(builder.getByRole('heading', { name: 'Browser Tool', exact: true })).toBeVisible();
    await builder.getByLabel('Commission value', { exact: true }).fill('20');
    await builder.getByLabel('Referral destination', { exact: true }).fill('https://browser-tool.example');
    await builder.getByRole('button', { name: 'Create offer', exact: true }).click();
    await expect(builder.getByRole('button', { name: 'Submit for review', exact: true })).toBeVisible();
    await builder.getByRole('button', { name: 'Submit for review', exact: true }).click();
    await expect(builder.locator('#workspace-feedback')).toContainText('product has not passed readiness assessment');
    await screenshot(builder, 'products');
    await builder.goto(fixture.base + '/owner/review');
    await expect(builder.getByRole('heading', { name: 'Prepare commission payouts' })).toBeVisible();
    await builder.goto(fixture.base + '/owner/earnings');
    await builder.getByLabel('Payment reference').fill('browser-check-reference');
    await builder.getByLabel('I have sent this payment outside Distributoor.').check();
    await builder.getByRole('button', { name: 'Report payment sent' }).click();
    await expect(builder.locator('#workspace-feedback')).toContainText('publisher must confirm receipt separately');
    const publisher = await owner(fixture.publisher);
    await publisher.goto(fixture.base + '/owner/settings');
    await publisher.getByLabel('Payment contact instructions').fill('Contact https://publisher.example.com/contact');
    await publisher.getByRole('button', { name: 'Save instructions' }).click();
    await expect(publisher.locator('#workspace-feedback')).toContainText('Payment instructions saved');
    await publisher.goto(fixture.base + '/owner/placements');
    await publisher.getByLabel('Published article URL').fill('https://publisher.example.com/tutorial');
    await publisher.getByLabel('What is this article about?').fill('InboxPilot email inbox productivity tutorial for developers.');
    await publisher.getByRole('button', { name: 'Find opportunities' }).click();
    await expect(publisher.getByRole('heading', { name: 'InboxPilot' })).toBeVisible();
    await publisher.getByLabel('Recommendation copy').fill('Consider InboxPilot for organizing an email inbox.');
    await publisher.getByRole('button', { name: 'Prepare for owner review' }).click();
    await expect(publisher.getByRole('heading', { name: 'Review this placement' })).toBeVisible();
    await expect(publisher.getByText('excluding tax and shipping', { exact: false })).toBeVisible();
    const approvalResponse = publisher.waitForResponse(response => response.url().endsWith('/approve'));
    await publisher.getByRole('button', { name: 'Approve this exact placement' }).click();
    const approved = await approvalResponse;
    expect(approved.status(), approved.status() === 303 ? '' : await approved.text()).toBe(303);
    await publisher.getByRole('button', { name: 'Create tracked link' }).click();
    await expect(publisher.getByRole('heading', { name: 'Your tracked link' })).toBeVisible();
    await screenshot(publisher, 'placement');
    await publisher.goto(fixture.base + '/owner/earnings');
    await publisher.getByRole('button', { name: 'Confirm funds arrived' }).click();
    await publisher.waitForURL(fixture.base + '/owner');
    await publisher.goto(fixture.base + '/owner/earnings');
    await expect(publisher.getByText('You confirmed receipt.', { exact: true })).toBeVisible();
    await screenshot(publisher, 'earnings');
    await publisher.setViewportSize({ width: 390, height: 844 });
    await publisher.goto(fixture.base + '/owner/placements');
    await expect(publisher.getByRole('button', { name: 'Find opportunities' })).toBeVisible();
    expect(await publisher.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await screenshot(publisher, 'mobile');
    expect(errors).toEqual([]);
    console.log('Workspace browser integration passed: products, offers, placement approval, tracked link, payment reporting and receipt.');
} finally {
    await Promise.all(contexts.map(context => context.close()));
    await browser.close();
}
