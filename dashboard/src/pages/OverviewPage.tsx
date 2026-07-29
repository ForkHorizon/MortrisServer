import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { EventAnomaly, Overview, OverviewDaily } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { combineFreshness, useApiData } from '../hooks/useApiData'
import { useAnomalies } from '../hooks/useAnomalies'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { StatGrid, StatTile, computeDelta } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { AnomalyBadge } from '../components/AnomalyBadge'
import { DigestPanel } from '../components/DigestPanel'
import { Entity } from '../components/Entity'
import { TimeQualityNote } from '../components/TimeQualityNote'

function dailyTrend(daily: OverviewDaily[], key: keyof OverviewDaily) {
  return daily.map((d) => ({ day: d.day, count: Number(d[key]), partial: d.partial }))
}

// The immediately preceding period of the same length — [from, to) is
// half-open, so the previous window ends exactly where this one starts.
function previousPeriodParams(from: string, to: string) {
  const durationMs = new Date(to).getTime() - new Date(from).getTime()
  return { from: new Date(new Date(from).getTime() - durationMs).toISOString(), to: from }
}

function OverviewStats({ data, previous }: { data: Overview; previous: Overview | null }) {
  const delta = (key: keyof Overview) => (previous ? computeDelta(Number(data[key]), Number(previous[key])) : null)
  return (
    <>
      <h2>Product</h2>
      <StatGrid>
        <StatTile
          label="Product events"
          value={data.product_events}
          trend={dailyTrend(data.daily, 'product_events')}
          delta={delta('product_events')}
        />
        <StatTile
          label="New installations"
          value={data.new_installations}
          trend={dailyTrend(data.daily, 'new_installations')}
          delta={delta('new_installations')}
        />
        <StatTile
          label="Daily active installations"
          value={data.daily_active_installations}
          trend={dailyTrend(data.daily, 'daily_active_installations')}
          delta={delta('daily_active_installations')}
        />
        <StatTile
          label="Weekly active installations"
          value={data.weekly_active_installations}
          trend={dailyTrend(data.daily, 'weekly_active_installations')}
          delta={delta('weekly_active_installations')}
        />
        <StatTile
          label="Monthly active installations"
          value={data.monthly_active_installations}
          trend={dailyTrend(data.daily, 'monthly_active_installations')}
          delta={delta('monthly_active_installations')}
        />
        <StatTile label="Sessions" value={data.sessions} trend={dailyTrend(data.daily, 'sessions')} delta={delta('sessions')} />
        <StatTile
          label="Average observed session duration"
          value={`${(data.avg_observed_session_duration_ms / 1000).toFixed(1)}s`}
          trend={dailyTrend(data.daily, 'avg_observed_session_duration_ms')}
          delta={delta('avg_observed_session_duration_ms')}
        />
      </StatGrid>
      <h2>Pipeline health</h2>
      <StatGrid>
        <StatTile
          label="Ingestion accepted"
          value={data.ingestion_accepted}
          trend={dailyTrend(data.daily, 'ingestion_accepted')}
          delta={delta('ingestion_accepted')}
        />
        <StatTile
          label="Ingestion duplicates"
          value={data.ingestion_duplicates}
          trend={dailyTrend(data.daily, 'ingestion_duplicates')}
          delta={delta('ingestion_duplicates')}
        />
        <StatTile
          label="Ingestion rejected"
          value={`${data.ingestion_rejected} (${data.ingestion_rejection_rate.toFixed(1)}%)`}
          trend={dailyTrend(data.daily, 'ingestion_rejected')}
          delta={delta('ingestion_rejected')}
        />
      </StatGrid>
    </>
  )
}

function OverviewEventsByKindChart({ data, timezone }: { data: Overview; timezone: string }) {
  if (data.events_by_kind.length === 0) return null
  return (
    <>
      {data.truncated && (
        <p role="alert">This chart hit its internal 92-day cap and may be missing days — narrow the date range.</p>
      )}
      <TrendChart
        categories={data.events_by_kind.map((d) => (d.partial ? `${d.day} (partial)` : d.day))}
        series={[
          { name: 'Product', data: data.events_by_kind.map((d) => d.product) },
          { name: 'System', data: data.events_by_kind.map((d) => d.system) },
        ]}
        label={`Events per day by kind (${timezone})`}
        type="stacked"
      />
    </>
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
            <Entity type="event" value={a.name} /> <AnomalyBadge anomaly={a} />
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

  const overview = useApiData<Overview>(fetchOverview, `overview:${currentProject}:${from}:${to}:${timezone}`)
  const { data } = overview
  const anomalies = useAnomalies(currentProject)

  const prev = previousPeriodParams(from, to)
  const fetchPreviousOverview = useCallback(
    () => apiGet<Overview>('/api/v1/analytics/overview', { project: currentProject, from: prev.from, to: prev.to, timezone }),
    [currentProject, prev.from, prev.to, timezone],
  )
  // Delta-only signal — kept out of the page's combined freshness stamp so a
  // slow/failed comparison fetch can't make the primary stats look stale.
  const previousOverview = useApiData<Overview>(
    fetchPreviousOverview,
    `overview:${currentProject}:${prev.from}:${prev.to}:${timezone}`,
  )

  if (!currentProject) return <p>Select a project to view its overview.</p>

  // One combined stamp for the banner + tiles below: always the older of
  // the two fetches, so the page never implies the anomaly banner is
  // fresher (or staler) than the stats sitting right under it.
  const freshness = combineFreshness(overview, anomalies)

  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading">Overview</h1>
      {anomalies.data && <AnomaliesBanner anomalies={anomalies.data.anomalies} />}
      <DigestPanel projectID={currentProject} />
      <DateRangeFields range={range} />
      <Freshness {...freshness} />
      {data && <TimeQualityNote pct={data.untrusted_pct} />}
      {data && <OverviewStats data={data} previous={previousOverview.data} />}
      {data && <OverviewEventsByKindChart data={data} timezone={timezone} />}
    </section>
  )
}
