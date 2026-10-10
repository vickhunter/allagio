const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const sourcePath = path.join(__dirname, '../admin/build/landing.js');
const source = fs.readFileSync(sourcePath, 'utf8');

function load(search, links) {
    const context = {
        URL,
        URLSearchParams,
        matchMedia() { return { matches: false, addEventListener() {} }; },
        navigator: {},
        window: { addEventListener() {} },
        document: {
            querySelector() { return null; },
            querySelectorAll(selector) {
                if (selector === 'a[href]') return links;
                return [];
            },
        },
        location: { search },
        console,
    };
    vm.createContext(context);
    vm.runInContext(source, context, { filename: 'landing.js' });
    return context;
}

function link(href) {
    return {
        href,
        getAttribute() { return this.href; },
        setAttribute(name, value) { if (name === 'href') this.href = value; },
    };
}

assert.strictEqual(
    load('', []).offerRailsWithAttributionSource('/owner?setup=maker', 'chatgpt'),
    '/owner?setup=maker&source=chatgpt'
);
assert.strictEqual(
    load('', []).offerRailsWithAttributionSource('/owner', 'Claude'),
    '/owner?source=claude'
);
assert.strictEqual(load('', []).offerRailsWithAttributionSource('/owner', 'facebook'), '/owner');
assert.strictEqual(load('', []).offerRailsWithAttributionSource('/owner', 'direct'), '/owner');
assert.strictEqual(
    load('', []).offerRailsWithAttributionSource('https://evil.example/phish', 'chatgpt'),
    'https://evil.example/phish'
);

const workspace = link('/owner');
const maker = link('/owner?setup=maker');
const settings = link('/owner/settings');
const anchor = link('#publishers');
load('?source=cursor', [workspace, maker, settings, anchor]);
assert.strictEqual(workspace.href, '/owner?source=cursor');
assert.strictEqual(maker.href, '/owner?setup=maker&source=cursor');
assert.strictEqual(settings.href, '/owner/settings?source=cursor');
assert.strictEqual(anchor.href, '#publishers');

const untouched = link('/owner?setup=maker');
load('?source=not-a-source', [untouched]);
assert.strictEqual(untouched.href, '/owner?setup=maker');

console.log('Landing links carry an allowed attribution source into signup.');
