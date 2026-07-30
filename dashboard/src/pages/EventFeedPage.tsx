import { useState } from 'react'
import type { RecentEvent } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useRecentEventsFeed } from '../hooks/useRecentEventsFeed'
import { useURLParams } from '../hooks/useURLParams'
import { EventSelect } from '../components/EventSelect'
import { PlatformSelect, AppVersionSelect } from '../components/DimensionSelects'
import { Freshness } from '../components/Freshness'
import { DataTable, type Column } from '../components/DataTable'
import { Entity } from '../components/Entity'
import { Timestamp } from '../components/Timestamp'
import { PropertiesPreview } from '../components/PropertiesPreview'

const feedColumns: Column<RecentEvent>[] = [
  { key: 'received_at', label: 'Received (server clock)', render: (r) => <Timestamp value={r.received_at} mode="relative" /> },
  { key: 'name', label: 'Event', render: (r) => <Entity type="event" value={r.name} /> },
  { key: 'event_kind', label: 'Kind' },
  { key: 'install_id', label: 'Install', render: (r) => <Entity type="install" value={r.install_id} /> },
  { key: 'platform', label: 'Platform', render: (r) => <Entity type="platform" value={r.platform} /> },
  { key: 'app_version', label: 'Version', render: (r) => <Entity type="app_version" value={r.app_version} /> },
  { key: 'properties', label: 'Properties', render: (r) => <PropertiesPreview properties={r.properties} /> },
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
        <PlatformSelect id="feed-platform" value={platform} onChange={setPlatform} />
      </div>
      <div className="field">
        <label htmlFor="feed-app-version">App version</label>
        <AppVersionSelect id="feed-app-version" value={appVersion} onChange={setAppVersion} />
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
  const { events, error, loading, updatedAt, stale, hasMore, loadOlder } = useRecentEventsFeed(
    currentProject,
    { name, platform, appVersion, installId },
    paused,
  )

  if (!currentProject) return <p>Select a project to view its live event feed.</p>

  const hasFilters = Boolean(name || platform || appVersion || installId)

  return (
    <section aria-labelledby="feed-heading">
      <h1 id="feed-heading">Live event feed</h1>
      <p>Newest events first, refreshing every ~5 seconds. Shows one page at a time — use "Load older" for earlier events.</p>
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

      <Freshness loading={loading} error={error} stale={stale} updatedAt={updatedAt} />
      <DataTable
        caption="Most recent events"
        columns={feedColumns}
        rows={events}
        getRowKey={(r) => r.event_id}
        emptyState={
          !hasFilters && !loading ? (
            <p>No events yet — SDK setup → this page will show your first event live.</p>
          ) : undefined
        }
      />
      {hasMore && (
        <button type="button" onClick={loadOlder}>
          Load older
        </button>
      )}
    </section>
  )
}
