import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEAL_TEMPLATES,
  advanceStep,
  applyHumanDecision,
  buildOrpEnvelope,
  createInitialState,
  getActiveArtifact,
  getActiveTerms,
  getTemplateById,
  resetSimulation,
  selectTemplate,
  shortHash,
  startPlaybackState,
  toggleJsonDrawer,
} from './deal-simulator-data.ts'

test('defines all 5 ORP deal templates with valid sha256 terms hashes and named agents', () => {
  assert.deepEqual(
    DEAL_TEMPLATES.map((t) => t.id),
    ['affiliate', 'sponsored', 'swap', 'ad_space', 'rev_share']
  )
  for (const tpl of DEAL_TEMPLATES) {
    assert.ok(tpl.label.length > 0)
    assert.match(tpl.initialTerms.termsHash, /^sha256:[0-9a-f]{64}$/)
    assert.match(tpl.counteredTerms.termsHash, /^sha256:[0-9a-f]{64}$/)
    assert.notEqual(tpl.initialTerms.termsHash, tpl.counteredTerms.termsHash)
    assert.ok(tpl.offerer.handle && tpl.acceptor.handle)
    assert.match(tpl.proofId, /^prf_/)
  }
})

test('template switching resets to ask gate and preserves JSON drawer state', () => {
  let state = createInitialState('affiliate')
  state = toggleJsonDrawer(state)
  assert.equal(state.showJson, true)

  state = applyHumanDecision(state, 'refuse')
  assert.equal(state.stage, 'refused')

  const switched = selectTemplate(state, 'rev_share')
  assert.equal(switched.templateId, 'rev_share')
  assert.equal(switched.stage, 'ask')
  assert.equal(switched.countered, false)
  assert.equal(switched.showJson, true)
})

test('playback advances discover -> offer -> ask and pauses at the human approval gate', () => {
  let state = startPlaybackState('sponsored')
  assert.equal(state.stage, 'discover')
  assert.equal(state.isPlaying, true)

  state = advanceStep(state)
  assert.equal(state.stage, 'offer')
  assert.equal(state.isPlaying, true)

  state = advanceStep(state)
  assert.equal(state.stage, 'ask')
  assert.equal(state.isPlaying, false)

  // Advancing while waiting at ask gate is a no-op until human decides
  const stillAsk = advanceStep(state)
  assert.equal(stillAsk.stage, 'ask')
})

test('approving at human gate transitions active -> execute -> receipt with proof ID', () => {
  let state = createInitialState('affiliate')
  state = applyHumanDecision(state, 'approve')
  assert.equal(state.stage, 'active')
  assert.equal(state.isPlaying, true)

  state = advanceStep(state)
  assert.equal(state.stage, 'execute')

  state = advanceStep(state)
  assert.equal(state.stage, 'receipt')
  assert.equal(state.isPlaying, false)

  const tpl = getTemplateById('affiliate')
  const envelope = buildOrpEnvelope(tpl, state)
  assert.equal(envelope.orp, '0.4')
  assert.equal(envelope.type, 'receipt')
  assert.equal(envelope.deal, tpl.dealId)
  assert.equal(envelope.body.proof_id, tpl.proofId)
  assert.equal(envelope.body.proof_url, `/orp/proof/${tpl.proofId}`)
  assert.equal(envelope.body.status, 'verified')
})

test('approving with immediate option (reduced motion) completes directly to receipt', () => {
  const state = applyHumanDecision(createInitialState('swap'), 'approve', { immediate: true })
  assert.equal(state.stage, 'receipt')
  assert.equal(state.isPlaying, false)
  assert.ok(state.history.includes('active'))
  assert.ok(state.history.includes('execute'))
  assert.ok(state.history.includes('receipt'))
})

test('countering updates terms hash and returns to ask gate for final decision', () => {
  const tpl = getTemplateById('affiliate')
  let state = createInitialState('affiliate')
  const initialHash = getActiveTerms(tpl, state.countered).termsHash

  state = applyHumanDecision(state, 'counter')
  assert.equal(state.stage, 'counter')
  assert.equal(state.countered, true)

  state = advanceStep(state)
  assert.equal(state.stage, 'ask')
  assert.equal(state.isPlaying, false)

  const counteredHash = getActiveTerms(tpl, state.countered).termsHash
  assert.notEqual(counteredHash, initialHash)
  assert.match(shortHash(counteredHash), /^sha256:[0-9a-f]{12}…[0-9a-f]{6}$/)

  const artifact = getActiveArtifact(tpl, state.countered)
  assert.equal(artifact.metrics[0].value, '20% recurring')
})

test('refusing halts the deal cleanly with zero placement and zero cash moved', () => {
  const tpl = getTemplateById('ad_space')
  const state = applyHumanDecision(createInitialState('ad_space'), 'refuse')
  assert.equal(state.stage, 'refused')
  assert.equal(state.isPlaying, false)

  const envelope = buildOrpEnvelope(tpl, state)
  assert.equal(envelope.orp, '0.4')
  assert.equal(envelope.type, 'decline')
  assert.equal(envelope.body.reason, 'halted_by_human')
  assert.equal(envelope.body.placements_inserted, 0)
  assert.equal(envelope.body.cash_moved, 0)

  const reset = resetSimulation(state)
  assert.equal(reset.stage, 'ask')
  assert.equal(reset.countered, false)
})
