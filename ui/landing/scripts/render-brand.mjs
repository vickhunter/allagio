// Rasterize the Offer Rails rail mark. The geometry is the header mark
// (two rails, four ties, one maker dot), squared so it still reads at 32px.
// Run: NODE_PATH=/tmp/brand-tools/node_modules node scripts/render-brand.mjs
// Needs @resvg/resvg-js, opentype.js, and /tmp/archivo-700.ttf (Archivo Bold subset).
import { mkdirSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

function loadTool(name) {
  const roots = [new URL('../package.json', import.meta.url)]
  if (process.env.BRAND_TOOLS) roots.push(process.env.BRAND_TOOLS.replace(/\/$/, '') + '/package.json')
  for (const root of roots) {
    try {
      return createRequire(root)(name)
    } catch {
      // try the next install
    }
  }
  throw new Error('Install ' + name + ' or set BRAND_TOOLS to a folder that has it.')
}

const { Resvg } = loadTool('@resvg/resvg-js')
const opentype = loadTool('opentype.js')

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const out = resolve(root, 'public/brand')
const fontFile = process.env.ARCHIVO_TTF || '/tmp/archivo-700.ttf'
const ink = '#232720'
const cream = '#F7F7EF'
const maker = '#C64F38'

function railsMark(stroke) {
  return `<g fill="none" stroke="${stroke}" stroke-linecap="square">
    <path d="M2.6 12.6h26.8M2.6 19.4h26.8" stroke-width="2.35"/>
    <path d="M5.9 9.3v13.4M12.6 9.3v13.4M19.4 9.3v13.4M26.1 9.3v13.4" stroke-width="1.8" stroke-opacity="0.72"/>
    <circle cx="20.5" cy="16" r="4.5" fill="${maker}" stroke="${cream}" stroke-width="1.8"/>
  </g>`
}

const rails = railsMark(ink)
const railsDark = railsMark(cream)

function svgDoc(inner, size = 32) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="${size}" height="${size}" role="img" aria-labelledby="title">
  <title id="title">Offer Rails</title>
  ${inner}
</svg>
`
}

function raster(svg, size) {
  const resvg = new Resvg(svg, {
    fitTo: { mode: 'width', value: size },
    font: { fontFiles: [fontFile], loadSystemFonts: true },
  })
  const rendered = resvg.render()
  if (rendered.width !== size) throw new Error('width ' + rendered.width + ' expected ' + size)
  return rendered.asPng()
}

function ico(images) {
  const header = Buffer.alloc(6)
  header.writeUInt16LE(0, 0)
  header.writeUInt16LE(1, 2)
  header.writeUInt16LE(images.length, 4)
  let offset = 6 + 16 * images.length
  const entries = images.map(({ size, png }) => {
    const entry = Buffer.alloc(16)
    entry.writeUInt8(size >= 256 ? 0 : size, 0)
    entry.writeUInt8(size >= 256 ? 0 : size, 1)
    entry.writeUInt16LE(1, 4)
    entry.writeUInt16LE(32, 6)
    entry.writeUInt32LE(png.length, 8)
    entry.writeUInt32LE(offset, 12)
    offset += png.length
    return entry
  })
  return Buffer.concat([header, ...entries, ...images.map((image) => image.png)])
}

function wordmarkParts() {
  const font = opentype.loadSync(fontFile)
  const fontSize = 22
  const path = font.getPath('Offer Rails', 46, 27.2, fontSize)
  const box = path.getBoundingBox()
  const width = Math.ceil(box.x2 + 2)
  const height = 36
  const inner = `<g transform="translate(2 2)">${rails}</g><path fill="${ink}" d="${path.toPathData(2)}"/>`
  return { width, height, inner }
}

function wordmark() {
  const { width, height, inner } = wordmarkParts()
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title">
  <title id="title">Offer Rails</title>
  ${inner}
</svg>
`
}

mkdirSync(out, { recursive: true })
const transparent = svgDoc(rails)
const dark = svgDoc(railsDark)
const light = svgDoc(`<rect width="32" height="32" fill="${cream}"/>\n  ${rails}`)
writeFileSync(resolve(out, 'mark.svg'), transparent)
writeFileSync(resolve(out, 'mark-dark.svg'), dark)
writeFileSync(resolve(out, 'favicon.svg'), light)
writeFileSync(resolve(root, 'public/favicon.svg'), light)
writeFileSync(resolve(out, 'wordmark.svg'), wordmark())

const sizes = [
  ['mark-transparent-512.png', transparent, 512],
  ['mark-transparent-1024.png', transparent, 1024],
  ['mark-dark-512.png', dark, 512],
  ['mark-dark-1024.png', dark, 1024],
  ['mark-light-512.png', light, 512],
  ['mark-light-1024.png', light, 1024],
  ['favicon-16.png', light, 16],
  ['favicon-32.png', light, 32],
  ['apple-touch-icon.png', light, 180],
]
const pngs = {}
for (const [name, svg, size] of sizes) {
  pngs[name] = raster(svg, size)
  writeFileSync(resolve(out, name), pngs[name])
}
writeFileSync(resolve(out, 'favicon.ico'), ico([
  { size: 16, png: pngs['favicon-16.png'] },
  { size: 32, png: pngs['favicon-32.png'] },
]))

const lockup = wordmarkParts()
const sheet = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1168" height="860" viewBox="0 0 1168 860">
  <rect width="1168" height="860" fill="${cream}"/>
  <text x="40" y="40" fill="${ink}" font-family="Inter" font-size="20" font-weight="650">Offer Rails rail mark</text>
  <text x="40" y="88" fill="${ink}" font-family="Inter" font-size="14" font-weight="600">32</text>
  <text x="112" y="88" fill="${ink}" font-family="Inter" font-size="14" font-weight="600">64</text>
  <rect x="36" y="100" width="40" height="40" fill="${cream}" stroke="#dde0d3"/>
  <g transform="translate(40 104)">${rails}</g>
  <rect x="108" y="100" width="72" height="72" fill="${cream}" stroke="#dde0d3"/>
  <g transform="translate(112 104) scale(2)">${rails}</g>
  <text x="40" y="214" fill="${ink}" font-family="Inter" font-size="14" font-weight="600">512 on light</text>
  <text x="616" y="214" fill="${cream}" font-family="Inter" font-size="14" font-weight="600">512 on dark</text>
  <rect x="36" y="228" width="520" height="520" fill="${cream}" stroke="#dde0d3"/>
  <g transform="translate(40 232) scale(16)">${rails}</g>
  <rect x="576" y="196" width="552" height="568" fill="${ink}"/>
  <g transform="translate(596 232) scale(16)">${railsDark}</g>
  <g transform="translate(40 800)">${lockup.inner}</g>
</svg>`
const sheetPng = new Resvg(sheet, {
  fitTo: { mode: 'width', value: 1168 },
  font: { fontFiles: [fontFile], loadSystemFonts: true },
}).render().asPng()
const shots = [
  '/opt/cursor/artifacts/screenshots/logo-contact-sheet.png',
  resolve(root, '../../affilio-wp/e2e/landing-v9-proof/logo-contact-sheet.png'),
]
for (const path of shots) {
  mkdirSync(dirname(path), { recursive: true })
  writeFileSync(path, sheetPng)
}
console.log('brand assets in public/brand')
