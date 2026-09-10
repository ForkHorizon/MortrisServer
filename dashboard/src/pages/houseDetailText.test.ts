import test from 'node:test'
import assert from 'node:assert/strict'
import type { PuzzleHouseBlock, PuzzleRetryLadder } from '../api/houseTypes.ts'
import { detailDiagnosis, detailDiagnosisTone, leadingFallReason } from '../components/detailDiagnosis.ts'

function makeLadder(sampleCount: number, overrides: Partial<PuzzleRetryLadder> = {}): PuzzleRetryLadder {
  return {
    sample_count: sampleCount,
    success_1st_try: 0, success_2nd_try: 0, success_3rd_try: 0, success_4th_plus_try: 0,
    never_succeeded: 0, median_tries: 1, p75_tries: 0, p90_tries: 0,
    reliable: sampleCount >= 5, ...overrides,
  }
}

function makeBlock(overrides: Partial<PuzzleHouseBlock> = {}): PuzzleHouseBlock {
  return {
    block_id: 1, wave_index: 0, order_in_layer: 1, is_ground: false, visual_key: 'test_block',
    required_groups: [], placements: 10, falls: 0, fall_rate: 0, falls_by_reason: {},
    rate_is_reliable: true, first_try_failures: 0, first_try_attempts: 10,
    first_try_failure_rate: 0, first_try_reliable: true, no_snap_rate: 0,
    missing_support_rate: 0, hint_pressure_count: 0, hint_pressure_rate: 0,
    median_tries_to_success: 1, successful_placements: 10, median_time_to_place_ms: 500,
    time_to_place_samples: 10, ...overrides,
  }
}

test('Rule 1: Insufficient Evidence for unplayed and low-sample blocks', () => {
  const unplayed = makeBlock({ placements: 0, rate_is_reliable: false })
  assert.strictEqual(detailDiagnosis(unplayed), 'There is not enough evidence yet.')
  assert.strictEqual(detailDiagnosisTone(unplayed), 'neutral')

  const onePlay = makeBlock({ placements: 1, rate_is_reliable: false })
  assert.strictEqual(detailDiagnosis(onePlay), 'Only 1 play so far — not enough evidence to diagnose friction.')
  assert.strictEqual(detailDiagnosisTone(onePlay), 'neutral')

  const threePlays = makeBlock({ placements: 3, rate_is_reliable: false })
  assert.strictEqual(detailDiagnosis(threePlays), 'Only 3 plays so far — not enough evidence to diagnose friction.')

  const ladderUnder5 = makeBlock({
    placements: 8,
    rate_is_reliable: true,
    retry_ladder: makeLadder(2, { success_1st_try: 2, reliable: false }),
  })
  assert.strictEqual(detailDiagnosis(ladderUnder5), 'Only 2 plays so far — not enough evidence to diagnose friction.')
})

test('Rule 2: Clean / Smooth First-Try placement', () => {
  const clean = makeBlock({ placements: 10, fall_rate: 0.10, median_tries_to_success: 1 })
  assert.strictEqual(detailDiagnosis(clean), 'The detail is placed cleanly on the first try with minimal friction.')
  assert.strictEqual(detailDiagnosisTone(clean), 'good')

  const boundary = makeBlock({ placements: 5, fall_rate: 0.14, median_tries_to_success: 1 })
  assert.strictEqual(detailDiagnosis(boundary), 'The detail is placed cleanly on the first try with minimal friction.')

  const fallExceeds = makeBlock({ placements: 5, fall_rate: 0.15, median_tries_to_success: 1 })
  assert.strictEqual(detailDiagnosis(fallExceeds), 'The detail shows moderate friction with mixed placement attempts.')
})

test('Rule 3: Missing Support Dominance', () => {
  const byRate = makeBlock({
    placements: 10,
    falls: 4,
    fall_rate: 0.40,
    missing_support_rate: 0.40,
    median_tries_to_success: 2,
    falls_by_reason: { fell_missing_support: 4 },
  })
  assert.strictEqual(detailDiagnosis(byRate), 'Players release close to the right slot but the required support is missing.')
  assert.strictEqual(detailDiagnosisTone(byRate), 'friction')

  const byLeadingReason = makeBlock({
    placements: 10,
    falls: 3,
    fall_rate: 0.30,
    missing_support_rate: 0.20,
    no_snap_rate: 0.10,
    median_tries_to_success: 2,
    falls_by_reason: { fell_missing_support: 2, fell_no_snap_target: 1 },
  })
  assert.strictEqual(detailDiagnosis(byLeadingReason), 'Players release close to the right slot but the required support is missing.')
})

test('Rule 4: No Snap / Scattered Drops Dominance', () => {
  const byRate = makeBlock({
    placements: 10,
    falls: 5,
    fall_rate: 0.50,
    no_snap_rate: 0.50,
    median_tries_to_success: 2,
    falls_by_reason: { fell_no_snap_target: 5 },
  })
  assert.strictEqual(detailDiagnosis(byRate), 'Drops are widely scattered and usually find no snap target.')
  assert.strictEqual(detailDiagnosisTone(byRate), 'friction')

  const byLeadingReason = makeBlock({
    placements: 10,
    falls: 4,
    fall_rate: 0.40,
    no_snap_rate: 0.30,
    missing_support_rate: 0.10,
    median_tries_to_success: 2,
    falls_by_reason: { fell_no_snap_target: 3, fell_missing_support: 1 },
  })
  assert.strictEqual(detailDiagnosis(byLeadingReason), 'Drops are widely scattered and usually find no snap target.')

  const noSnapBeatsMissingRate = makeBlock({
    placements: 10,
    falls: 9,
    fall_rate: 0.90,
    no_snap_rate: 0.50,
    missing_support_rate: 0.40,
    median_tries_to_success: 2,
    falls_by_reason: { fell_no_snap_target: 5, fell_missing_support: 4 },
  })
  assert.strictEqual(detailDiagnosis(noSnapBeatsMissingRate), 'Drops are widely scattered and usually find no snap target.')
})

test('Rule 5: High Retries / Friction Before Success', () => {
  const byMedianTries = makeBlock({
    placements: 10,
    falls: 2,
    fall_rate: 0.20,
    median_tries_to_success: 3,
    successful_placements: 8,
    retry_ladder: makeLadder(8, { median_tries: 3, success_1st_try: 1, success_3rd_try: 3, success_4th_plus_try: 2 }),
  })
  assert.strictEqual(detailDiagnosis(byMedianTries), 'The detail is placed successfully but takes several retries.')
  assert.strictEqual(detailDiagnosisTone(byMedianTries), 'friction')

  const byLadderSkew = makeBlock({
    placements: 10,
    falls: 2,
    fall_rate: 0.20,
    median_tries_to_success: 2,
    successful_placements: 8,
    retry_ladder: makeLadder(8, { median_tries: 2, success_1st_try: 2, success_3rd_try: 2, success_4th_plus_try: 2 }),
  })
  assert.strictEqual(detailDiagnosis(byLadderSkew), 'The detail is placed successfully but takes several retries.')
})

test('Rule 6: High Abandonment / Unplaced', () => {
  const highNeverSucceeded = makeBlock({
    placements: 10,
    falls: 5,
    fall_rate: 0.50,
    missing_support_rate: 0.20,
    no_snap_rate: 0.20,
    median_tries_to_success: 1,
    successful_placements: 5,
    retry_ladder: makeLadder(10, { success_1st_try: 5, never_succeeded: 5 }),
  })
  assert.strictEqual(detailDiagnosis(highNeverSucceeded), 'Players repeatedly drop this detail and frequently fail to place it.')
  assert.strictEqual(detailDiagnosisTone(highNeverSucceeded), 'warning')

  const unrecoveredFallsOnly = makeBlock({
    placements: 8,
    falls: 8,
    fall_rate: 1.0,
    missing_support_rate: 0.25,
    no_snap_rate: 0.25,
    successful_placements: 0,
  })
  assert.strictEqual(detailDiagnosis(unrecoveredFallsOnly), 'Players repeatedly drop this detail and frequently fail to place it.')
  assert.strictEqual(detailDiagnosisTone(unrecoveredFallsOnly), 'warning')
})

test('Rule 7: Hint Pressure', () => {
  const hintHeavy = makeBlock({
    placements: 10,
    falls: 2,
    fall_rate: 0.20,
    hint_pressure_count: 4,
    hint_pressure_rate: 0.40,
    median_tries_to_success: 2,
    successful_placements: 8,
    retry_ladder: makeLadder(8, { median_tries: 2, success_1st_try: 4, success_2nd_try: 4 }),
  })
  assert.strictEqual(detailDiagnosis(hintHeavy), 'Players struggle with this detail and frequently request the blueprint hint.')
  assert.strictEqual(detailDiagnosisTone(hintHeavy), 'friction')
})

test('Rule 8: Default Balanced moderate friction', () => {
  const balanced = makeBlock({
    placements: 10,
    falls: 2,
    fall_rate: 0.20,
    hint_pressure_rate: 0.10,
    median_tries_to_success: 2,
    successful_placements: 8,
    retry_ladder: makeLadder(8, { median_tries: 2, success_1st_try: 4, success_2nd_try: 4 }),
  })
  assert.strictEqual(detailDiagnosis(balanced), 'The detail shows moderate friction with mixed placement attempts.')
  assert.strictEqual(detailDiagnosisTone(balanced), 'friction')
})

test('leadingFallReason helper edge cases', () => {
  assert.strictEqual(leadingFallReason(makeBlock({ falls_by_reason: {} })), null)
  assert.strictEqual(
    leadingFallReason(makeBlock({ falls_by_reason: { fell_missing_support: 5, fell_no_snap_target: 2 } })),
    'fell_missing_support',
  )
  assert.strictEqual(
    leadingFallReason(makeBlock({ falls_by_reason: { fell_missing_support: 1, fell_no_snap_target: 3 } })),
    'fell_no_snap_target',
  )
  const tie = makeBlock({ falls_by_reason: { fell_no_snap_target: 3, fell_missing_support: 3 } })
  assert.strictEqual(leadingFallReason(tie), 'fell_missing_support')
})

test('Rule 1 boundaries: 4 vs 5 samples and unconfirmed rate reliability', () => {
  const fourPlays = makeBlock({ placements: 4, rate_is_reliable: false })
  assert.strictEqual(detailDiagnosis(fourPlays), 'Only 4 plays so far — not enough evidence to diagnose friction.')

  const fiveUnreliable = makeBlock({ placements: 5, rate_is_reliable: false })
  assert.strictEqual(detailDiagnosis(fiveUnreliable), 'There is not enough evidence yet.')
})

test('Starvation prevention: isolated fall does not starve high retries', () => {
  const retriesWithOneFall = makeBlock({
    placements: 10,
    falls: 1,
    fall_rate: 0.10,
    missing_support_rate: 0.10,
    falls_by_reason: { fell_missing_support: 1 },
    median_tries_to_success: 3,
    successful_placements: 9,
    retry_ladder: makeLadder(9, { median_tries: 3, success_3rd_try: 5, success_1st_try: 2 }),
  })
  assert.strictEqual(detailDiagnosis(retriesWithOneFall), 'The detail is placed successfully but takes several retries.')
  assert.strictEqual(detailDiagnosisTone(retriesWithOneFall), 'friction')
})

test('Boundary checks for rates: 0.39 vs 0.40 support and 0.34 vs 0.35 hint', () => {
  const sub40Support = makeBlock({
    placements: 100,
    falls: 39,
    fall_rate: 0.39,
    missing_support_rate: 0.39,
    median_tries_to_success: 2,
    successful_placements: 61,
  })
  assert.strictEqual(detailDiagnosis(sub40Support), 'The detail shows moderate friction with mixed placement attempts.')

  const sub35Hint = makeBlock({
    placements: 100,
    falls: 20,
    fall_rate: 0.20,
    hint_pressure_rate: 0.34,
    median_tries_to_success: 2,
    successful_placements: 80,
  })
  assert.strictEqual(detailDiagnosis(sub35Hint), 'The detail shows moderate friction with mixed placement attempts.')
})

test('Edge cases: undefined falls_by_reason, zero-ladder fallback, and prominence guard', () => {
  // Undefined falls_by_reason
  const noFallsMap = makeBlock({ falls_by_reason: undefined })
  assert.strictEqual(leadingFallReason(noFallsMap), null)

  // Retry ladder with zeros preserves block.median_tries_to_success
  const zeroLadder = makeBlock({
    placements: 10,
    median_tries_to_success: 1,
    fall_rate: 0.05,
    retry_ladder: makeLadder(0, { median_tries: 0, reliable: false }),
  })
  assert.strictEqual(detailDiagnosis(zeroLadder), 'The detail is placed cleanly on the first try with minimal friction.')

  // Conjunction test: 3 falls in 100 drops (fall_rate 0.03 < 0.25) does NOT trigger dominance
  const lowRateManyFalls = makeBlock({
    placements: 100,
    falls: 3,
    fall_rate: 0.03,
    missing_support_rate: 0.03,
    falls_by_reason: { fell_missing_support: 3 },
    median_tries_to_success: 2,
  })
  assert.strictEqual(detailDiagnosis(lowRateManyFalls), 'The detail shows moderate friction with mixed placement attempts.')
})
