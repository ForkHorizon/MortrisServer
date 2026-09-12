import type { PuzzleReplayStep } from '../api/houseTypes'
import type {
  MarkerCategory,
  MarkerShape,
  PacingInteraction,
  PacingMarker,
  PacingSpan,
  PacingTimelineData,
  PacingWaveBoundary,
} from './pacingTypes'

export const PAUSE_GAP_CAP_MS = 30 * 60 * 1000 // 30 minutes, matching puzzle_diagnostics.go

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '0s'
  const totalSeconds = Math.round(ms / 1000)
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remMinutes = minutes % 60
  return remMinutes > 0 ? `${hours}h ${remMinutes}m` : `${hours}h`
}

export function classifyMarker(step: PuzzleReplayStep): {
  category: MarkerCategory
  shape: MarkerShape
  icon: string
  label: string
} {
  const blk = step.block_id >= 0 ? `Detail ${step.block_id}` : 'Detail'
  if (step.name === 'placement_resolved') {
    if (step.outcome === 'placed') {
      return { category: 'success', shape: 'circle', icon: '✓', label: `${blk} placed` }
    }
    if (step.outcome === 'returned') {
      return { category: 'release', shape: 'dot', icon: '•', label: `${blk} returned to inventory` }
    }
    if (step.outcome === 'fell_missing_support') {
      return { category: 'fall_missing_support', shape: 'triangle-down', icon: '▼', label: `${blk} fell (missing support)` }
    }
    if (step.outcome === 'fell_no_snap_target') {
      return { category: 'fell_no_snap_target', shape: 'triangle-up', icon: '▲', label: `${blk} fell (no snap target)` }
    }
    return { category: 'fall_other', shape: 'triangle-down', icon: '▼', label: `${blk} fell (${step.outcome})` }
  }
  if (step.name === 'hint_used') return { category: 'hint', shape: 'pin', icon: '💡', label: 'Hint requested' }
  if (step.name === 'state_checkpoint') return { category: 'checkpoint', shape: 'square', icon: '■', label: 'State checkpoint' }
  if (step.name === 'attempt_recovered') return { category: 'recovery', shape: 'square', icon: '⚑', label: 'Attempt recovered' }
  if (step.name.startsWith('developer_') || step.origin === 'developer_menu') {
    const cmd = step.developer_command || step.developer_action_id || step.name.replace(/_/g, ' ')
    return { category: 'developer', shape: 'gear', icon: '⚙', label: `Dev: ${cmd}` }
  }
  if (step.name === 'detail_taken') return { category: 'take', shape: 'dot', icon: '•', label: `${blk} picked up` }
  if (step.name === 'detail_released') return { category: 'release', shape: 'dot', icon: '•', label: `${blk} released` }
  if (step.name === 'wave_completed') return { category: 'success', shape: 'circle', icon: '★', label: `Wave ${Math.max(1, step.wave_index + 1)} completed` }
  if (step.name === 'house_completed') return { category: 'success', shape: 'circle', icon: '★', label: 'House completed' }
  return { category: 'generic', shape: 'dot', icon: '•', label: step.name.replace(/_/g, ' ') }
}

interface StepTimes {
  wallOffsets: number[]
  activeTimes: number[]
  virtualTimes: number[]
  totalVirtualMs: number
}

function parseStepTime(at?: string): number | null {
  if (!at) return null
  const t = new Date(at).getTime()
  return Number.isFinite(t) ? t : null
}

function computeStepTimes(steps: PuzzleReplayStep[]): StepTimes {
  const n = steps.length
  if (n === 0) return { wallOffsets: [], activeTimes: [], virtualTimes: [], totalVirtualMs: 0 }
  const firstT = parseStepTime(steps[0]?.at)
  const t0 = firstT != null ? firstT : (steps.map((s) => parseStepTime(s.at)).find((t) => t != null) ?? 0)

  let lastObservedActive = 0
  const cleanActiveRaw = steps.map((s) => {
    const raw = Number.isFinite(s.active_elapsed_ms) && (s.active_elapsed_ms as number) >= 0
      ? (s.active_elapsed_ms as number)
      : lastObservedActive
    lastObservedActive = Math.max(lastObservedActive, raw)
    return lastObservedActive
  })
  const a0 = cleanActiveRaw[0] || 0
  const activeTimes = cleanActiveRaw.map((v) => Math.max(0, v - a0))

  let lastWall = 0
  const wallOffsets = steps.map((s) => {
    const t = parseStepTime(s.at)
    if (t != null) {
      lastWall = Math.max(lastWall, Math.max(0, t - t0))
    }
    return lastWall
  })

  const virtualTimes = [0]
  let currentVirtual = 0
  for (let i = 1; i < n; i++) {
    const wallDelta = Math.max(0, wallOffsets[i] - wallOffsets[i - 1])
    const activeDelta = Math.max(0, activeTimes[i] - activeTimes[i - 1])
    const isPaused = steps[i - 1].name === 'app_backgrounded' || (wallDelta > 2000 && activeDelta === 0)
    if (isPaused) {
      const pauseDuration = Math.max(0, wallDelta - activeDelta)
      // Visually compress pause so it stays visible as a gap without dominating active play
      const visualPause = Math.min(pauseDuration, 15000)
      currentVirtual += activeDelta + visualPause
    } else {
      currentVirtual += activeDelta > 0 ? activeDelta : Math.min(wallDelta, 1000)
    }
    virtualTimes.push(currentVirtual)
  }
  return { wallOffsets, activeTimes, virtualTimes, totalVirtualMs: currentVirtual }
}

function buildSpans(steps: PuzzleReplayStep[], times: StepTimes): PacingSpan[] {
  const spans: PacingSpan[] = []
  if (steps.length === 0) return spans
  const { wallOffsets, activeTimes, virtualTimes, totalVirtualMs } = times
  const denom = totalVirtualMs > 0 ? totalVirtualMs : 1
  let spanStartIdx = 0
  const initialWallDelta = steps.length > 1 ? Math.max(0, wallOffsets[1] - wallOffsets[0]) : 0
  const initialActiveDelta = steps.length > 1 ? Math.max(0, activeTimes[1] - activeTimes[0]) : 0
  let isCurrentSpanPaused = steps.length > 1 && (steps[0].name === 'app_backgrounded' || (initialWallDelta > 2000 && initialActiveDelta === 0))

  for (let i = 1; i < steps.length; i++) {
    const wallDelta = Math.max(0, wallOffsets[i] - wallOffsets[i - 1])
    const activeDelta = Math.max(0, activeTimes[i] - activeTimes[i - 1])
    const isStepPaused = steps[i - 1].name === 'app_backgrounded' || (wallDelta > 2000 && activeDelta === 0)
    if (isStepPaused !== isCurrentSpanPaused) {
      if (i - 1 > spanStartIdx) {
        spans.push({
          type: isCurrentSpanPaused ? 'paused' : 'active',
          startMs: virtualTimes[spanStartIdx],
          endMs: virtualTimes[i - 1],
          durationMs: virtualTimes[i - 1] - virtualTimes[spanStartIdx],
          wallDurationMs: wallOffsets[i - 1] - wallOffsets[spanStartIdx],
          startPct: (virtualTimes[spanStartIdx] / denom) * 100,
          endPct: (virtualTimes[i - 1] / denom) * 100,
          startIndex: spanStartIdx,
          endIndex: i - 1,
          capped: isCurrentSpanPaused && wallOffsets[i - 1] - wallOffsets[spanStartIdx] > PAUSE_GAP_CAP_MS,
        })
      }
      spanStartIdx = i - 1
      isCurrentSpanPaused = isStepPaused
    }
  }
  const lastIdx = steps.length - 1
  spans.push({
    type: isCurrentSpanPaused ? 'paused' : 'active',
    startMs: virtualTimes[spanStartIdx],
    endMs: virtualTimes[lastIdx],
    durationMs: virtualTimes[lastIdx] - virtualTimes[spanStartIdx],
    wallDurationMs: wallOffsets[lastIdx] - wallOffsets[spanStartIdx],
    startPct: (virtualTimes[spanStartIdx] / denom) * 100,
    endPct: 100,
    startIndex: spanStartIdx,
    endIndex: lastIdx,
    capped: isCurrentSpanPaused && wallOffsets[lastIdx] - wallOffsets[spanStartIdx] > PAUSE_GAP_CAP_MS,
  })
  return spans
}

function buildMarkers(steps: PuzzleReplayStep[], times: StepTimes): PacingMarker[] {
  const { wallOffsets, activeTimes, virtualTimes, totalVirtualMs } = times
  const denom = totalVirtualMs > 0 ? totalVirtualMs : (steps.length > 1 ? steps.length - 1 : 1)
  return steps.map((step, i) => {
    const { category, shape, icon, label } = classifyMarker(step)
    const pct = totalVirtualMs > 0 ? (virtualTimes[i] / denom) * 100 : (i / denom) * 100
    const isTerm = step.name === 'placement_resolved' || step.name === 'detail_returned' || step.name === 'interaction_abandoned'
    const ariaLabel = `Step ${i + 1}: ${label} at ${formatDuration(activeTimes[i])} active play (+${formatDuration(wallOffsets[i])} wall clock)`
    const tooltip = `${label} · Step ${i + 1} · ${formatDuration(activeTimes[i])} active`
    return {
      stepIndex: i, name: step.name, category, shape, pct, activeMs: activeTimes[i], wallMs: wallOffsets[i],
      at: step.at, label, icon, ariaLabel, tooltip, blockId: step.block_id, targetId: step.target_id,
      outcome: step.outcome, isInteractionTerminal: isTerm,
    }
  })
}

export function extractInteractions(steps: PuzzleReplayStep[], markers: PacingMarker[]): PacingInteraction[] {
  const map = new Map<string, { startIdx: number; blockId: number }>()
  const interactions: PacingInteraction[] = []
  steps.forEach((step, i) => {
    if (!step.interaction_id) return
    if (step.name === 'detail_taken') {
      map.set(step.interaction_id, { startIdx: i, blockId: step.block_id })
    } else if (map.has(step.interaction_id) && (step.name === 'placement_resolved' || step.name === 'detail_returned' || step.name === 'interaction_abandoned')) {
      const open = map.get(step.interaction_id)!
      map.delete(step.interaction_id)
      const startMarker = markers[open.startIdx]
      const endMarker = markers[i]
      const activeDurationMs = startMarker && endMarker ? Math.max(0, endMarker.activeMs - startMarker.activeMs) : 0
      interactions.push({
        interactionId: step.interaction_id,
        blockId: open.blockId >= 0 ? open.blockId : -1,
        startStepIndex: open.startIdx,
        endStepIndex: i,
        startPct: startMarker ? startMarker.pct : 0,
        endPct: endMarker ? endMarker.pct : 0,
        activeDurationMs,
        outcome: step.outcome || step.name,
        completed: step.name === 'placement_resolved' && step.outcome === 'placed',
      })
    }
  })
  return interactions
}

function buildWaveBoundaries(steps: PuzzleReplayStep[], markers: PacingMarker[]): PacingWaveBoundary[] {
  const boundaries: PacingWaveBoundary[] = []
  let lastWave = -1
  steps.forEach((step, i) => {
    if (step.wave_index !== lastWave) {
      lastWave = step.wave_index
      const marker = markers[i]
      boundaries.push({
        waveIndex: step.wave_index,
        stepIndex: i,
        pct: marker ? marker.pct : 0,
        label: `Wave ${step.wave_index + 1}`,
      })
    }
  })
  return boundaries
}

export function buildPacingTimeline(steps: PuzzleReplayStep[]): PacingTimelineData {
  if (!steps || steps.length === 0) {
    return {
      spans: [], markers: [], interactions: [], waveBoundaries: [],
      totalActiveMs: 0, totalWallMs: 0, totalRenderedMs: 0,
      hasPauses: false, pauseCount: 0, totalPauseMs: 0,
    }
  }
  const times = computeStepTimes(steps)
  const spans = buildSpans(steps, times)
  const markers = buildMarkers(steps, times)
  const interactions = extractInteractions(steps, markers)
  const waveBoundaries = buildWaveBoundaries(steps, markers)
  const pausedSpans = spans.filter((s) => s.type === 'paused')
  const totalPauseMs = pausedSpans.reduce((acc, s) => acc + s.wallDurationMs, 0)
  const totalActiveMs = times.activeTimes[times.activeTimes.length - 1] || 0
  const totalWallMs = times.wallOffsets[times.wallOffsets.length - 1] || 0

  return {
    spans, markers, interactions, waveBoundaries,
    totalActiveMs, totalWallMs, totalRenderedMs: times.totalVirtualMs,
    hasPauses: pausedSpans.length > 0, pauseCount: pausedSpans.length, totalPauseMs,
  }
}

export function findNearestStepIndex(pct: number, timeline: PacingTimelineData): number {
  if (!timeline.markers || timeline.markers.length === 0) return 0
  let nearestIdx = 0
  let minDiff = Infinity
  for (const marker of timeline.markers) {
    const diff = Math.abs(marker.pct - pct)
    if (diff < minDiff) {
      minDiff = diff
      nearestIdx = marker.stepIndex
    }
  }
  return nearestIdx
}
