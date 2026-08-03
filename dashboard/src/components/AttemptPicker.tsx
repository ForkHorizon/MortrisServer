import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { PuzzleAttemptList, PuzzleHouseBlock, PuzzleReplay } from '../api/houseTypes'
import { useApiData } from '../hooks/useApiData'
import { ReplayPlayer } from './ReplayPlayer'
import { Timestamp } from './Timestamp'

type Props = { project: string; city: string; house: string; from: string; to: string; blocks: PuzzleHouseBlock[]; label: string }

// Attempts are listed worst-first rather than newest-first: the reason to
// open a replay is almost always "show me one that went badly".
export function AttemptPicker({ project, city, house, from, to, blocks, label }: Props) {
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

  const rows = [...(attempts.data?.attempts ?? [])].sort((a, b) => b.falls - a.falls)
  if (rows.length === 0) return <p className="muted">No attempts on this house in this range.</p>
  return (
    <>
      <ul className="attempt-list">
        {rows.map((a) => (
          <li key={a.attempt_id}>
            <button type="button" aria-pressed={selected === a.attempt_id} onClick={() => setSelected(a.attempt_id)}>
              <Timestamp value={a.started_at} mode="absolute" />
              {' · '}wave {a.wave_index + 1}
              {' · '}{a.placements} {a.placements === 1 ? 'drop' : 'drops'}
              {a.falls > 0 && `, ${a.falls} fell`}
              {a.hints > 0 && `, ${a.hints} hint${a.hints === 1 ? '' : 's'}`}
              {a.completed ? ' · finished' : ' · not finished'}
            </button>
          </li>
        ))}
      </ul>
      {selected && replay.data && <ReplayPlayer replay={replay.data} blocks={blocks} label={label} />}
      {selected && replay.loading && <p className="muted">Loading replay…</p>}
    </>
  )
}
