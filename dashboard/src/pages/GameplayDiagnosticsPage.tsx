import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { GameplayAttempt, GameplayDiagnostics, GameplayPlayersResult, TimelineResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { DataTable } from '../components/DataTable'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { StatGrid, StatTile } from '../components/StatTile'
import { shortId } from '../components/Entity'
import { Timestamp } from '../components/Timestamp'
import { PropertiesPreview } from '../components/PropertiesPreview'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

const duration = (ms: number) => `${Math.round(ms / 1000)}s`
const percent = (ratio: number) => `${Math.round(ratio * 100)}%`

function summaryVerdict(data: GameplayDiagnostics): string {
  const { attempts, falls } = data.summary
  const completed = data.summary.by_outcome?.find((o) => o.outcome === 'completed')?.attempts ?? 0
  if (attempts === 0) return 'No attempts in this range.'
  const completionRate = Math.round((completed / attempts) * 100)
  const fallsPerAttempt = (falls / attempts).toFixed(1)
  return `${completionRate}% of attempts completed (${completed} of ${attempts}), averaging ${fallsPerAttempt} falls per attempt.`
}

function GameplaySummarySection({ data }: { data: GameplayDiagnostics }) {
  return <>
    <p className="verdict">{summaryVerdict(data)}</p>
    <StatGrid>
      <StatTile label="Attempts" value={data.summary.attempts} />
      <StatTile label="Falls" value={data.summary.falls} />
      <StatTile label="Active time" value={duration(data.summary.active_elapsed_ms)} />
      <StatTile label="Wall time" value={duration(data.summary.wall_elapsed_ms)} />
      <StatTile label="Breaks" value={data.summary.pause_count} />
      <StatTile label="Hints" value={data.summary.hints} />
    </StatGrid>
    <p>Active and wall time above cover completed and given-up attempts only — see the breakdown below for attempts still in progress or lost.</p>
  </>
}

function GameplayTables({ data }: { data: GameplayDiagnostics }) {
  return <>
    <details>
      <summary>Attempts by outcome</summary>
      <DataTable caption="Attempts by outcome" rows={data.summary.by_outcome ?? []} getRowKey={(r) => r.outcome} columns={[
        { key: 'outcome', label: 'Outcome' }, { key: 'attempts', label: 'Attempts' },
        { key: 'active_elapsed_ms', label: 'Active', render: (r) => duration(r.active_elapsed_ms) },
        { key: 'wall_elapsed_ms', label: 'Wall', render: (r) => duration(r.wall_elapsed_ms) },
        { key: 'pause_count', label: 'Breaks' },
      ]} />
    </details>
    <details>
      <summary>City, house, and wave outcomes</summary>
      <DataTable caption="City, house, and wave outcomes" rows={data.scopes} getRowKey={(r) => `${r.city_id}-${r.house_id}-${r.wave_index}`} columns={[
        { key: 'city_id', label: 'City' }, { key: 'house_id', label: 'House' }, { key: 'wave_index', label: 'Wave' }, { key: 'attempts', label: 'Attempts' },
        { key: 'falls', label: 'Falls' }, { key: 'hints', label: 'Hints' }, { key: 'active_elapsed_ms', label: 'Active', render: (r) => duration(r.active_elapsed_ms) },
      ]} />
    </details>
    <details>
      <summary>Target and detail friction</summary>
      <DataTable caption="Target and detail friction" rows={data.friction} getRowKey={(r) => `${r.block_id}-${r.target_id}`} columns={[
        { key: 'block_id', label: 'Detail' }, { key: 'target_id', label: 'Target' }, { key: 'attempts', label: 'Attempts' }, { key: 'placements', label: 'Placed' }, { key: 'falls', label: 'Falls' },
        { key: 'fall_rate', label: 'Fall rate', render: (r) => percent(r.fall_rate) }, { key: 'first_attempt_failure_rate', label: 'First fail', render: (r) => percent(r.first_attempt_failure_rate) },
      ]} />
    </details>
    <details>
      <summary>Daily outcomes</summary>
      <DataTable caption="Daily outcomes (selected IANA timezone)" rows={data.daily} getRowKey={(r) => `${r.day}-${r.city_id}-${r.house_id}-${r.wave_index}-${r.content_revision}-${r.build_number}`} columns={[
        { key: 'day', label: 'Day' }, { key: 'city_id', label: 'City' }, { key: 'house_id', label: 'House' }, { key: 'wave_index', label: 'Wave' },
        { key: 'build_number', label: 'Build' }, { key: 'attempts', label: 'Attempts' }, { key: 'falls', label: 'Falls' }, { key: 'hints', label: 'Hints' },
      ]} />
    </details>
  </>
}

export function GameplayDiagnosticsPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to, timezone } = range.params
  const fetchDiagnostics = useCallback(() => apiGet<GameplayDiagnostics>('/api/v1/analytics/gameplay/diagnostics', { project: currentProject, from, to, timezone }), [currentProject, from, to, timezone])
  const diagnostics = useApiData(fetchDiagnostics, `gameplay-diagnostics:${currentProject}:${from}:${to}:${timezone}`)

  if (!currentProject) return <p>Select a project to inspect gameplay.</p>
  return <section aria-labelledby="gameplay-heading">
    <h1 id="gameplay-heading">Gameplay Diagnostics</h1>
    <p>Gravity outcomes are reconstructed from immutable content revisions. Active time excludes background intervals; wall time includes them.</p>
    <DateRangeFields range={range} />
    <Freshness loading={diagnostics.loading} error={diagnostics.error} stale={diagnostics.stale} updatedAt={diagnostics.updatedAt} />
    {diagnostics.data && <>
      <GameplaySummarySection data={diagnostics.data} />
      <GameplayTables data={diagnostics.data} />
    </>}
    <details>
      <summary>Anonymous players and devices</summary>
      <PlayersSection project={currentProject} from={from} to={to} />
    </details>
    <details>
      <summary>Attempt timeline and gravity reconstruction</summary>
      <AttemptSection project={currentProject} />
    </details>
  </section>
}

function AttemptSection({ project }: { project: string }) {
  const [attemptID, setAttemptID] = useState('')
  const [selectedAttempt, setSelectedAttempt] = useState('')
  const fetchAttempt = useCallback(() => selectedAttempt ? apiGet<GameplayAttempt>(`/api/v1/analytics/gameplay/attempts/${encodeURIComponent(selectedAttempt)}`, { project }) : Promise.resolve(null), [project, selectedAttempt])
  const timeline = useApiData<GameplayAttempt | null>(fetchAttempt, `gameplay-attempt:${project}:${selectedAttempt}`)
  return <>
    <div className="field"><label htmlFor="attempt-id">Attempt ID</label><input id="attempt-id" value={attemptID} onChange={(e) => setAttemptID(e.target.value)} /><button type="button" onClick={() => setSelectedAttempt(attemptID)}>Load attempt</button></div>
    {selectedAttempt && <Freshness loading={timeline.loading} error={timeline.error} stale={timeline.stale} updatedAt={timeline.updatedAt} loadingLabel="Loading attempt…" />}
    {timeline.data && <DataTable caption={`Attempt ${timeline.data.attempt_id}`} rows={timeline.data.events} getRowKey={(r) => r.event_id} columns={[
      { key: 'effective_at', label: 'Time', render: (r) => <Timestamp value={r.effective_at} mode="absolute" /> }, { key: 'name', label: 'Action' },
      { key: 'properties', label: 'Payload', render: (r) => <PropertiesPreview properties={r.properties} /> },
      { key: 'missing_support_groups', label: 'Missing support by alternative', render: (r) => r.missing_support_groups?.map((group) => `[${group.join(', ')}]`).join(' OR ') ?? '' },
    ]} />}
  </>
}

function PlayersSection({ project, from, to }: { project: string; from: string; to: string }) {
  const [selectedPlayer, setSelectedPlayer] = useState('')
  const fetchPlayers = useCallback(() => apiGet<GameplayPlayersResult>('/api/v1/analytics/gameplay/players', { project, from, to }), [project, from, to])
  const players = useApiData<GameplayPlayersResult>(fetchPlayers, `gameplay-players:${project}:${from}:${to}`)
  const fetchTimeline = useCallback(() => selectedPlayer ? apiGet<TimelineResult>(`/api/v1/analytics/installations/${encodeURIComponent(selectedPlayer)}`, { project }) : Promise.resolve(null), [project, selectedPlayer])
  const timeline = useApiData<TimelineResult | null>(fetchTimeline, `installation:${project}:${selectedPlayer}`)
  return <>
    <p>The player ID is the SDK installation ID, not a game account or person. Select it to inspect every raw event for that device.</p>
    <Freshness loading={players.loading} error={players.error} stale={players.stale} updatedAt={players.updatedAt} loadingLabel="Loading players…" />
    {players.data && <DataTable caption="Anonymous player devices" rows={players.data.players} getRowKey={(r) => r.install_id} columns={[
      { key: 'install_id', label: 'Player ID', render: (r) => <button type="button" className="entity-id" title={r.install_id} onClick={() => setSelectedPlayer(r.install_id)}>{shortId(r.install_id)}</button> },
      { key: 'device_class', label: 'Phone' }, { key: 'os_version', label: 'OS' }, { key: 'device_total_memory_mb', label: 'Device RAM (MB)' },
      { key: 'last_allocated_memory_mb', label: 'Allocated RAM (MB)' }, { key: 'last_reserved_memory_mb', label: 'Reserved RAM (MB)' }, { key: 'last_mono_used_memory_mb', label: 'Mono RAM (MB)' }, { key: 'attempts', label: 'Attempts' }, { key: 'falls', label: 'Falls' },
    ]} />}
    {selectedPlayer && <Freshness loading={timeline.loading} error={timeline.error} stale={timeline.stale} updatedAt={timeline.updatedAt} loadingLabel="Loading player events…" />}
    {timeline.data && <DataTable caption={`Events for ${timeline.data.install_id}`} rows={timeline.data.events} getRowKey={(r) => r.event_id} columns={[
      { key: 'effective_at', label: 'Time', render: (r) => <Timestamp value={r.effective_at} mode="absolute" /> }, { key: 'name', label: 'Event' },
      { key: 'properties', label: 'Payload', render: (r) => <PropertiesPreview properties={r.properties} /> },
    ]} />}
  </>
}
