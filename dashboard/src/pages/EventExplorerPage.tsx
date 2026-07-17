import { useState } from 'react'
import { apiGet } from '../api/client'
import type { EventExplorerResult } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { StatGrid, StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { DataTable } from '../components/DataTable'

export function EventExplorerPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const [name, setName] = useState('')
  const [appVersion, setAppVersion] = useState('')
  const [buildNumber, setBuildNumber] = useState('')
  const [platform, setPlatform] = useState('')

  const { data, error, loading } = useApiData<EventExplorerResult>(
    () =>
      apiGet<EventExplorerResult>('/api/v1/analytics/events', {
        project: currentProject,
        ...range.params,
        name: name || undefined,
        app_version: appVersion || undefined,
        build_number: buildNumber || undefined,
        platform: platform || undefined,
      }),
    [currentProject, range.params.from, range.params.to, range.params.timezone, name, appVersion, buildNumber, platform],
  )

  if (!currentProject) return <p>Select a project to explore its events.</p>

  return (
    <section aria-labelledby="events-heading">
      <h1 id="events-heading">Event Explorer</h1>
      <DateRangeFields range={range} />
      <fieldset>
        <legend>Filters</legend>
        <div className="field">
          <label htmlFor="filter-name">Event name</label>
          <input id="filter-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="level_start" />
        </div>
        <div className="field">
          <label htmlFor="filter-app-version">App version</label>
          <input id="filter-app-version" value={appVersion} onChange={(e) => setAppVersion(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="filter-build">Build number</label>
          <input id="filter-build" value={buildNumber} onChange={(e) => setBuildNumber(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="filter-platform">Platform</label>
          <input id="filter-platform" value={platform} onChange={(e) => setPlatform(e.target.value)} />
        </div>
      </fieldset>

      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <>
          <StatGrid>
            <StatTile label="Total events" value={data.total_events} />
            <StatTile label="Active installations" value={data.active_installations} />
          </StatGrid>
          <TrendChart data={data.trend} label="Events" />
          <DataTable
            caption="Event count by day"
            columns={[
              { key: 'day', label: 'Day' },
              { key: 'count', label: 'Count' },
            ]}
            rows={data.trend}
            getRowKey={(r) => r.day}
          />
        </>
      )}
    </section>
  )
}
