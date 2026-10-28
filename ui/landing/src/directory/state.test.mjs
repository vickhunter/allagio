import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { DirectoryFailure } from './client.mjs'
import { directoryChrome, directoryFailureCopy, directoryFailureStatus } from './state.mjs'

test('a failed directory load names the status and is not the empty list', () => {
  assert.equal(directoryFailureCopy(new DirectoryFailure(404, 'not found')), "Couldn't load the directory. 404")
  assert.equal(directoryFailureCopy(new DirectoryFailure(0, 'network')), "Couldn't load the directory. network")
  assert.equal(directoryFailureStatus(new DirectoryFailure(502, 'json')), '502')
  const page = readFileSync(new URL('./Directory.tsx', import.meta.url), 'utf8')
  assert.equal(page.includes('No makers listed yet'), false)
  assert.equal(page.includes('No publishers listed yet'), false)
  assert.match(page, /directoryFailureCopy/)
  assert.equal(directoryChrome('error', 0).error, true)
  assert.equal(directoryChrome('error', 0).search, false)
  assert.equal(directoryChrome('error', 0).heading, false)
  assert.equal(directoryFailureCopy(new DirectoryFailure(404, 'not found')).includes('No makers listed yet'), false)
})

test('an empty directory hides the search and the heading', () => {
  const empty = directoryChrome('empty', 0)
  assert.equal(empty.search, false)
  assert.equal(empty.heading, false)
  assert.equal(empty.list, false)
  assert.equal(empty.add, true)
  const live = directoryChrome('results', 3)
  assert.equal(live.search, true)
  assert.equal(live.heading, true)
  assert.equal(live.list, true)
  assert.equal(directoryChrome('loading', 0).add, false)
})
