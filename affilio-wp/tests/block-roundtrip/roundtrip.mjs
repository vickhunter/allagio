import { JSDOM, VirtualConsole } from 'jsdom';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', () => {});
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://blog.example/', virtualConsole });
const { window } = dom;
for (const [key, value] of Object.entries({
  window,
  document: window.document,
  navigator: window.navigator,
  HTMLElement: window.HTMLElement,
  Node: window.Node,
  Element: window.Element,
  CSS: window.CSS,
  DOMParser: window.DOMParser,
  MutationObserver: window.MutationObserver,
  getComputedStyle: window.getComputedStyle.bind(window),
  requestAnimationFrame: (callback) => setTimeout(callback, 0),
  cancelAnimationFrame: (id) => clearTimeout(id),
})) {
  Object.defineProperty(global, key, { value, configurable: true });
}

const require = createRequire(import.meta.url);
const { parse, serialize } = require('@wordpress/blocks');
const { registerCoreBlocks } = require('@wordpress/block-library');
registerCoreBlocks();

const id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
const copy = 'Uno strumento utile per il team.';
const disclosure = 'Affiliate disclosure: I may earn a commission if you buy through this link.';
const href = `https://distribution.example/v1/go/test?placement=${id}`;
const anchor = 'Vedi&nbsp;il&nbsp;prodotto';
const legacyAnchor = 'Vedi il prodotto';
const kept = '<!-- wp:paragraph -->\n<p>Articolo da conservare.</p>\n<!-- /wp:paragraph -->';
const placementClass = `affilio-placement-${id}`;
const disclosureClass = `affilio-disclosure-${id}`;
const link = `${copy} <a href="${href}" rel="sponsored nofollow">${anchor}</a> (link sponsorizzato)`;
const legacyLink = `${copy} <a href="${href}" rel="sponsored nofollow">${legacyAnchor}</a>`;
const inserted = [
  kept,
  `<!-- wp:paragraph {"className":"${placementClass}"} -->`,
  `<p class="${placementClass}">${link}</p>`,
  '<!-- /wp:paragraph -->',
  `<!-- wp:paragraph {"className":"${disclosureClass}"} -->`,
  `<p class="${disclosureClass}">${disclosure}</p>`,
  '<!-- /wp:paragraph -->',
].join('\n');

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}

function capturing(fn) {
  const logs = [];
  const previous = {};
  for (const method of ['log', 'info', 'warn', 'error', 'debug']) {
    previous[method] = console[method];
    console[method] = (...args) => {
      logs.push(args.map((arg) => (typeof arg === 'string' ? arg : '')).join(' '));
    };
  }
  try {
    return { value: fn(), logs };
  } finally {
    Object.assign(console, previous);
  }
}

function placementBlock(blocks) {
  return blocks.find((block) => String(block.attributes.className || '').split(/\s+/).includes(placementClass));
}

const parsed = capturing(() => parse(inserted));
const placement = placementBlock(parsed.value);
if (!placement || placement.name !== 'core/paragraph') {
  fail('inserted placement was not a core/paragraph');
}
if (!placement.isValid) {
  fail('placement block is not valid with the current save()');
}
if (parsed.logs.some((line) => line.includes('Block successfully updated'))) {
  fail('placement block used a deprecation');
}
const saved = String(placement.attributes.content || '');
if (!saved.includes('Vedi&nbsp;il&nbsp;prodotto') && !saved.includes('Vedi\u00A0il\u00A0prodotto')) {
  fail('anchor text lost the non-breaking spaces');
}
if (!saved.includes('(link sponsorizzato)') || /<(span|badge)\b/i.test(saved)) {
  fail('sponsor label is not plain text in the same paragraph');
}
if (!/<\/a>\s*\(link sponsorizzato\)/.test(saved)) {
  fail('sponsor label is inside the link or missing after it');
}
const nbspInserted = inserted.replaceAll(anchor, 'Vedi\u00A0il\u00A0prodotto');
const nbspParsed = capturing(() => parse(nbspInserted));
const nbspPlacement = placementBlock(nbspParsed.value);
if (!nbspPlacement || nbspPlacement.name !== 'core/paragraph' || !nbspPlacement.isValid) {
  fail('placement with a raw U+00A0 anchor was not a valid core/paragraph');
}
if (nbspParsed.logs.some((line) => line.includes('Block successfully updated'))) {
  fail('raw U+00A0 anchor used a deprecation');
}
const extraSource = inserted
  .replaceAll(`"className":"${placementClass}"`, `"className":"${placementClass} my-extra"`)
  .replaceAll(`<p class="${placementClass}">`, `<p class="${placementClass} my-extra">`);
const extraParsed = capturing(() => parse(extraSource));
const extraBlock = placementBlock(extraParsed.value);
if (!extraBlock || !extraBlock.isValid || extraParsed.logs.some((line) => line.includes('Block successfully updated'))) {
  fail('an extra editor class did not stay valid on the current save()');
}
const roundTripped = capturing(() => serialize(parsed.value)).value;
const legacy = `${kept}\n<!-- wp:paragraph -->\n<!-- distributoor-placement:${id} --><p>${legacyLink}</p>\n<!-- /wp:paragraph -->`;
const resaved = capturing(() => serialize(parse(legacy.replace(`${kept}\n`, '')))).value;
const here = path.dirname(fileURLToPath(import.meta.url));
const fixture = readFileSync(path.join(here, '../fixtures/editor-resave-013.txt'), 'utf8');
if (fixture !== resaved) {
  fail(`editor re-save fixture drifted. Produced:\n${resaved}`);
}
const note = readFileSync(path.join(here, '../fixtures/editor-resave-013.note.txt'), 'utf8');
if (!note.includes('@wordpress/blocks') || !note.includes('serialize(parse(')) {
  fail('editor re-save note does not say how the fixture was made');
}

const otherId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
const otherBlock = [
  '<!-- wp:paragraph -->',
  `<p>Altro <a href="https://distribution.example/v1/go/other?placement=${otherId}" rel="sponsored nofollow">${anchor}</a> (link sponsorizzato)</p>`,
  '<!-- /wp:paragraph -->',
].join('\n');
const mentionBlock = [
  '<!-- wp:paragraph -->',
  `<p>Il placement ${id} compare solo nel testo.</p>`,
  '<!-- /wp:paragraph -->',
].join('\n');
const stripped = [
  kept,
  '<!-- wp:paragraph -->',
  `<p>${copy} <a href="${href}" rel="sponsored nofollow">${anchor}</a> (link sponsorizzato)</p>`,
  '<!-- /wp:paragraph -->',
  otherBlock,
  mentionBlock,
].join('\n');
const foreign = [
  kept,
  '<!-- wp:paragraph -->',
  `<p><a href="https://shop.example/item?placement=${id}" rel="sponsored nofollow">${anchor}</a> (link sponsorizzato)</p>`,
  '<!-- /wp:paragraph -->',
].join('\n');
const php = spawnSync('php', [path.join(here, 'revert-fixture.php')], {
  input: JSON.stringify({
    inserted: roundTripped,
    extra: capturing(() => serialize(extraParsed.value)).value,
    resaved: `${kept}\n${resaved}`,
    kept,
    id,
    otherId,
    stripped,
    strippedKept: [kept, otherBlock, mentionBlock].join('\n'),
    foreign,
  }),
  encoding: 'utf8',
});
if (php.status !== 0) {
  fail(php.stderr || php.stdout || 'php revert of round-tripped content failed');
}
process.stdout.write('Gutenberg save round-trip keeps the placement valid and revertible.\n');
