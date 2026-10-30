import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { DirectoryFailure, listDirectory } from './client.mjs'
import { directoryChrome, directoryEmptyCopy, directoryFailureCopy, directoryFailureStatus } from './state.mjs'

function jsonResponse(status, body) {
  return { ok: status >= 200 && status < 300, status, json: async () => body }
}

test('a 503 is an error and a 200 with total 0 is the empty list', async () => {
  await assert.rejects(
    listDirectory('maker', {}, async () => jsonResponse(503, { error: 'database' })),
    (error) => error instanceof DirectoryFailure && error.status === 503,
  )
  const empty = await listDirectory('maker', {}, async () => jsonResponse(200, { items: [], next_cursor: null, total: 0 }))
  assert.deepEqual(empty, { items: [], next_cursor: null, total: 0 })

  const failed = directoryChrome('error', 0)
  assert.equal(failed.error, true)
  assert.equal(failed.empty, false)
  assert.equal(failed.heading, true)
  assert.equal(failed.search, true)
  assert.equal(directoryFailureCopy(new DirectoryFailure(503, 'directory')), 'The list could not load.')
  assert.equal(directoryFailureCopy(new DirectoryFailure(503, 'directory')).includes('No makers listed yet'), false)
  assert.equal(directoryFailureStatus(new DirectoryFailure(503, 'directory')), '503')

  const blank = directoryChrome('empty', 0)
  assert.equal(blank.search, false)
  assert.equal(blank.heading, false)
  assert.equal(blank.empty, true)
  assert.equal(blank.error, false)
  assert.match(directoryEmptyCopy('maker'), /^No makers listed yet/)
  assert.match(directoryEmptyCopy('publisher'), /^No publishers listed yet/)

  const page = readFileSync(new URL('./Directory.tsx', import.meta.url), 'utf8')
  assert.match(page, /role="alert"/)
  assert.match(page, /Retry/)
  assert.match(page, /directoryEmptyCopy/)
})

test('an empty directory hides the search and a live list shows it', () => {
  const empty = directoryChrome('empty', 0)
  assert.equal(empty.search, false)
  assert.equal(empty.add, true)
  const live = directoryChrome('results', 3)
  assert.equal(live.search, true)
  assert.equal(live.heading, true)
  assert.equal(live.list, true)
  assert.equal(directoryChrome('loading', 0).add, false)
  assert.equal(directoryChrome('error', 0).search, true)
  assert.equal(directoryChrome('error', 0).heading, true)
})
