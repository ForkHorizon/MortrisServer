import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiGet, ApiError } from '../api/client'
import type { RecentEvent, RecentEventsResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { EventSelect } from '../components/EventSelect'
import { DataTable } from '../components/DataTable'

const POLL_MS = 5000

export function EventFeedPage() {
  const { currentProject } = useAuth()
  const [name, setName] = useState('')
  const [platform, setPlatform] = useState('')
  const [appVersion, setAppVersion] = useState('')
  const [installId, setInstallId] = useState('')
  const [paused, setPaused] = useState(false)
  const [events, setEvents] = useState<RecentEvent[]>([])
  const [cursor, setCursor] = useState<{ receivedAt?: string; eventId?: string }>({})
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(() => {
    if (!currentProject) return Promise.resolve()
    return apiGet<RecentEventsResult>('/api/v1/analytics/events/recent', {
      project: currentProject,
      name: name || undefined,
      platform: platform || undefined,
      app_version: appVersion || undefined,
      install_id: installId || undefined,
    })
      .then((res) => {
        setEvents(res.events)
        setCursor({ receivedAt: res.next_before_received_at, eventId: res.next_before_event_id })
        setError(null)
      })
      .catch((e: unknown) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'))
  }, [currentProject, name, platform, appVersion, installId])

  // Refetch page one on mount and whenever a filter changes.
  useEffect(() => {
    setLoading(true)
    refresh().finally(() => setLoading(false))
  }, [refresh])

  // Poll page one every ~5s unless paused. "Load older" (below) is only
  // meaningful while paused — the next poll always replaces the feed with
  // the newest page, which is the point of a live tail.
  useEffect(() => {
    if (paused) return
    const timer = setInterval(refresh, POLL_MS)
    return () => clearInterval(timer)
  }, [paused, refresh])

  const loadOlder = () => {
    if (!currentProject || !cursor.receivedAt || !cursor.eventId) return
    apiGet<RecentEventsResult>('/api/v1/analytics/events/recent', {
      project: currentProject,
      name: name || undefined,
      platform: platform || undefined,
      app_version: appVersion || undefined,
      install_id: installId || undefined,
      before_received_at: cursor.receivedAt,
      before_event_id: cursor.eventId,
    }).then((res) => {
      setEvents((prev) => [...prev, ...res.events])
      setCursor({ receivedAt: res.next_before_received_at, eventId: res.next_before_event_id })
    })
  }

  if (!currentProject) return <p>Select a project to view its live event feed.</p>

  return (
    <section aria-labelledby="feed-heading">
      <h1 id="feed-heading">Live event feed</h1>
      <p>Newest events first, refreshing every ~5 seconds.</p>
      <fieldset>
        <legend>Filters</legend>
        <div className="field">
          <label htmlFor="feed-name">Event name</label>
          <EventSelect id="feed-name" value={name} onChange={setName} />
        </div>
        <div className="field">
          <label htmlFor="feed-platform">Platform</label>
          <input id="feed-platform" value={platform} onChange={(e) => setPlatform(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="feed-app-version">App version</label>
          <input id="feed-app-version" value={appVersion} onChange={(e) => setAppVersion(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="feed-install-id">Install ID</label>
          <input id="feed-install-id" value={installId} onChange={(e) => setInstallId(e.target.value)} />
        </div>
      </fieldset>
      <button type="button" onClick={() => setPaused((p) => !p)}>
        {paused ? 'Resume' : 'Pause'}
      </button>

      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      <DataTable
        caption="Most recent events"
        columns={[
          { key: 'received_at', label: 'Received', render: (r) => new Date(r.received_at).toLocaleString() },
          { key: 'name', label: 'Event' },
          { key: 'event_kind', label: 'Kind' },
          {
            key: 'install_id',
            label: 'Install',
            render: (r) => <Link to={`/installations?id=${encodeURIComponent(r.install_id)}`}>{r.install_id}</Link>,
          },
          { key: 'platform', label: 'Platform' },
          { key: 'app_version', label: 'Version' },
          {
            key: 'properties',
            label: 'Properties',
            render: (r) => (
              <details>
                <summary>view</summary>
                <pre>{JSON.stringify(r.properties, null, 2)}</pre>
              </details>
            ),
          },
        ]}
        rows={events}
        getRowKey={(r) => r.event_id}
      />
      {cursor.receivedAt && (
        <button type="button" onClick={loadOlder}>
          Load older
        </button>
      )}
    </section>
  )
}
