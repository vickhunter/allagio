import { chromium, expect } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';

let input = '';
for await (const chunk of process.stdin) input += chunk;
const fixture = JSON.parse(input);
const browser = await chromium.launch({ headless: true, args: ['--enable-unsafe-swiftshader'] });
const errors = [];
const contexts = [];
const desktop = { width: 1440, height: 900 };
const mobile = { width: 390, height: 844 };
// Current approved home heading. The retired "Someone has the product" line is not on this page.
const approvedH1 = 'Offers, ready for your agent.';

async function owner(cookie, viewport = desktop) {
    const context = await browser.newContext({ viewport });
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
    await page.screenshot({ path: join(fixture.screenshots, name + '.png') });
}
async function noHorizontalOverflow(page) {
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
}
async function assertApprovedHeading(page) {
    await expect(page.getByRole('heading', { level: 1, name: approvedH1, exact: true })).toBeVisible();
}
async function assertHydratedHome(page) {
    await assertApprovedHeading(page);
    await expect(page.getByRole('navigation', { name: 'Main' })).toBeVisible();
    for (const name of ['Your limits', 'Makers', 'Publishers', 'Protocol', 'Agents']) {
        await expect(page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name, exact: true })).toBeVisible();
    }
    await expect(page.getByRole('link', { name: 'Sign in', exact: true }).first()).toBeVisible();
    await expect(page.locator('[data-hero-bg="fallback"]')).toBeVisible();
    await expect(page.locator('#vision')).toBeVisible();
    await expect(page.getByRole('region', { name: 'What your agent does' })).toBeVisible();
    await expect(page.getByRole('region', { name: 'What you keep control of' })).toBeVisible();
    await expect(page.getByRole('region', { name: 'What it replaces' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'See it on a real post.', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Straight answers.', exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'llms.txt', exact: true })).toHaveCount(1);
    await expect(page.locator('canvas')).toHaveCount(0);
}
const joinLine = 'Join with POST /v1/agent-tokens and send the bearer.';
async function copyJoinLine(page) {
    const prompt = page.locator('[data-join-prompt]');
    await prompt.getByRole('button', { name: 'Copy setup prompt', exact: true }).click();
    await expect(prompt.getByRole('status')).toHaveText('Copied.');
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(joinLine);
}
async function revealGoogleSignIn(page) {
    const link = page.getByRole('link', { name: 'Sign in with Google', exact: true });
    await expect(link).toBeVisible();
}
async function assertStaticHome(page) {
    await assertApprovedHeading(page);
    await expect(page.getByRole('link', { name: 'Sign in with Google', exact: true }).first()).toBeVisible();
    await expect(page.locator('.hero-fallback').first()).toBeVisible();
    await expect(page.locator('#how')).toBeVisible();
    await expect(page.locator('#publishers')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'You send the bearer yourself.', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'The offer stays inside your limits.', exact: true })).toBeVisible();
}
async function assertWorkspaceChrome(page) {
    const nav = page.getByRole('navigation', { name: 'Workspace' });
    await expect(nav).toBeVisible();
    for (const name of ['Overview', 'Products & offers', 'Placements', 'Earnings', 'Setup & agents']) {
        await expect(nav.getByRole('link', { name, exact: true })).toBeVisible();
    }
}
async function fulfillStripped(route, transform) {
    const response = await route.fetch();
    const body = await response.text();
    const next = transform(body);
    if (next === body) throw new Error('negative fixture did not change the served document');
    await route.fulfill({ response, body: next });
}
function withoutApprovedHeading(html) {
    const next = html
        .replace(/<h1\b[^>]*>[\s\S]*?<\/h1>/gi, '')
        .replace(/<script\b[^>]*\btype=["']module["'][^>]*>[\s\S]*?<\/script>/gi, '');
    if (/<h1\b/i.test(next)) throw new Error('negative fixture did not remove the home h1');
    return next;
}
function withoutWorkspaceNav(html) {
    const next = html.replace(/<nav\b[^>]*aria-label="Workspace"[^>]*>[\s\S]*?<\/nav>/i, '');
    if (next.includes('aria-label="Workspace"')) throw new Error('negative fixture did not remove the workspace navigation');
    return next;
}
async function expectAssertionFailure(check, needles) {
    let caught = null;
    try {
        await check();
    } catch (error) {
        caught = error;
    }
    const message = String(caught && (caught.message || caught) || '');
    if (!caught || needles.some(needle => !message.includes(needle))) {
        throw new Error('negative regression: shared assertion did not fail when required content was removed\n' + message);
    }
    console.log('NEGATIVE REGRESSION assertion failed as required:');
    console.log(message);
}
async function proveMissingContent() {
    const headingContext = await browser.newContext({ viewport: desktop });
    contexts.push(headingContext);
    const headingPage = await headingContext.newPage();
    await headingPage.route(fixture.base + '/', async route => {
        if (route.request().resourceType() !== 'document') {
            await route.continue();
            return;
        }
        await fulfillStripped(route, withoutApprovedHeading);
    });
    await headingPage.goto(fixture.base + '/');
    await expect(headingPage.getByRole('link', { name: 'Sign in with Google', exact: true }).first()).toBeVisible();
    await expect(headingPage.locator('#how')).toBeVisible();
    await expectAssertionFailure(() => assertApprovedHeading(headingPage), ['toBeVisible', approvedH1, 'element(s) not found']);

    const regionPage = await owner(fixture.builder);
    await regionPage.route(fixture.base + '/owner/settings', async route => {
        if (route.request().resourceType() !== 'document') {
            await route.continue();
            return;
        }
        await fulfillStripped(route, withoutWorkspaceNav);
    });
    await regionPage.goto(fixture.base + '/owner/settings');
    await expect(regionPage.getByRole('heading', { level: 1, name: 'Setup & agents', exact: true })).toBeVisible();
    await expect(regionPage.locator('#main')).toBeVisible();
    await expectAssertionFailure(() => assertWorkspaceChrome(regionPage), ['toBeVisible', 'Workspace', 'element(s) not found']);
    console.log('Workspace browser negative regression passed: removing the approved heading or the workspace navigation fails the shared assertions.');
}
async function journey() {
    const builder = await owner(fixture.builder);
    await builder.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await builder.goto(fixture.base + '/');
    await assertHydratedHome(builder);
    await copyJoinLine(builder);
    await builder.getByRole('button', { name: 'Pause motion', exact: true }).click();
    await expect(builder.getByRole('button', { name: 'Play motion', exact: true })).toBeVisible();
    if (fixture.screenshots) {
        await mkdir(fixture.screenshots, { recursive: true });
        await builder.screenshot({ path: join(fixture.screenshots, 'hero.png') });
    }
    await screenshot(builder, 'home-desktop-1440x900');
    await noHorizontalOverflow(builder);
    await builder.emulateMedia({ reducedMotion: 'reduce' });
    await expect(builder.locator('canvas')).toHaveCount(0);
    await expect(builder.locator('[data-hero-bg="fallback"]')).toBeVisible();
    await revealGoogleSignIn(builder);
    await builder.setViewportSize(mobile);
    await noHorizontalOverflow(builder);
    await screenshot(builder, 'home-mobile-390x844');
    await builder.setViewportSize(desktop);
    await builder.emulateMedia({ reducedMotion: 'no-preference' });
    await expect(builder.getByRole('button', { name: 'Play motion', exact: true })).toBeVisible();
    const fallbackContext = await browser.newContext({ javaScriptEnabled: false, viewport: mobile });
    contexts.push(fallbackContext);
    const fallbackPage = await fallbackContext.newPage();
    await fallbackPage.goto(fixture.base + '/');
    await assertStaticHome(fallbackPage);
    await noHorizontalOverflow(fallbackPage);
    const noGPUContext = await browser.newContext({ viewport: desktop });
    contexts.push(noGPUContext);
    await noGPUContext.addInitScript(() => {
        Object.defineProperty(navigator, 'gpu', { configurable: true, get: () => undefined });
    });
    const noGPUPage = await noGPUContext.newPage();
    noGPUPage.on('pageerror', error => errors.push(error.message));
    await noGPUPage.goto(fixture.base + '/');
    await assertHydratedHome(noGPUPage);
    await revealGoogleSignIn(noGPUPage);
    await builder.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await builder.goto(fixture.base + '/owner/settings');
    await assertWorkspaceChrome(builder);
    await expect(builder.getByRole('heading', { level: 1, name: 'Setup & agents', exact: true })).toBeVisible();
    await expect(builder.getByRole('heading', { name: 'Connect your existing agent', exact: true })).toBeVisible();
    const command = await builder.locator('.setup-command pre').textContent();
    await builder.getByRole('button', { name: 'Copy command', exact: true }).click();
    await expect(builder.locator('.setup-command .copy-status')).toContainText('Copied.');
    expect(await builder.evaluate(() => navigator.clipboard.readText())).toBe(command);
    await builder.context().clearPermissions();
    await builder.addInitScript(() => {
        Object.defineProperty(navigator, 'clipboard', { value: { writeText: async () => { throw new Error('Clipboard unavailable'); } } });
    });
    await builder.reload();
    await assertWorkspaceChrome(builder);
    await builder.getByRole('button', { name: 'Copy command', exact: true }).click();
    await expect(builder.locator('.setup-command .copy-status')).toContainText('Select and copy');
    expect(await builder.evaluate(() => window.getSelection().toString())).toBe(command);
    await builder.goto(fixture.base + '/owner/products');
    await assertWorkspaceChrome(builder);
    await expect(builder.getByRole('heading', { level: 1, name: 'Products & offers', exact: true })).toBeVisible();
    await expect(builder.getByRole('region', { name: 'Offer progress' })).toBeVisible();
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
    await assertWorkspaceChrome(builder);
    await expect(builder.getByRole('heading', { level: 1, name: 'Offer review', exact: true })).toBeVisible();
    await expect(builder.getByRole('heading', { name: 'Prepare commission payouts' })).toBeVisible();
    await builder.goto(fixture.base + '/owner/earnings');
    await assertWorkspaceChrome(builder);
    await expect(builder.getByRole('heading', { level: 1, name: 'Earnings', exact: true })).toBeVisible();
    await builder.getByLabel('Payment reference').fill('browser-check-reference');
    await builder.getByLabel('I have sent this payment outside Offer Rails.').check();
    await builder.getByRole('button', { name: 'Report payment sent' }).click();
    await expect(builder.locator('#workspace-feedback')).toContainText('publisher must confirm receipt separately');
    const publisher = await owner(fixture.publisher);
    await publisher.goto(fixture.base + '/owner/settings');
    await assertWorkspaceChrome(publisher);
    await publisher.getByLabel('Payment contact instructions').fill('Contact https://publisher.example.com/contact');
    await publisher.getByRole('button', { name: 'Save instructions' }).click();
    await expect(publisher.locator('#workspace-feedback')).toContainText('Payment instructions saved');
    await publisher.goto(fixture.base + '/owner/placements');
    await assertWorkspaceChrome(publisher);
    await expect(publisher.getByRole('heading', { level: 1, name: 'Placements', exact: true })).toBeVisible();
    await expect(publisher.getByRole('heading', { name: 'Find a relevant offer', exact: true })).toBeVisible();
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
    await assertWorkspaceChrome(publisher);
    await publisher.getByRole('button', { name: 'Confirm funds arrived' }).click();
    await publisher.waitForURL(fixture.base + '/owner');
    await publisher.goto(fixture.base + '/owner/earnings');
    await expect(publisher.getByText('You confirmed receipt.', { exact: true })).toBeVisible();
    await screenshot(publisher, 'earnings');
    await publisher.setViewportSize(mobile);
    await publisher.goto(fixture.base + '/owner/placements');
    await expect(publisher.getByRole('button', { name: 'Find opportunities' })).toBeVisible();
    await noHorizontalOverflow(publisher);
    await screenshot(publisher, 'placements-mobile-390x844');
    expect(errors).toEqual([]);
    console.log('Workspace browser integration passed: products, offers, placement approval, tracked link, payment reporting and receipt.');
}
try {
    if (fixture.mode === 'negative') await proveMissingContent();
    else if (fixture.mode === undefined || fixture.mode === 'journey') await journey();
    else throw new Error('unknown workspace browser mode: ' + fixture.mode);
} finally {
    await Promise.all(contexts.map(context => context.close()));
    await browser.close();
}
