import type { KeyboardEvent, PointerEvent, RefObject } from 'react'

export type PacingSpanType = 'active' | 'paused'

export interface PacingSpan {
  type: PacingSpanType
  startMs: number
  endMs: number
  durationMs: number
  wallDurationMs: number
  startPct: number
  endPct: number
  startIndex: number
  endIndex: number
  capped: boolean
}

export type MarkerCategory =
  | 'success'
  | 'fall_missing_support'
  | 'fell_no_snap_target'
  | 'fall_other'
  | 'hint'
  | 'checkpoint'
  | 'recovery'
  | 'developer'
  | 'take'
  | 'release'
  | 'generic'

export type MarkerShape =
  | 'circle'
  | 'triangle-down'
  | 'triangle-up'
  | 'diamond'
  | 'square'
  | 'pin'
  | 'gear'
  | 'dot'

export interface PacingMarker {
  stepIndex: number
  name: string
  category: MarkerCategory
  shape: MarkerShape
  pct: number
  activeMs: number
  wallMs: number
  at: string
  label: string
  icon: string
  ariaLabel: string
  tooltip: string
  blockId: number
  targetId: number
  outcome: string
  isInteractionTerminal: boolean
}

export interface PacingInteraction {
  interactionId: string
  blockId: number
  startStepIndex: number
  endStepIndex: number
  startPct: number
  endPct: number
  activeDurationMs: number
  outcome: string
  completed: boolean
}

export interface PacingWaveBoundary {
  waveIndex: number
  stepIndex: number
  pct: number
  label: string
}

export interface PacingTimelineData {
  spans: PacingSpan[]
  markers: PacingMarker[]
  interactions: PacingInteraction[]
  waveBoundaries: PacingWaveBoundary[]
  totalActiveMs: number
  totalWallMs: number
  totalRenderedMs: number
  hasPauses: boolean
  pauseCount: number
  totalPauseMs: number
}

export interface PacingTrackProps {
  trackRef: RefObject<HTMLDivElement | null>
  timelineId: string
  totalSteps: number
  currentIndex: number
  playheadPct: number
  timeline: PacingTimelineData
  onPointerDown: (e: PointerEvent<HTMLDivElement>) => void
  onPointerMove: (e: PointerEvent<HTMLDivElement>) => void
  onPointerUp: (e: PointerEvent<HTMLDivElement>) => void
  onPointerCancel: (e: PointerEvent<HTMLDivElement>) => void
  onKeyDown: (e: KeyboardEvent<HTMLDivElement>) => void
  onSelectStep: (idx: number) => void
  onHoverMarker: (m: PacingMarker | null) => void
}
