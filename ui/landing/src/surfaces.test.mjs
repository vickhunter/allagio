import assert from 'node:assert/strict'
import test from 'node:test'
import { splitSurfaces, surfaces } from './surfaces.mjs'

test('where it runs leads with live surfaces from one list', () => {
  const publishers = splitSurfaces('publisher')
  assert.deepEqual(publishers.live.map((item) => item.state), ['Live'])
  assert.deepEqual(publishers.live.map((item) => item.name), ['WordPress'])
  assert.deepEqual(publishers.planned.map((item) => item.name), ['X', 'Newsletter', 'YouTube', 'Instagram', 'TikTok'])
  assert.ok(publishers.planned.every((item) => item.state === 'Planned'))
  const makers = splitSurfaces('maker')
  assert.deepEqual(makers.live.map((item) => item.name), ['WordPress'])
  assert.deepEqual(makers.planned.map((item) => item.name), ['X', 'Shopify'])
  assert.equal(new Set(surfaces.map((item) => item.name)).size, surfaces.length)
})
