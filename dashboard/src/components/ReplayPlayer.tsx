import { useEffect, useState } from 'react'
import type { PuzzleHouseBlock, PuzzleReplay, PuzzleReplayStep } from '../api/houseTypes'
import { HouseCanvas } from './HouseCanvas'
import { outcomeWords } from './houseColors'

const STEP_MS = 900

// Only the alternatives that are actually still short of something are
// worth naming — an alternative with nothing missing was satisfied, and
// listing it as "missing []" reads as a bug.
function missingBlocks(step: PuzzleReplayStep): number[] {
  return [...new Set((step.missing_support ?? []).flat())]
}

function stepSentence(step: PuzzleReplayStep): string {
  if (step.name === 'detail_taken') return `Picked up detail ${step.block_id}.`
  if (step.name === 'detail_released') return `Released detail ${step.block_id}${step.target_id >= 0 ? ` toward slot ${step.target_id}` : ''}.`
  if (step.name === 'detail_returned') return `Detail ${step.block_id} returned to inventory.`
  if (step.name !== 'placement_resolved') return eventWords(step.name)
  if (step.outcome === 'placed') return `Detail ${step.block_id} went in.`
  const missing = missingBlocks(step)
  const why = outcomeWords(step.outcome)
  if (missing.length === 0) return `Detail ${step.block_id} — ${why}.`
  return `Detail ${step.block_id} — ${why}. It was waiting on ${missing.length === 1 ? `detail ${missing[0]}` : `details ${missing.join(', ')}`}.`
}

const EVENT_WORDS: Record<string, string> = {
  house_run_started: 'Started the house run.',
  wave_attempt_started: 'A new wave attempt started.',
  detail_taken: 'Picked up a detail.',
  hint_used: 'Asked for the blueprint hint.',
  app_backgrounded: 'Left the app.',
  app_foregrounded: 'Came back.',
  wave_completed: 'Finished the wave.',
  house_completed: 'Finished the house.',
  attempt_closed: 'Attempt ended.',
  attempt_recovered: 'Recovered the attempt after restart.',
  state_checkpoint: 'Confirmed the saved house state.',
}

function eventWords(name: string): string {
  return EVENT_WORDS[name] ?? name.replace(/_/g, ' ')
}

type ControlProps = {
  index: number
  last: number
  total: number
  playing: boolean
  setPlaying: (v: boolean) => void
  setIndex: (updater: (i: number) => number) => void
}

function ReplayControls({ index, last, total, playing, setPlaying, setIndex }: ControlProps) {
  // Functional updates, not index +/- 1: a handler closes over the index
  // from its render, so two clicks before a re-render would otherwise
  // both compute the same next step.
  const step = (delta: number) => {
    setPlaying(false)
    setIndex((i) => Math.min(last, Math.max(0, i + delta)))
  }
  return (
    <div className="replay-controls">
      <button type="button" onClick={() => setPlaying(!playing)} disabled={index >= last && !playing}>
        {playing ? 'Pause' : 'Play'}
      </button>
      <button type="button" onClick={() => step(-1)} disabled={index === 0}>
        Back
      </button>
      <button type="button" onClick={() => step(1)} disabled={index >= last}>
        Next
      </button>
      <input
        type="range"
        min={0}
        max={last}
        value={index}
        aria-label="Replay step"
        onChange={(e) => { setPlaying(false); setIndex(() => Number(e.target.value)) }}
      />
      <span className="muted">step {index + 1} of {total}</span>
    </div>
  )
}

export function ReplayPlayer({ replay, blocks, label }: { replay: PuzzleReplay; blocks: PuzzleHouseBlock[]; label: string }) {
  const [index, setIndex] = useState(0)
  const [playing, setPlaying] = useState(false)
  const last = replay.steps.length - 1

  useEffect(() => setIndex(0), [replay.attempt_id])
  useEffect(() => {
    if (!playing) return
    if (index >= last) {
      setPlaying(false)
      return
    }
    const timer = setTimeout(() => setIndex((i) => Math.min(i + 1, last)), STEP_MS)
    return () => clearTimeout(timer)
  }, [playing, index, last])

  const step = replay.steps[index]
  if (!step) return <p className="muted">This attempt has no steps to replay.</p>
  return (
    <div className="replay">
      <HouseCanvas
        blocks={blocks}
        interactive={false}
        title={`${label}, attempt replay at step ${index + 1}`}
        placed={new Set(step.placed)}
        active={step.block_id >= 0 ? step.block_id : null}
        missing={new Set(missingBlocks(step))}
        replayRelease={step.name === 'detail_released' || step.name === 'placement_resolved' ? { x: step.release_x_milli, y: step.release_y_milli, targetID: step.target_id } : null}
      />
      <p className="verdict">{stepSentence(step)}</p>
      {step.progress_origin && step.progress_origin !== 'natural' && <p className="muted">Tester progress: {step.progress_origin.replace(/_/g, ' ')}.</p>}
      {(replay.sequence_gaps > 0 || replay.sequence_duplicates > 0 || replay.orphan_interactions > 0 || replay.state_hash_mismatches > 0) && <p className="warning">Telemetry integrity: {replay.sequence_gaps} missing, {replay.sequence_duplicates} duplicate, {replay.orphan_interactions} unfinished interactions, {replay.state_hash_mismatches} state mismatches.</p>}
      {replay.revision_warning && <p className="warning">{replay.revision_warning}</p>}
      {step.active_elapsed_ms >= 0 && <p className="muted">{Math.round(step.active_elapsed_ms / 1000)}s of active play into this attempt.</p>}
      <ReplayControls
        index={index}
        last={last}
        total={replay.steps.length}
        playing={playing}
        setPlaying={setPlaying}
        setIndex={setIndex}
      />
      {replay.truncated && <p className="muted">Only the first 10,000 events of this attempt are shown.</p>}
    </div>
  )
}
