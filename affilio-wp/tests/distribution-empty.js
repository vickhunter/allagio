const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const sourcePath = path.join(__dirname, '../admin/build/distribution.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const context = {
    document: { getElementById() { return null; } },
    console,
};
vm.createContext(context);
vm.runInContext(source, context, { filename: 'distribution.js' });

assert.strictEqual(typeof context.affilioEmptyOfferMessage, 'function', 'affilioEmptyOfferMessage is missing');
assert.strictEqual(
    context.affilioEmptyOfferMessage({ reason: 'below_relevance', best_score: 6, threshold: 25 }),
    'Nessuna offerta adatta: la più vicina ha rilevanza 6/100, serve almeno 25.'
);
assert.strictEqual(
    context.affilioEmptyOfferMessage({ reason: 'no_eligible_offers' }),
    'Nessuna offerta idonea al momento.'
);
assert.ok(
    source.includes('Valuta ${offer.product_name} se risolve il problema descritto in questo articolo.')
    || source.includes("Valuta ' + offer.product_name + ' se risolve il problema descritto in questo articolo."),
    'default recommendation copy is not Italian'
);

console.log('Distribution empty state explains no_match in Italian.');
