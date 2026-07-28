import { useCallback, useMemo, useState } from 'react'
import { apiGet } from '../api/client'
import type { CatalogEntry, CatalogResult, EventAnomaly } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { combineFreshness, useApiData } from '../hooks/useApiData'
import { useAnomalies } from '../hooks/useAnomalies'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { DataTable, type Column } from '../components/DataTable'
import { Sparkline } from '../components/Sparkline'
import { AnomalyBadge } from '../components/AnomalyBadge'
import { Entity } from '../components/Entity'

type SortKey = 'name' | 'count'

function catalogColumns(anomaliesByName: Map<string, EventAnomaly>): Column<CatalogEntry>[] {
  return [
  { key: 'name', label: 'Name', render: (r) => <Entity type="event" value={r.name} /> },
  { key: 'kind', label: 'Kind' },
  { key: 'known', label: 'Status', render: (r) => (r.known ? 'Declared' : 'Auto-discovered (undeclared)') },
  { key: 'event_count', label: 'Events (range)' },
  { key: 'percent_of_total', label: '% of total', render: (r) => `${r.percent_of_total.toFixed(1)}%` },
  { key: 'sparkline', label: 'Trend', render: (r) => <Sparkline data={r.sparkline} /> },
  {
    key: 'anomaly',
    label: 'Today vs typical',
    render: (r) => {
      const a = anomaliesByName.get(r.name)
      return a ? <AnomalyBadge anomaly={a} /> : '—'
    },
  },
  {
    key: 'rejection_rate',
    label: 'Rejected',
    render: (r) => (r.rejected_count > 0 ? `${r.rejected_count} (${r.rejection_rate.toFixed(1)}%)` : '—'),
  },
  {
    key: 'drift',
    label: 'Schema drift',
    render: (r) =>
      r.drift && r.drift.length > 0
        ? r.drift.map((d) => `${d.property_key} (${d.count})`).join(', ')
        : '—',
  },
  { key: 'description', label: 'Description', render: (r) => r.description || '—' },
  { key: 'owner', label: 'Owner', render: (r) => r.owner || '—' },
  {
    key: 'properties',
    label: 'Properties',
    render: (r) => (r.properties.length ? r.properties.map((p) => p.name).join(', ') : '—'),
  },
  {
    key: 'last_seen_at',
    label: 'Last seen',
    render: (r) => (r.last_seen_at ? new Date(r.last_seen_at).toLocaleString() : '—'),
  },
  ]
}

function SortControls({ sortKey, setSortKey }: { sortKey: SortKey; setSortKey: (k: SortKey) => void }) {
  return (
    <div className="field">
      <span>Sort by</span>
      <button type="button" disabled={sortKey === 'count'} onClick={() => setSortKey('count')}>
        Volume
      </button>
      <button type="button" disabled={sortKey === 'name'} onClick={() => setSortKey('name')}>
        Name
      </button>
    </div>
  )
}

export function CatalogPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const [sortKey, setSortKey] = useState<SortKey>('count')
  const fetchCatalog = useCallback(
    () =>
      apiGet<CatalogResult>('/api/v1/analytics/catalog', {
        project: currentProject,
        from: range.params.from,
        to: range.params.to,
        timezone: range.params.timezone,
      }),
    [currentProject, range.params.from, range.params.to, range.params.timezone],
  )

  const catalog = useApiData<CatalogResult>(
    fetchCatalog,
    `catalog:${currentProject}:${range.params.from}:${range.params.to}:${range.params.timezone}`,
  )
  const { data } = catalog
  const anomalies = useAnomalies(currentProject)
  const anomaliesByName = useMemo(
    () => new Map(anomalies.data?.anomalies.map((a) => [a.name, a]) ?? []),
    [anomalies.data],
  )
  // Combined so the "today vs typical" column never implies a different
  // freshness than the volume column sitting right next to it.
  const freshness = combineFreshness(catalog, anomalies)
  const sortedEntries = useMemo(() => {
    if (!data) return []
    const entries = [...data.entries]
    return sortKey === 'count'
      ? entries.sort((a, b) => b.event_count - a.event_count)
      : entries.sort((a, b) => a.name.localeCompare(b.name))
  }, [data, sortKey])

  if (!currentProject) return <p>Select a project to view its event catalog.</p>

  return (
    <section aria-labelledby="catalog-heading">
      <h1 id="catalog-heading">Event catalog</h1>
      <DateRangeFields range={range} />
      <SortControls sortKey={sortKey} setSortKey={setSortKey} />
      <Freshness {...freshness} />
      {data && (
        <DataTable
          caption="Declared and auto-discovered events, with volume for the selected range"
          columns={catalogColumns(anomaliesByName)}
          rows={sortedEntries}
          getRowKey={(r) => r.name}
        />
      )}
    </section>
  )
}
