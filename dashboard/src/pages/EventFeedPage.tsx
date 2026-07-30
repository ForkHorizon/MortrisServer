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

function FeedResults({
  events,
  freshness,
  hasFilters,
  hasMore,
  loadOlder,
}: {
  events: RecentEvent[]
  freshness: { loading: boolean; error: string | null; stale: boolean; updatedAt: number | null }
  hasFilters: boolean
  hasMore: boolean
  loadOlder: () => void
}) {
  return (
    <>
      <Freshness {...freshness} />
      <DataTable
        caption="Most recent events"
        columns={feedColumns}
        rows={events}
        getRowKey={(r) => r.event_id}
        emptyState={
          !hasFilters && !freshness.loading ? (
            <p>No events yet — SDK setup → this page will show your first event live.</p>
          ) : undefined
        }
      />
      {hasMore && (
        <button type="button" onClick={loadOlder}>
          Load older
        </button>
      )}
    </>
  )
}

export function EventFeedPage() {
  const { currentProject } = useAuth()
  const { get, set } = useURLParams()
  const name = get('name')
  const platform = get('platform')
  const appVersion = get('app_version')
  const installId = get('install_id')
  const [paused, setPaused] = useState(false)
  const { events, error, loading, updatedAt, stale, hasMore, loadOlder } = useRecentEventsFeed(
    currentProject,
    { name, platform, appVersion, installId },
    paused,
  )

  if (!currentProject) return <p>Select a project to view its live event feed.</p>

  return (
    <section aria-labelledby="feed-heading">
      <h1 id="feed-heading">Live event feed</h1>
      <p>Newest events first, refreshing every ~5 seconds. Shows one page at a time — use "Load older" for earlier events.</p>
      <FeedFilters
        name={name}
        setName={(v) => set('name', v)}
        platform={platform}
        setPlatform={(v) => set('platform', v)}
        appVersion={appVersion}
        setAppVersion={(v) => set('app_version', v)}
        installId={installId}
        setInstallId={(v) => set('install_id', v)}
      />
      <button type="button" onClick={() => setPaused((p) => !p)}>
        {paused ? 'Resume' : 'Pause'}
      </button>
      <FeedResults
        events={events}
        freshness={{ loading, error, stale, updatedAt }}
        hasFilters={Boolean(name || platform || appVersion || installId)}
        hasMore={hasMore}
        loadOlder={loadOlder}
      />
    </section>
  )
}
