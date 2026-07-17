import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { Overview } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { StatGrid, StatTile } from '../components/StatTile'

export function OverviewPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to, timezone } = range.params
  const fetchOverview = useCallback(
    () => apiGet<Overview>('/api/v1/analytics/overview', { project: currentProject, from, to, timezone }),
    [currentProject, from, to, timezone],
  )

  const { data, error, loading } = useApiData<Overview>(fetchOverview)

  if (!currentProject) return <p>Select a project to view its overview.</p>

  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading">Overview</h1>
      <DateRangeFields range={range} />
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <StatGrid>
          <StatTile label="Product events" value={data.product_events} />
          <StatTile label="New installations" value={data.new_installations} />
          <StatTile label="Daily active installations" value={data.daily_active_installations} />
          <StatTile label="Weekly active installations" value={data.weekly_active_installations} />
          <StatTile label="Monthly active installations" value={data.monthly_active_installations} />
          <StatTile label="Sessions" value={data.sessions} />
          <StatTile
            label="Average observed session duration"
            value={`${(data.avg_observed_session_duration_ms / 1000).toFixed(1)}s`}
          />
          <StatTile label="Ingestion accepted" value={data.ingestion_accepted} />
          <StatTile label="Ingestion duplicates" value={data.ingestion_duplicates} />
          <StatTile label="Ingestion rejected" value={data.ingestion_rejected} />
        </StatGrid>
      )}
    </section>
  )
}
