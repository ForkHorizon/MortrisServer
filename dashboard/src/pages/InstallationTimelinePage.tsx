import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { TimelineEvent, TimelineResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { DataTable, type Column } from '../components/DataTable'
import { Entity } from '../components/Entity'
import { Freshness } from '../components/Freshness'

type SessionEvent = TimelineEvent & { deltaMs: number }

// Groups the (newest-first) event list by session_id, preserving
// most-recent-session-first order, and sorts each group ascending by
// session_elapsed_ms so the +elapsed deltas read chronologically.
function groupBySession(events: TimelineEvent[]): Array<{ sessionId: string; events: SessionEvent[] }> {
  const order: string[] = []
  const bySession = new Map<string, TimelineEvent[]>()
  for (const e of events) {
    if (!bySession.has(e.session_id)) {
      order.push(e.session_id)
      bySession.set(e.session_id, [])
    }
    bySession.get(e.session_id)!.push(e)
  }
  return order.map((sessionId) => {
    const sorted = [...bySession.get(sessionId)!].sort((a, b) => a.session_elapsed_ms - b.session_elapsed_ms)
    const withDeltas = sorted.map((e, i) => ({
      ...e,
      deltaMs: i === 0 ? 0 : e.session_elapsed_ms - sorted[i - 1].session_elapsed_ms,
    }))
    return { sessionId, events: withDeltas }
  })
}

const sessionColumns: Column<SessionEvent>[] = [
  { key: 'effective_at', label: 'When', render: (r) => new Date(r.effective_at).toLocaleString() },
  { key: 'delta', label: '+elapsed', render: (r) => `+${r.deltaMs}ms` },
  { key: 'name', label: 'Event', render: (r) => <Entity type="event" value={r.name} /> },
  { key: 'event_kind', label: 'Kind' },
  { key: 'time_quality', label: 'Clock quality' },
  { key: 'properties', label: 'Properties', render: (r) => JSON.stringify(r.properties) },
]

function LookupForm({ inputId, setInputId, onSubmit }: { inputId: string; setInputId: (v: string) => void; onSubmit: () => void }) {
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
    >
      <div className="field">
        <label htmlFor="install-id">Installation ID</label>
        <input
          id="install-id"
          value={inputId}
          onChange={(e) => setInputId(e.target.value)}
          placeholder="09ffb634-1792-40cd-bd9e-0a89938ff411"
        />
      </div>
      <button type="submit">Look up</button>
    </form>
  )
}

function SessionSection({ sessionId, events }: { sessionId: string; events: SessionEvent[] }) {
  return (
    <details open>
      <summary>
        Session <Entity type="session" value={sessionId} /> — {events.length} event{events.length === 1 ? '' : 's'},
        spanning {events[events.length - 1].session_elapsed_ms}ms
      </summary>
      <DataTable
        caption={`Events for session ${sessionId}`}
        columns={sessionColumns}
        rows={events}
        getRowKey={(r) => r.event_id}
      />
    </details>
  )
}

function InstallationDetail({ data }: { data: TimelineResult }) {
  const sessions = useMemo(() => groupBySession(data.events), [data])
  return (
    <>
      <dl>
        <dt>Registered</dt>
        <dd>{new Date(data.registered_at).toLocaleString()}</dd>
        <dt>Activated</dt>
        <dd>{data.activated_at ? new Date(data.activated_at).toLocaleString() : 'Never'}</dd>
      </dl>
      {data.truncated && <p role="alert">Showing the most recent 500 events only.</p>}
      {sessions.map(({ sessionId, events }) => (
        <SessionSection key={sessionId} sessionId={sessionId} events={events} />
      ))}
    </>
  )
}

export function InstallationTimelinePage() {
  const { currentProject } = useAuth()
  const [searchParams] = useSearchParams()
  const idFromFeed = searchParams.get('id') ?? ''
  const [inputId, setInputId] = useState(idFromFeed)
  const [installId, setInstallId] = useState(idFromFeed)
  const fetchTimeline = useCallback(
    () =>
      installId && currentProject
        ? apiGet<TimelineResult>(`/api/v1/analytics/installations/${encodeURIComponent(installId)}`, {
            project: currentProject,
          })
        : Promise.resolve(null),
    [currentProject, installId],
  )

  const { data, error, loading, updatedAt, stale } = useApiData<TimelineResult | null>(
    fetchTimeline,
    `installation:${currentProject}:${installId}`,
  )

  if (!currentProject) return <p>Select a project to look up an installation.</p>

  return (
    <section aria-labelledby="timeline-heading">
      <h1 id="timeline-heading">Installation timeline</h1>
      <p>Admin-only: full product and system event history for one anonymous installation ID.</p>
      <LookupForm inputId={inputId} setInputId={setInputId} onSubmit={() => setInstallId(inputId.trim())} />

      {installId && <Freshness loading={loading} error={error} stale={stale} updatedAt={updatedAt} />}
      {data && <InstallationDetail data={data} />}
    </section>
  )
}
