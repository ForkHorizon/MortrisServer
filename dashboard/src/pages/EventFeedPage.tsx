import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { RecentEvent } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useRecentEventsFeed } from '../hooks/useRecentEventsFeed'
import { useURLParams } from '../hooks/useURLParams'
import { EventSelect } from '../components/EventSelect'
import { DataTable, type Column } from '../components/DataTable'

const feedColumns: Column<RecentEvent>[] = [
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
]

function FeedFilters({
  name,
  setName,
  platform,
  setPlatform,
  appVersion,
  setAppVersion,
  installId,
  setInstallId,
}: {
  name: string
  setName: (v: string) => void
  platform: string
  setPlatform: (v: string) => void
  appVersion: string
  setAppVersion: (v: string) => void
  installId: string
  setInstallId: (v: string) => void
}) {
  return (
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
  )
}

export function EventFeedPage() {
  const { currentProject } = useAuth()
  const { get, set } = useURLParams()
  const name = get('name')
  const platform = get('platform')
  const appVersion = get('app_version')
  const installId = get('install_id')
  const setName = (v: string) => set('name', v)
  const setPlatform = (v: string) => set('platform', v)
  const setAppVersion = (v: string) => set('app_version', v)
  const setInstallId = (v: string) => set('install_id', v)
  const [paused, setPaused] = useState(false)
  const { events, error, loading, hasMore, loadOlder } = useRecentEventsFeed(
    currentProject,
    { name, platform, appVersion, installId },
    paused,
  )

  if (!currentProject) return <p>Select a project to view its live event feed.</p>

  return (
    <section aria-labelledby="feed-heading">
      <h1 id="feed-heading">Live event feed</h1>
      <p>Newest events first, refreshing every ~5 seconds.</p>
      <FeedFilters
        name={name}
        setName={setName}
        platform={platform}
        setPlatform={setPlatform}
        appVersion={appVersion}
        setAppVersion={setAppVersion}
        installId={installId}
        setInstallId={setInstallId}
      />
      <button type="button" onClick={() => setPaused((p) => !p)}>
        {paused ? 'Resume' : 'Pause'}
      </button>

      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      <DataTable caption="Most recent events" columns={feedColumns} rows={events} getRowKey={(r) => r.event_id} />
      {hasMore && (
        <button type="button" onClick={loadOlder}>
          Load older
        </button>
      )}
    </section>
  )
}
