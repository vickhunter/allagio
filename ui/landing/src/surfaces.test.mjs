import assert from 'node:assert/strict'
import test from 'node:test'
import { orderedSurfaces, surfaceStatus, surfaces } from './surfaces.mjs'

test('a surface is verified end to end only when both checks passed', () => {
  assert.equal(surfaceStatus({ adapterTests: true, e2e: true }), 'Verified end to end')
  assert.equal(surfaceStatus({ adapterTests: true, e2e: false }), 'In testing')
  assert.equal(surfaceStatus({ adapterTests: false, e2e: true }), 'In testing')
  assert.ok(surfaces.every((item) => item.adapterTests === false && item.e2e === false))
  assert.ok(surfaces.every((item) => item.date === '2026-10-21'))
  const publishers = orderedSurfaces('publisher')
  assert.deepEqual(publishers.map((item) => item.name), ['WordPress', 'X', 'Newsletter', 'YouTube', 'Instagram', 'TikTok'])
  assert.ok(publishers.every((item) => surfaceStatus(item) === 'In testing'))
  const makers = orderedSurfaces('maker')
  assert.deepEqual(makers.map((item) => item.name), ['WordPress', 'X', 'Shopify'])
  assert.equal(new Set(surfaces.map((item) => item.name)).size, surfaces.length)
  assert.equal(JSON.stringify(surfaces).includes('Planned'), false)
})
