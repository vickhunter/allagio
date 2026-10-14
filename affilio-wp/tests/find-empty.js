const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const sourcePath = path.join(__dirname, '../admin/build/workspace.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const context = {
    document: {
        getElementById() { return null; },
        querySelector() { return null; },
        querySelectorAll() { return []; },
        body: { innerText: '' },
    },
    location: { pathname: '/owner/placements', assign() {}, reload() {} },
    sessionStorage: { getItem() { return null; }, setItem() {} },
    fetch() { return Promise.resolve({ ok: false, status: 404, json: async () => ({}) }); },
    console,
};
vm.createContext(context);
vm.runInContext(source, context, { filename: 'workspace.js' });

assert.strictEqual(typeof context.offerRailsFindEmptyState, 'function');
assert.strictEqual(typeof context.offerRailsFindInputError, 'function');

const below = context.offerRailsFindEmptyState(
    { reason: 'below_relevance', best_score: 18, threshold: 25 },
    'Gardening tips for tomatoes',
    ''
);
assert.ok(below.detail.includes('relevance 18/100, needs 25'), below.detail);
assert.ok(!below.detail.includes('verified payment'), below.detail);
assert.ok(below.detail.includes('longer excerpt'), below.detail);
assert.ok(below.detail.includes('fetch the text'), below.detail);
assert.strictEqual(below.offerFetch, false);

const withURL = context.offerRailsFindEmptyState(
    { reason: 'below_relevance', best_score: 12, threshold: 25 },
    'A short topic',
    'http://127.0.0.1:8088/post'
);
assert.ok(withURL.detail.includes('relevance 12/100, needs 25'), withURL.detail);
assert.ok(!withURL.detail.includes('verified payment'), withURL.detail);
assert.strictEqual(withURL.offerFetch, true);

const long = context.offerRailsFindEmptyState(
    { reason: 'below_relevance', best_score: 18, threshold: 25 },
    'x'.repeat(400),
    'http://127.0.0.1:8088/post'
);
assert.ok(long.detail.includes('relevance 18/100, needs 25'), long.detail);
assert.ok(!long.detail.includes('longer excerpt'), long.detail);
assert.ok(!long.detail.includes('verified payment'), long.detail);
assert.strictEqual(long.offerFetch, false);

const payment = context.offerRailsFindEmptyState({ reason: 'no_eligible_offers' }, 'x'.repeat(500), 'https://publisher.example.com/post');
assert.ok(payment.detail.includes('Only listed offers with verified payment configuration are eligible.'), payment.detail);
assert.ok(!payment.detail.includes('relevance'), payment.detail);
assert.strictEqual(payment.offerFetch, false);

assert.strictEqual(context.offerRailsFindInputError('Gardening tips for tomatoes'), '');
assert.ok(context.offerRailsFindInputError('hi').includes('optional'));
assert.ok(context.offerRailsFindInputError(''));

assert.ok(
    source.includes("field('surface_url','Published article URL','','type=\"url\"',true)"),
    'article URL is still required on the find form'
);
assert.ok(
    !source.includes("field('surface_url','Published article URL','','type=\"url\" required',true)"),
    'find form still marks the article URL required'
);

console.log('Find empty state names the real reason, and the article URL is optional when the about-text is filled.');
