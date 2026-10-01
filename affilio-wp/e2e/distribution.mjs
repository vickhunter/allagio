import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';

let input = '';
for await (const chunk of process.stdin) input += chunk;
const fixture = JSON.parse(input);
const credentials = JSON.parse(await readFile(join(fixture.qa, 'credentials.json'), 'utf8'));
const wp = (...args) => execFileSync('docker', ['compose', '-p', 'distributoor-mvp-wp-qa', '-f', join(fixture.qa, 'compose.json'), 'run', '--rm', 'cli', 'wp', ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
if (fixture.download) {
    const response = await fetch(fixture.base + '/downloads/affilio.zip');
    expect(response.status).toBe(200);
    await writeFile(join(fixture.qa, 'affilio.zip'), Buffer.from(await response.arrayBuffer()));
    wp('plugin', 'install', '/release/affilio.zip', '--force', '--activate');
    wp('eval', "delete_option('affilio_api_url_override');");
    expect(wp('eval', 'echo AFFILIO_API_URL;')).toBe(fixture.base + '/v1');
} else {
    wp('option', 'update', 'affilio_api_url_override', fixture.base + '/v1');
}
wp('eval', "foreach (array('affilio_api_key','affilio_site_id','affilio_installation_key','affilio_setup_state','affilio_setup_lock') as $option) delete_option($option); delete_transient('affilio_connection_status');");
const existing = '<p>InboxPilot email inbox productivity tutorial for developers. Keep C:\\tools and the "quoted" R&amp;D example.</p>';
const postID = wp('post', 'create', '--post_title=Email inbox developer tutorial', '--post_status=publish', '--post_content=' + existing, '--porcelain');
const browser = await chromium.launch({ headless: true, args: ['--host-resolver-rules=MAP publisher.example.com 127.0.0.1', '--no-proxy-server'] });
const errors = [];
let page;
try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    const wordpress = 'https://publisher.example.com:8443';
    await page.goto(wordpress + '/wp-login.php');
    await expect(page.locator('#user_login')).toBeFocused();
    await page.locator('#user_login').fill(credentials.user);
    await page.locator('#user_pass').fill(credentials.password);
    expect(await page.locator('#loginform').evaluate(form => form.checkValidity())).toBe(true);
    await Promise.all([
        page.waitForURL(/wp-admin/, { waitUntil: 'domcontentloaded' }),
        page.locator('#user_pass').press('Enter')
    ]);
    await page.goto(wordpress + '/wp-admin/admin.php?page=affilio-settings');
    await page.getByRole('button', { name: 'Connect and find opportunities', exact: true }).click();
    await expect(page.locator('[data-progress]')).toContainText('Scanning in the background');
    await page.goto(wordpress + '/wp-admin/admin.php?page=affilio-distribution');
    await page.getByRole('button', { name: 'Connect publisher account', exact: true }).click();
    const connection = page.getByRole('link', { name: 'Sign in and confirm this site ↗' });
    await expect(connection).toBeVisible();
    const ownerContext = await browser.newContext();
    await ownerContext.addCookies([{ name: fixture.owner.Name, value: fixture.owner.Value, url: fixture.base, httpOnly: true, sameSite: 'Lax' }]);
    const owner = await ownerContext.newPage();
    owner.on('pageerror', error => errors.push(error.message));
    await owner.goto(await connection.getAttribute('href'));
    await owner.getByRole('button', { name: 'Connect this site to my account' }).click();
    await expect(owner.getByRole('heading', { name: 'Your site is connected' })).toBeVisible();
    await page.getByRole('button', { name: 'Refresh connection', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Your publisher account is connected.' })).toBeVisible();
    await page.getByLabel('Published article').selectOption(postID);
    await page.getByRole('button', { name: 'Find relevant offers', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'InboxPilot', exact: true })).toBeVisible();
    await page.getByLabel('Recommendation copy').fill('InboxPilot can help developers organize an email inbox for R&D.');
    await page.getByRole('button', { name: 'Prepare for review', exact: true }).click();
    const review = page.getByRole('link', { name: 'Review copy & terms ↗', exact: true });
    await expect(review).toBeVisible();
    await expect(page.getByRole('button', { name: 'Insert approved recommendation', exact: true })).toHaveCount(0);
    await owner.goto(await review.getAttribute('href'));
    await owner.getByRole('button', { name: 'Approve this exact placement', exact: true }).click();
    await expect(owner.getByRole('button', { name: 'Create tracked link' })).toBeVisible();
    await page.getByRole('button', { name: 'Refresh approvals', exact: true }).click();
    await page.getByRole('button', { name: 'Insert approved recommendation', exact: true }).click();
    await expect(page.locator('#distribution-message')).toContainText('Published.');
    await expect(page.getByRole('button', { name: 'Insert approved recommendation', exact: true })).toHaveCount(0);
    const content = wp('post', 'get', postID, '--field=post_content');
    expect(content.startsWith(existing)).toBe(true);
    expect(content.match(/distributoor-placement:/g)).toHaveLength(1);
    expect(content.match(/Affiliate disclosure:/g)).toHaveLength(1);
    expect(content).toContain('rel="sponsored nofollow"');
    expect(content).toContain('R&amp;D.');
    await page.getByRole('button', { name: 'Refresh approvals', exact: true }).click();
    expect(wp('post', 'get', postID, '--field=post_content')).toBe(content);
    const reader = await browser.newContext({ ignoreHTTPSErrors: true });
    const published = await reader.newPage();
    await published.goto(wordpress + '/?p=' + postID);
    const recommendation = published.locator('a[rel="sponsored nofollow"]');
    await expect(recommendation).toHaveCount(1);
    await expect(published.getByText('Affiliate disclosure:', { exact: false })).toBeVisible();
    const tracked = await recommendation.getAttribute('href');
    expect(new URL(tracked).origin).toBe(fixture.base);
    expect(new URL(tracked).searchParams.get('placement')).toMatch(/^[0-9a-f-]{36}$/);
    const redirect = await reader.request.get(tracked, { maxRedirects: 0 });
    expect(redirect.status()).toBe(302);
    const destination = new URL(redirect.headers().location);
    expect(destination.origin).toBe('https://inboxpilot.example.com');
    expect(destination.searchParams.get('via')).toMatch(/^[0-9a-f-]{36}$/);
    await reader.close();
    if (fixture.screenshots) {
        await mkdir(fixture.screenshots, { recursive: true });
        await page.screenshot({ path: join(fixture.screenshots, 'wordpress.png'), fullPage: true });
    }
    expect(errors).toEqual([]);
    await ownerContext.close();
    await context.close();
    console.log(`WordPress browser integration passed: ${fixture.download ? 'downloaded ZIP installation and embedded API origin, ' : ''}installation connection, owner handoff, matching, approval, real post insertion, publication receipt and attributed reader click.`);
} catch (error) {
    console.error('WordPress page:', page?.url(), (await page?.locator('body').innerText())?.slice(0, 1200));
    throw error;
} finally {
    await browser.close();
}
