import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { GameplayAttempt, GameplayDiagnostics } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { DataTable } from '../components/DataTable'
import { DateRangeFields } from '../components/DateRangeFields'
import { StatGrid, StatTile } from '../components/StatTile'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

const duration = (ms: number) => `${Math.round(ms / 1000)}s`
const percent = (ratio: number) => `${Math.round(ratio * 100)}%`

export function GameplayDiagnosticsPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to, timezone } = range.params
  const [attemptID, setAttemptID] = useState('')
  const [selectedAttempt, setSelectedAttempt] = useState('')
  const fetchDiagnostics = useCallback(() => apiGet<GameplayDiagnostics>('/api/v1/analytics/gameplay/diagnostics', { project: currentProject, from, to, timezone }), [currentProject, from, to, timezone])
  const diagnostics = useApiData(fetchDiagnostics)
  const fetchAttempt = useCallback(() => selectedAttempt ? apiGet<GameplayAttempt>(`/api/v1/analytics/gameplay/attempts/${encodeURIComponent(selectedAttempt)}`, { project: currentProject }) : Promise.resolve(null), [currentProject, selectedAttempt])
  const timeline = useApiData<GameplayAttempt | null>(fetchAttempt)

  if (!currentProject) return <p>Select a project to inspect gameplay.</p>
  return <section aria-labelledby="gameplay-heading">
    <h1 id="gameplay-heading">Gameplay Diagnostics</h1>
    <p>Gravity outcomes are reconstructed from immutable content revisions. Active time excludes background intervals; wall time includes them.</p>
    <DateRangeFields range={range} />
    {diagnostics.loading && <p role="status">Loading…</p>}
    {diagnostics.error && <p role="alert">{diagnostics.error}</p>}
    {diagnostics.data && <>
      <StatGrid>
        <StatTile label="Attempts" value={diagnostics.data.summary.attempts} />
        <StatTile label="Falls" value={diagnostics.data.summary.falls} />
        <StatTile label="Active time" value={duration(diagnostics.data.summary.active_elapsed_ms)} />
        <StatTile label="Wall time" value={duration(diagnostics.data.summary.wall_elapsed_ms)} />
        <StatTile label="Breaks" value={diagnostics.data.summary.pause_count} />
        <StatTile label="Hints" value={diagnostics.data.summary.hints} />
      </StatGrid>
      <DataTable caption="City, house, and wave outcomes" rows={diagnostics.data.scopes} getRowKey={(r) => `${r.city_id}-${r.house_id}-${r.wave_index}`} columns={[
        { key: 'city_id', label: 'City' }, { key: 'house_id', label: 'House' }, { key: 'wave_index', label: 'Wave' }, { key: 'attempts', label: 'Attempts' },
        { key: 'falls', label: 'Falls' }, { key: 'hints', label: 'Hints' }, { key: 'active_elapsed_ms', label: 'Active', render: (r) => duration(r.active_elapsed_ms) },
      ]} />
      <DataTable caption="Target and detail friction" rows={diagnostics.data.friction} getRowKey={(r) => `${r.block_id}-${r.target_id}`} columns={[
        { key: 'block_id', label: 'Detail' }, { key: 'target_id', label: 'Target' }, { key: 'attempts', label: 'Attempts' }, { key: 'placements', label: 'Placed' }, { key: 'falls', label: 'Falls' },
        { key: 'fall_rate', label: 'Fall rate', render: (r) => percent(r.fall_rate) }, { key: 'first_attempt_failure_rate', label: 'First fail', render: (r) => percent(r.first_attempt_failure_rate) },
      ]} />
      <DataTable caption="Daily outcomes (selected IANA timezone)" rows={diagnostics.data.daily} getRowKey={(r) => `${r.day}-${r.city_id}-${r.house_id}-${r.wave_index}-${r.content_revision}-${r.build_number}`} columns={[
        { key: 'day', label: 'Day' }, { key: 'city_id', label: 'City' }, { key: 'house_id', label: 'House' }, { key: 'wave_index', label: 'Wave' },
        { key: 'build_number', label: 'Build' }, { key: 'attempts', label: 'Attempts' }, { key: 'falls', label: 'Falls' }, { key: 'hints', label: 'Hints' },
      ]} />
    </>}
    <h2>Attempt timeline and gravity reconstruction</h2>
    <div className="field"><label htmlFor="attempt-id">Attempt ID</label><input id="attempt-id" value={attemptID} onChange={(e) => setAttemptID(e.target.value)} /><button type="button" onClick={() => setSelectedAttempt(attemptID)}>Load attempt</button></div>
    {timeline.loading && <p role="status">Loading attempt…</p>}
    {timeline.error && <p role="alert">{timeline.error}</p>}
    {timeline.data && <DataTable caption={`Attempt ${timeline.data.attempt_id}`} rows={timeline.data.events} getRowKey={(r) => r.event_id} columns={[
      { key: 'effective_at', label: 'Time', render: (r) => new Date(r.effective_at).toLocaleString() }, { key: 'name', label: 'Action' },
      { key: 'properties', label: 'Payload', render: (r) => <code>{JSON.stringify(r.properties)}</code> },
      { key: 'missing_support_groups', label: 'Missing support by alternative', render: (r) => r.missing_support_groups?.map((group) => `[${group.join(', ')}]`).join(' OR ') ?? '' },
    ]} />}
  </section>
}
