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
  if (step.name !== 'placement_resolved') return eventWords(step.name)
  if (step.outcome === 'placed') return `Detail ${step.block_id} went in.`
  const missing = missingBlocks(step)
  const why = outcomeWords(step.outcome)
  if (missing.length === 0) return `Detail ${step.block_id} — ${why}.`
  return `Detail ${step.block_id} — ${why}. It was waiting on ${missing.length === 1 ? `detail ${missing[0]}` : `details ${missing.join(', ')}`}.`
}

const EVENT_WORDS: Record<string, string> = {
  house_opened: 'Opened the house.',
  wave_presented: 'A new wave of details appeared.',
  detail_taken: 'Picked up a detail.',
  hint_used: 'Asked for the blueprint hint.',
  app_backgrounded: 'Left the app.',
  app_foregrounded: 'Came back.',
  wave_completed: 'Finished the wave.',
  house_completed: 'Finished the house.',
  attempt_closed: 'Attempt ended.',
}

function eventWords(name: string): string {
  return EVENT_WORDS[name] ?? name.replace(/_/g, ' ')
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
        active={step.name === 'placement_resolved' ? step.block_id : null}
        missing={new Set(missingBlocks(step))}
      />
      <p className="verdict">{stepSentence(step)}</p>
      <div className="replay-controls">
        <button type="button" onClick={() => setPlaying(!playing)} disabled={index >= last && !playing}>
          {playing ? 'Pause' : 'Play'}
        </button>
        {/* Functional updates, not index +/- 1: the handler closes over
            the index from its render, so two clicks before a re-render
            would both compute the same next step. */}
        <button type="button" onClick={() => { setPlaying(false); setIndex((i) => Math.max(0, i - 1)) }} disabled={index === 0}>
          Back
        </button>
        <button type="button" onClick={() => { setPlaying(false); setIndex((i) => Math.min(last, i + 1)) }} disabled={index >= last}>
          Next
        </button>
        <input
          type="range"
          min={0}
          max={last}
          value={index}
          aria-label="Replay step"
          onChange={(e) => { setPlaying(false); setIndex(Number(e.target.value)) }}
        />
        <span className="muted">step {index + 1} of {replay.steps.length}</span>
      </div>
      {replay.truncated && <p className="muted">Only the first 500 events of this attempt are shown.</p>}
    </div>
  )
}
