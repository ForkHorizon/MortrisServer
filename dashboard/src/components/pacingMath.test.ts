import test from 'node:test'
import assert from 'node:assert/strict'
import type { PuzzleReplayStep } from '../api/houseTypes.ts'
import {
  buildPacingTimeline,
  classifyMarker,
  extractInteractions,
  findNearestStepIndex,
  formatDuration,
  PAUSE_GAP_CAP_MS,
} from './pacingMath.ts'

function makeStep(overrides: Partial<PuzzleReplayStep> = {}): PuzzleReplayStep {
  return {
    index: 0,
    name: 'wave_attempt_started',
    at: '2026-08-06T12:00:00Z',
    block_id: -1,
    target_id: -1,
    outcome: '',
    rule_state: '',
    wave_index: 0,
    active_elapsed_ms: 0,
    placed: [],
    ...overrides,
  }
}

test('Empty steps returns empty timeline with zero durations', () => {
  const timeline = buildPacingTimeline([])
  assert.strictEqual(timeline.spans.length, 0)
  assert.strictEqual(timeline.markers.length, 0)
  assert.strictEqual(timeline.totalActiveMs, 0)
  assert.strictEqual(timeline.totalWallMs, 0)
  assert.strictEqual(timeline.hasPauses, false)
})

test('Single step returns 0 duration without division by zero', () => {
  const step = makeStep({ index: 0, name: 'wave_attempt_started', active_elapsed_ms: 0 })
  const timeline = buildPacingTimeline([step])
  assert.strictEqual(timeline.markers.length, 1)
  assert.strictEqual(timeline.markers[0].pct, 0)
  assert.strictEqual(timeline.totalActiveMs, 0)
  assert.strictEqual(timeline.totalWallMs, 0)
})

test('Active progression with no pauses distributes percentages linearly by virtual time', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:05Z', active_elapsed_ms: 5000, name: 'detail_taken', block_id: 1 }),
    makeStep({ index: 2, at: '2026-08-06T12:00:10Z', active_elapsed_ms: 10000, name: 'placement_resolved', block_id: 1, outcome: 'placed' }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.hasPauses, false)
  assert.strictEqual(timeline.totalActiveMs, 10000)
  assert.strictEqual(timeline.totalWallMs, 10000)
  assert.strictEqual(timeline.markers[0].pct, 0)
  assert.strictEqual(timeline.markers[1].pct, 50)
  assert.strictEqual(timeline.markers[2].pct, 100)
})

test('Background pauses are detected and distinguished from active play', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:05Z', active_elapsed_ms: 5000, name: 'app_backgrounded' }),
    makeStep({ index: 2, at: '2026-08-06T12:01:05Z', active_elapsed_ms: 5000, name: 'app_foregrounded' }),
    makeStep({ index: 3, at: '2026-08-06T12:01:10Z', active_elapsed_ms: 10000, name: 'wave_completed' }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.hasPauses, true)
  assert.strictEqual(timeline.pauseCount, 1)
  assert.strictEqual(timeline.totalActiveMs, 10000)
  assert.strictEqual(timeline.totalWallMs, 70000) // 5s + 60s pause + 5s = 70s
  assert.strictEqual(timeline.totalPauseMs, 60000)
  const pausedSpan = timeline.spans.find((s) => s.type === 'paused')
  assert.ok(pausedSpan)
  assert.strictEqual(pausedSpan.wallDurationMs, 60000)
})

test('Multi-hour background gaps are capped at 30 minutes', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:10Z', active_elapsed_ms: 10000, name: 'app_backgrounded' }),
    // 4 hours in background
    makeStep({ index: 2, at: '2026-08-06T16:00:10Z', active_elapsed_ms: 10000, name: 'app_foregrounded' }),
    makeStep({ index: 3, at: '2026-08-06T16:00:20Z', active_elapsed_ms: 20000, name: 'house_completed' }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.hasPauses, true)
  const pausedSpan = timeline.spans.find((s) => s.type === 'paused')
  assert.ok(pausedSpan)
  assert.strictEqual(pausedSpan.capped, true)
  assert.strictEqual(PAUSE_GAP_CAP_MS, 1800000)
  assert.ok(pausedSpan.wallDurationMs > PAUSE_GAP_CAP_MS)
  assert.strictEqual(pausedSpan.wallDurationMs, 4 * 3600 * 1000)
})

test('extractInteractions helper isolates completed chains from markers', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, name: 'detail_taken', block_id: 5, interaction_id: 'i-5' }),
    makeStep({ index: 1, name: 'placement_resolved', block_id: 5, outcome: 'placed', interaction_id: 'i-5' }),
  ]
  const timeline = buildPacingTimeline(steps)
  const result = extractInteractions(steps, timeline.markers)
  assert.strictEqual(result.length, 1)
  assert.strictEqual(result[0].blockId, 5)
  assert.strictEqual(result[0].completed, true)
})

test('Interaction chains pair take, release, and resolution', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:02Z', active_elapsed_ms: 2000, name: 'detail_taken', block_id: 12, interaction_id: 'intr-1' }),
    makeStep({ index: 2, at: '2026-08-06T12:00:04Z', active_elapsed_ms: 4000, name: 'detail_released', block_id: 12, target_id: 12, interaction_id: 'intr-1' }),
    makeStep({ index: 3, at: '2026-08-06T12:00:05Z', active_elapsed_ms: 5000, name: 'placement_resolved', block_id: 12, target_id: 12, outcome: 'fell_missing_support', interaction_id: 'intr-1' }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.interactions.length, 1)
  const chain = timeline.interactions[0]
  assert.strictEqual(chain.interactionId, 'intr-1')
  assert.strictEqual(chain.blockId, 12)
  assert.strictEqual(chain.startStepIndex, 1)
  assert.strictEqual(chain.endStepIndex, 3)
  assert.strictEqual(chain.activeDurationMs, 3000) // 5000 - 2000
  assert.strictEqual(chain.outcome, 'fell_missing_support')
  assert.strictEqual(chain.completed, false)
})

test('Marker classification handles falls, hints, checkpoints and developer commands', () => {
  const successStep = makeStep({ name: 'placement_resolved', outcome: 'placed', block_id: 7 })
  const fallSuppStep = makeStep({ name: 'placement_resolved', outcome: 'fell_missing_support', block_id: 8 })
  const fallSnapStep = makeStep({ name: 'placement_resolved', outcome: 'fell_no_snap_target', block_id: 9 })
  const hintStep = makeStep({ name: 'hint_used' })
  const checkpointStep = makeStep({ name: 'state_checkpoint' })
  const recoveryStep = makeStep({ name: 'attempt_recovered' })
  const devStep = makeStep({ name: 'developer_command_started', developer_command: 'CompleteHouse99', origin: 'developer_menu' })

  assert.strictEqual(classifyMarker(successStep).category, 'success')
  assert.strictEqual(classifyMarker(fallSuppStep).category, 'fall_missing_support')
  assert.strictEqual(classifyMarker(fallSnapStep).category, 'fell_no_snap_target')
  assert.strictEqual(classifyMarker(hintStep).category, 'hint')
  assert.strictEqual(classifyMarker(checkpointStep).category, 'checkpoint')
  assert.strictEqual(classifyMarker(recoveryStep).category, 'recovery')
  assert.strictEqual(classifyMarker(devStep).category, 'developer')
  assert.ok(classifyMarker(devStep).label.includes('CompleteHouse99'))
})

test('Wave boundaries detect wave changes across an attempt', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, wave_index: 0 }),
    makeStep({ index: 1, wave_index: 0, name: 'wave_completed' }),
    makeStep({ index: 2, wave_index: 1, name: 'wave_attempt_started' }),
    makeStep({ index: 3, wave_index: 1, name: 'wave_completed' }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.waveBoundaries.length, 2)
  assert.strictEqual(timeline.waveBoundaries[0].waveIndex, 0)
  assert.strictEqual(timeline.waveBoundaries[1].waveIndex, 1)
})

test('findNearestStepIndex locates closest step for scrubbing', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:10Z', active_elapsed_ms: 10000 }),
    makeStep({ index: 2, at: '2026-08-06T12:00:20Z', active_elapsed_ms: 20000 }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(findNearestStepIndex(0, timeline), 0)
  assert.strictEqual(findNearestStepIndex(48, timeline), 1)
  assert.strictEqual(findNearestStepIndex(95, timeline), 2)
})

test('formatDuration produces human-readable times', () => {
  assert.strictEqual(formatDuration(0), '0s')
  assert.strictEqual(formatDuration(12400), '12s')
  assert.strictEqual(formatDuration(75000), '1m 15s')
  assert.strictEqual(formatDuration(3600000), '1h')
  assert.strictEqual(formatDuration(3660000), '1h 1m')
})

test('Edge cases: overlapping timestamps and zero active duration do not produce NaN', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0, name: 'detail_taken', block_id: 1 }),
    makeStep({ index: 2, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0, name: 'placement_resolved', block_id: 1 }),
  ]
  const timeline = buildPacingTimeline(steps)
  assert.strictEqual(timeline.markers.length, 3)
  for (const m of timeline.markers) {
    assert.ok(!Number.isNaN(m.pct), `Marker pct is NaN: ${m.pct}`)
  }
})

test('Orphan interactions and abandoned interactions are handled gracefully', () => {
  const steps: PuzzleReplayStep[] = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0, name: 'detail_taken', block_id: 2, interaction_id: 'orphan-1' }),
    makeStep({ index: 1, at: '2026-08-06T12:00:02Z', active_elapsed_ms: 2000, name: 'detail_taken', block_id: 3, interaction_id: 'aband-1' }),
    makeStep({ index: 2, at: '2026-08-06T12:00:04Z', active_elapsed_ms: 4000, name: 'interaction_abandoned', block_id: 3, interaction_id: 'aband-1' }),
  ]
  const timeline = buildPacingTimeline(steps)
  // orphan-1 was never resolved, aband-1 was terminated with interaction_abandoned
  assert.strictEqual(timeline.interactions.length, 1)
  assert.strictEqual(timeline.interactions[0].interactionId, 'aband-1')
  assert.strictEqual(timeline.interactions[0].outcome, 'interaction_abandoned')
  assert.strictEqual(timeline.interactions[0].completed, false)
})

test('Audit fixes: placement_resolved returned, non-zero active start, initial pause, formatDuration NaN', () => {
  // 1. returned outcome
  const returnedStep = makeStep({ name: 'placement_resolved', outcome: 'returned', block_id: 5 })
  const returnedMarker = classifyMarker(returnedStep)
  assert.strictEqual(returnedMarker.category, 'release')
  assert.strictEqual(returnedMarker.icon, '•')
  assert.strictEqual(returnedMarker.label, 'Detail 5 returned to inventory')

  // 2. formatDuration with NaN or negative
  assert.strictEqual(formatDuration(NaN), '0s')
  assert.strictEqual(formatDuration(-100), '0s')

  // 3. active_elapsed_ms starting at high cumulative value
  const stepsCumulative = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 60000 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:10Z', active_elapsed_ms: 70000 }),
  ]
  const timelineCum = buildPacingTimeline(stepsCumulative)
  assert.strictEqual(timelineCum.totalActiveMs, 10000)
  assert.strictEqual(timelineCum.totalWallMs, 10000)

  // 4. Initial interval is paused: no phantom 0-length active span
  const stepsPauseStart = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0, name: 'app_backgrounded' }),
    makeStep({ index: 1, at: '2026-08-06T12:05:00Z', active_elapsed_ms: 0, name: 'app_foregrounded' }),
  ]
  const timelinePause = buildPacingTimeline(stepsPauseStart)
  assert.strictEqual(timelinePause.spans.length, 1)
  assert.strictEqual(timelinePause.spans[0].type, 'paused')

  // 5. active_elapsed_ms omitted (-1 or undefined): active time should not reset to 0
  const stepsMissingActive = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 5000 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:05Z', active_elapsed_ms: -1, name: 'state_checkpoint' }),
    makeStep({ index: 2, at: '2026-08-06T12:00:10Z', active_elapsed_ms: 8000 }),
  ]
  const timelineMissingActive = buildPacingTimeline(stepsMissingActive)
  assert.strictEqual(timelineMissingActive.markers[1].activeMs, 0)
  assert.strictEqual(timelineMissingActive.markers[2].activeMs, 3000)
  assert.strictEqual(timelineMissingActive.totalActiveMs, 3000)
})

test('Audit fixes: malformed date and Round 3 Infinity / sentinel block ID', () => {
  // 1. Malformed date string does not produce 56-year pause
  const stepsMalformedDate = [
    makeStep({ index: 0, at: 'invalid-date', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:02Z', active_elapsed_ms: 2000 }),
  ]
  const timelineMalformed = buildPacingTimeline(stepsMalformedDate)
  assert.ok(timelineMalformed.totalWallMs < 100000)

  // 2. Infinity in active_elapsed_ms does not poison timeline into NaN
  const stepsInfinity = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0 }),
    makeStep({ index: 1, at: '2026-08-06T12:00:02Z', active_elapsed_ms: Infinity }),
    makeStep({ index: 2, at: '2026-08-06T12:00:04Z', active_elapsed_ms: 4000 }),
  ]
  const timelineInf = buildPacingTimeline(stepsInfinity)
  assert.strictEqual(Number.isNaN(timelineInf.totalActiveMs), false)
  assert.strictEqual(timelineInf.markers.every((m) => !Number.isNaN(m.pct)), true)

  // 3. Sentinel block ID -1 in interaction does not crash and preserves valid range
  const stepsSentinel = [
    makeStep({ index: 0, at: '2026-08-06T12:00:00Z', active_elapsed_ms: 0, name: 'detail_taken', block_id: -1, interaction_id: 'sent-1' }),
    makeStep({ index: 1, at: '2026-08-06T12:00:02Z', active_elapsed_ms: 2000, name: 'interaction_abandoned', block_id: -1, interaction_id: 'sent-1' }),
  ]
  const timelineSentinel = buildPacingTimeline(stepsSentinel)
  assert.strictEqual(timelineSentinel.interactions.length, 1)
  assert.strictEqual(timelineSentinel.interactions[0].blockId, -1)

  // 4. Sentinel block ID -1 in classifyMarker renders 'Detail' instead of 'Detail -1'
  const stepPlacedSentinel = makeStep({ name: 'placement_resolved', outcome: 'placed', block_id: -1 })
  assert.strictEqual(classifyMarker(stepPlacedSentinel).label, 'Detail placed')
  const stepFallSentinel = makeStep({ name: 'placement_resolved', outcome: 'fell_missing_support', block_id: -1 })
  assert.strictEqual(classifyMarker(stepFallSentinel).label, 'Detail fell (missing support)')
})

