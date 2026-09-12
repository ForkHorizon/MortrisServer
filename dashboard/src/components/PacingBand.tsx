import { useEffect, useId, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent, PointerEvent } from 'react'
import type { PuzzleReplay, PuzzleReplayStep } from '../api/houseTypes'
import { buildPacingTimeline, findNearestStepIndex, formatDuration } from './pacingMath'
import type { PacingMarker, PacingTimelineData, PacingTrackProps } from './pacingTypes'

interface PacingBandProps {
  replay: PuzzleReplay
  currentIndex: number
  onSelectStep: (index: number) => void
}

function handleKeyDownNav(currentIndex: number, stepsLength: number, onSelectStep: (idx: number) => void) {
  return (e: KeyboardEvent<HTMLDivElement>) => {
    const jump = Math.max(1, Math.round(stepsLength / 10))
    if (e.key === 'ArrowRight' || e.key === 'ArrowUp') { e.preventDefault(); if (currentIndex < stepsLength - 1) onSelectStep(currentIndex + 1) }
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') { e.preventDefault(); if (currentIndex > 0) onSelectStep(currentIndex - 1) }
    else if (e.key === 'PageUp') { e.preventDefault(); onSelectStep(Math.min(stepsLength - 1, currentIndex + jump)) }
    else if (e.key === 'PageDown') { e.preventDefault(); onSelectStep(Math.max(0, currentIndex - jump)) }
    else if (e.key === 'Home') { e.preventDefault(); onSelectStep(0) }
    else if (e.key === 'End') { e.preventDefault(); onSelectStep(stepsLength - 1) }
  }
}

function usePacingScrubber(stepsLength: number, timeline: PacingTimelineData, onSelectStep: (index: number) => void) {
  const trackRef = useRef<HTMLDivElement>(null)
  const isDraggingRef = useRef(false)

  const seekToClientX = (clientX: number) => {
    if (!trackRef.current) return
    const rect = trackRef.current.getBoundingClientRect()
    if (rect.width <= 0) return
    const pct = Math.max(0, Math.min(100, ((clientX - rect.left) / rect.width) * 100))
    onSelectStep(findNearestStepIndex(pct, timeline))
  }

  const handlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return
    isDraggingRef.current = true
    try {
      e.currentTarget.setPointerCapture(e.pointerId)
    } catch {
      // safe fallback on platforms without pointer capture support
    }
    seekToClientX(e.clientX)
  }

  const handlePointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (isDraggingRef.current) seekToClientX(e.clientX)
  }

  const handlePointerEnd = (e: PointerEvent<HTMLDivElement>) => {
    if (!isDraggingRef.current) return
    isDraggingRef.current = false
    try {
      e.currentTarget.releasePointerCapture(e.pointerId)
    } catch {
      // capture may already be released
    }
  }

  return {
    trackRef,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp: handlePointerEnd,
    handlePointerCancel: handlePointerEnd,
    handleKeyDown: (i: number) => handleKeyDownNav(i, stepsLength, onSelectStep),
  }
}

function PacingTrack(p: PacingTrackProps) {
  const currentMarker = p.timeline.markers[p.currentIndex]
  const ariaValueText = currentMarker
    ? `Step ${p.currentIndex + 1} of ${p.totalSteps}: ${currentMarker.label} (${formatDuration(currentMarker.activeMs)} active)`
    : `Step ${p.currentIndex + 1} of ${p.totalSteps}`

  return (
    <div
      ref={p.trackRef}
      className="pacing-track"
      id={p.timelineId}
      tabIndex={0}
      role="slider"
      aria-orientation="horizontal"
      aria-label="Replay pacing scrubber"
      aria-valuemin={0}
      aria-valuemax={p.totalSteps - 1}
      aria-valuenow={p.currentIndex}
      aria-valuetext={ariaValueText}
      onPointerDown={p.onPointerDown}
      onPointerMove={p.onPointerMove}
      onPointerUp={p.onPointerUp}
      onPointerCancel={p.onPointerCancel}
      onKeyDown={p.onKeyDown}
    >
      <PacingSpansLayer timeline={p.timeline} />
      <PacingWavesLayer timeline={p.timeline} />
      <PacingInteractionsLayer timeline={p.timeline} currentIndex={p.currentIndex} />
      <PacingMarkersLayer
        timeline={p.timeline}
        currentIndex={p.currentIndex}
        onSelectStep={p.onSelectStep}
        onHoverMarker={p.onHoverMarker}
      />
      <div className="pacing-playhead" style={{ left: `${p.playheadPct}%` }}>
        <div className="pacing-playhead-cap" />
      </div>
    </div>
  )
}

export function PacingBand({ replay, currentIndex, onSelectStep }: PacingBandProps) {
  const timelineId = useId()
  const [hoveredMarker, setHoveredMarker] = useState<PacingMarker | null>(null)
  useEffect(() => setHoveredMarker(null), [currentIndex])
  const timeline = useMemo(() => buildPacingTimeline(replay.steps), [replay.steps])
  const handleSelect = (idx: number) => {
    setHoveredMarker(null)
    onSelectStep(idx)
  }
  const scrubber = usePacingScrubber(replay.steps.length, timeline, handleSelect)

  if (replay.steps.length === 0) return null
  const currentMarker = timeline.markers[currentIndex]
  const playheadPct = currentMarker ? currentMarker.pct : 0

  return (
    <div className="pacing-band-container" role="region" aria-label="Attempt pacing timeline">
      <PacingSummary timeline={timeline} />
      <PacingTrack
        trackRef={scrubber.trackRef}
        timelineId={timelineId}
        totalSteps={replay.steps.length}
        currentIndex={currentIndex}
        playheadPct={playheadPct}
        timeline={timeline}
        onPointerDown={scrubber.handlePointerDown}
        onPointerMove={scrubber.handlePointerMove}
        onPointerUp={scrubber.handlePointerUp}
        onPointerCancel={scrubber.handlePointerCancel}
        onKeyDown={scrubber.handleKeyDown(currentIndex)}
        onSelectStep={handleSelect}
        onHoverMarker={setHoveredMarker}
      />
      <PacingTooltip hovered={hoveredMarker} currentMarker={currentMarker} currentStep={replay.steps[currentIndex]} />
      <PacingLegend hasDeveloper={replay.steps.some((s) => s.origin === 'developer_menu' || s.name.startsWith('developer_'))} />
    </div>
  )
}

function PacingSummary({ timeline }: { timeline: PacingTimelineData }) {
  const pauseSuffix = timeline.pauseCount === 1 ? 'pause' : 'pauses'
  return (
    <div className="pacing-summary">
      <div className="pacing-summary-title">
        <strong>Pacing Band</strong>
        <span className="muted">
          Active play: {formatDuration(timeline.totalActiveMs)} · Wall time: {formatDuration(timeline.totalWallMs)}
          {timeline.hasPauses && ` · ${timeline.pauseCount} ${pauseSuffix} (${formatDuration(timeline.totalPauseMs)})`}
        </span>
      </div>
    </div>
  )
}

function PacingSpansLayer({ timeline }: { timeline: PacingTimelineData }) {
  return (
    <div className="pacing-spans-layer" aria-hidden="true">
      {timeline.spans.map((span, idx) => (
        <div
          key={idx}
          className={`pacing-span pacing-span--${span.type}`}
          style={{ left: `${span.startPct}%`, width: `${Math.max(0.5, span.endPct - span.startPct)}%` }}
          title={span.type === 'paused' ? `Background pause: ${formatDuration(span.wallDurationMs)}${span.capped ? ' (capped)' : ''}` : 'Active gameplay'}
        >
          {span.type === 'paused' && <span className="pacing-pause-label" aria-hidden="true">⏸</span>}
        </div>
      ))}
    </div>
  )
}

function PacingWavesLayer({ timeline }: { timeline: PacingTimelineData }) {
  return (
    <div className="pacing-waves-layer" aria-hidden="true">
      {timeline.waveBoundaries.map((wb, idx) => (
        <div key={idx} className="pacing-wave-boundary" style={{ left: `${wb.pct}%` }}>
          <span className="pacing-wave-tag">{wb.label}</span>
        </div>
      ))}
    </div>
  )
}

function PacingInteractionsLayer({ timeline, currentIndex }: { timeline: PacingTimelineData; currentIndex: number }) {
  return (
    <div className="pacing-interactions-layer" aria-hidden="true">
      {timeline.interactions.map((interaction) => {
        const isActive = currentIndex >= interaction.startStepIndex && currentIndex <= interaction.endStepIndex
        const width = Math.max(1, interaction.endPct - interaction.startPct)
        const blockLabel = interaction.blockId >= 0 ? `Detail ${interaction.blockId}` : 'Detail'
        return (
          <div
            key={`${interaction.interactionId}-${interaction.startStepIndex}`}
            className={`pacing-interaction ${isActive ? 'pacing-interaction--active' : ''} pacing-interaction--${interaction.completed ? 'complete' : 'fail'}`}
            style={{ left: `${interaction.startPct}%`, width: `${width}%` }}
            title={`${blockLabel}: ${interaction.outcome} (${formatDuration(interaction.activeDurationMs)})`}
          />
        )
      })}
    </div>
  )
}

function PacingMarkersLayer(p: {
  timeline: PacingTimelineData
  currentIndex: number
  onSelectStep: (idx: number) => void
  onHoverMarker: (m: PacingMarker | null) => void
}) {
  return (
    <div className="pacing-markers-layer" aria-hidden="true">
      {p.timeline.markers.map((marker) => {
        const isCurrent = marker.stepIndex === p.currentIndex
        return (
          <span
            key={marker.stepIndex}
            className={`pacing-marker pacing-marker--${marker.category} pacing-marker--${marker.shape} ${isCurrent ? 'pacing-marker--current' : ''}`}
            style={{ left: `${marker.pct}%` }}
            onPointerDown={() => p.onSelectStep(marker.stepIndex)}
            onClick={(e) => {
              e.stopPropagation()
              p.onSelectStep(marker.stepIndex)
            }}
            onMouseEnter={() => p.onHoverMarker(marker)}
            onMouseLeave={() => p.onHoverMarker(null)}
          >
            <span className="pacing-marker-icon">{marker.icon}</span>
          </span>
        )
      })}
    </div>
  )
}

function PacingTooltip({
  hovered,
  currentMarker,
  currentStep,
}: {
  hovered: PacingMarker | null
  currentMarker?: PacingMarker
  currentStep?: PuzzleReplayStep
}) {
  const display = hovered || currentMarker || {
    label: currentStep?.name ? currentStep.name.replace(/_/g, ' ') : 'Step',
    activeMs: currentStep?.active_elapsed_ms || 0,
    wallMs: 0,
    outcome: currentStep?.outcome,
    stepIndex: currentStep?.index ?? 0,
    category: 'generic' as const,
  }
  const showOutcome = display.outcome && !display.label.toLowerCase().includes(display.outcome.toLowerCase())
  return (
    <div className="pacing-tooltip">
      <span className="pacing-tooltip-badge">Step {display.stepIndex + 1}</span>
      <span className="pacing-tooltip-label">{display.label}</span>
      {showOutcome && <span className="pacing-tooltip-outcome">({display.outcome})</span>}
      <span className="muted">· {formatDuration(display.activeMs)} active</span>
    </div>
  )
}

function PacingLegend({ hasDeveloper }: { hasDeveloper: boolean }) {
  return (
    <div className="pacing-legend" aria-label="Timeline symbol legend">
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--success pacing-marker--circle" aria-hidden="true">✓</span> Placed</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--release pacing-marker--dot" aria-hidden="true">•</span> Returned / Pickup</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--fall_missing_support pacing-marker--triangle-down" aria-hidden="true">▼</span> Missing Support</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--fell_no_snap_target pacing-marker--triangle-up" aria-hidden="true">▲</span> No Snap Target</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--fall_other pacing-marker--triangle-down" aria-hidden="true">▼</span> Other Fall</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--hint pacing-marker--pin" aria-hidden="true"><span className="pacing-marker-icon">💡</span></span> Hint</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--checkpoint pacing-marker--square" aria-hidden="true">■</span> Checkpoint</span>
      <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--recovery pacing-marker--square" aria-hidden="true">⚑</span> Recovery</span>
      {hasDeveloper && <span className="pacing-legend-item"><span className="pacing-marker pacing-marker--developer pacing-marker--gear" aria-hidden="true">⚙</span> Developer</span>}
      <span className="pacing-legend-item"><span className="pacing-legend-bar pacing-legend-bar--complete" aria-hidden="true" /> Complete Chain</span>
      <span className="pacing-legend-item"><span className="pacing-legend-bar pacing-legend-bar--fail" aria-hidden="true" /> Failed Chain</span>
      <span className="pacing-legend-item"><span className="pacing-legend-pause-swatch" aria-hidden="true">⏸</span> Background Pause</span>
    </div>
  )
}
