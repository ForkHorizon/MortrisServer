import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleAttemptList, PuzzleAttemptSummary, PuzzleHouseBlock, PuzzleReplay } from '../api/houseTypes'
import { useApiData } from '../hooks/useApiData'
import { ReplayPlayer } from './ReplayPlayer'
import { Timestamp } from './Timestamp'
import { outcomeWords } from './houseColors'

type Props = { project: string; city: string; house: string; from: string; to: string; blocks: PuzzleHouseBlock[]; label: string; waveIndex?: number | null }

// Attempts are listed worst-first rather than newest-first: the reason to
// open a replay is almost always "show me one that went badly". waveIndex,
// when set (via the wave tabs or a wave-staircase step — plan section 7,
// "clicking a step should filter the attempt/run examples below it"),
// narrows the list to attempts of that one wave.
export function AttemptPicker({ project, city, house, from, to, blocks, label, waveIndex }: Props) {
  const [selected, setSelected] = useState('')
  const fetchAttempts = useCallback(
    () => apiGet<PuzzleAttemptList>(`/api/v1/analytics/gameplay/houses/${city}/${house}/attempts`, { project, from, to }),
    [project, city, house, from, to],
  )
  const attempts = useApiData(fetchAttempts, `puzzle-attempts:${project}:${city}:${house}:${from}:${to}`)
  const fetchReplay = useCallback(
    () => (selected ? apiGet<PuzzleReplay>(`/api/v1/analytics/gameplay/attempts/${encodeURIComponent(selected)}/replay`, { project }) : Promise.resolve(null)),
    [selected, project],
  )
  const replay = useApiData<PuzzleReplay | null>(fetchReplay, `puzzle-replay:${project}:${selected}`)

  const scoped = waveIndex != null ? (attempts.data?.attempts ?? []).filter((row) => row.wave_index === waveIndex) : (attempts.data?.attempts ?? [])
  const rows = [...scoped].sort((a, b) => b.falls - a.falls || b.active_duration_ms - a.active_duration_ms)
  if (rows.length === 0) return <p className="muted">{waveIndex != null ? `No attempts on wave ${waveIndex + 1} in this range.` : 'No attempts on this house in this range.'}</p>
  return (
    <>
      <AttemptGroup title="Representative failure" attempts={rows.filter((row) => row.falls > 0).slice(0, 1)} selected={selected} onSelect={setSelected} />
      <AttemptGroup title="Longest or hardest attempt" attempts={rows.filter((row) => row.falls === 0).slice(0, 1)} selected={selected} onSelect={setSelected} />
      <AttemptGroup title="Completed attempt" attempts={rows.filter((row) => row.completed).slice(0, 1)} selected={selected} onSelect={setSelected} />
      <details><summary>All {rows.length} attempts</summary><AttemptGroup attempts={rows} selected={selected} onSelect={setSelected} /></details>
      {selected && replay.data && <ReplayPlayer replay={replay.data} blocks={blocks} label={label} />}
      {selected && replay.loading && <p className="muted">Loading replay…</p>}
    </>
  )
}

function AttemptGroup({ title, attempts, selected, onSelect }: { title?: string; attempts: PuzzleAttemptSummary[]; selected: string; onSelect: (id: string) => void }) {
  if (attempts.length === 0) return null
  return <section className="attempt-group">{title && <h3>{title}</h3>}<ul className="attempt-list">{attempts.map((attempt) => <li key={attempt.attempt_id}><button type="button" aria-pressed={selected === attempt.attempt_id} onClick={() => onSelect(attempt.attempt_id)}><Timestamp value={attempt.started_at} mode="absolute" /> · wave {attempt.last_wave_index + 1} · {attempt.placements} drops{attempt.falls > 0 && `, ${attempt.falls} fell`} {attempt.dominant_failure && `(${outcomeWords(attempt.dominant_failure)})`} · {Math.round(attempt.active_duration_ms / 1000)}s active · {attempt.completed ? 'finished' : 'not finished'}{attempt.developer_affected && ' · cheat used'}{attempt.progress_origin !== 'natural' && ` · tester: ${attempt.progress_origin.replace(/_/g, ' ')}`}</button>{attempt.install_id && <Link to={`/installations/${attempt.install_id}`} className="muted">Open anonymous device timeline</Link>}</li>)}</ul></section>
}
