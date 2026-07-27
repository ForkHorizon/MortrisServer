import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { EventAnomaly, Overview, OverviewDaily } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { useAnomalies } from '../hooks/useAnomalies'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { StatGrid, StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { AnomalyBadge } from '../components/AnomalyBadge'
import { DigestPanel } from '../components/DigestPanel'

function dailyTrend(daily: OverviewDaily[], key: keyof OverviewDaily) {
  return daily.map((d) => ({ day: d.day, count: Number(d[key]) }))
}

function OverviewStats({ data }: { data: Overview }) {
  return (
    <StatGrid>
      <StatTile label="Product events" value={data.product_events} trend={dailyTrend(data.daily, 'product_events')} />
      <StatTile label="New installations" value={data.new_installations} trend={dailyTrend(data.daily, 'new_installations')} />
      <StatTile
        label="Daily active installations"
        value={data.daily_active_installations}
        trend={dailyTrend(data.daily, 'daily_active_installations')}
      />
      <StatTile
        label="Weekly active installations"
        value={data.weekly_active_installations}
        trend={dailyTrend(data.daily, 'weekly_active_installations')}
      />
      <StatTile
        label="Monthly active installations"
        value={data.monthly_active_installations}
        trend={dailyTrend(data.daily, 'monthly_active_installations')}
      />
      <StatTile label="Sessions" value={data.sessions} trend={dailyTrend(data.daily, 'sessions')} />
      <StatTile
        label="Average observed session duration"
        value={`${(data.avg_observed_session_duration_ms / 1000).toFixed(1)}s`}
        trend={dailyTrend(data.daily, 'avg_observed_session_duration_ms')}
      />
      <StatTile label="Ingestion accepted" value={data.ingestion_accepted} trend={dailyTrend(data.daily, 'ingestion_accepted')} />
      <StatTile label="Ingestion duplicates" value={data.ingestion_duplicates} trend={dailyTrend(data.daily, 'ingestion_duplicates')} />
      <StatTile label="Ingestion rejected" value={data.ingestion_rejected} trend={dailyTrend(data.daily, 'ingestion_rejected')} />
    </StatGrid>
  )
}

function OverviewEventsByKindChart({ data }: { data: Overview }) {
  if (data.events_by_kind.length === 0) return null
  return (
    <TrendChart
      categories={data.events_by_kind.map((d) => d.day)}
      series={[
        { name: 'Product', data: data.events_by_kind.map((d) => d.product) },
        { name: 'System', data: data.events_by_kind.map((d) => d.system) },
      ]}
      label="Events per day by kind"
      type="stacked"
    />
  )
}

function AnomaliesBanner({ anomalies }: { anomalies: EventAnomaly[] }) {
  if (anomalies.length === 0) return null
  return (
    <div className="anomalies-banner" role="status">
      <h2>Anomalies today</h2>
      <ul>
        {anomalies.map((a) => (
          <li key={a.name}>
            <strong>{a.name}</strong> <AnomalyBadge anomaly={a} />
          </li>
        ))}
      </ul>
    </div>
  )
}

export function OverviewPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to, timezone } = range.params
  const fetchOverview = useCallback(
    () => apiGet<Overview>('/api/v1/analytics/overview', { project: currentProject, from, to, timezone }),
    [currentProject, from, to, timezone],
  )

  const { data, error, loading } = useApiData<Overview>(fetchOverview)
  const { data: anomalies } = useAnomalies(currentProject)

  if (!currentProject) return <p>Select a project to view its overview.</p>

  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading">Overview</h1>
      {anomalies && <AnomaliesBanner anomalies={anomalies.anomalies} />}
      <DigestPanel projectID={currentProject} />
      <DateRangeFields range={range} />
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && <OverviewStats data={data} />}
      {data && <OverviewEventsByKindChart data={data} />}
    </section>
  )
}
