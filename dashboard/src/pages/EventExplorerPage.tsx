import { useCallback, useMemo } from 'react'
import { apiGet } from '../api/client'
import type { CatalogResult, EventExplorerResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { useURLParams } from '../hooks/useURLParams'
import { DateRangeFields } from '../components/DateRangeFields'
import { EventSelect } from '../components/EventSelect'
import { StatGrid, StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { DataTable } from '../components/DataTable'

// Only used by the Property Value Inspector below — kept local instead
// of growing the shared api/types.ts for a single consumer.
type PropertyValueCount = { value: string; count: number }
type PropertyValuesResult = { values: PropertyValueCount[] }

// A filtered Event Explorer view, including a property drill-down, is
// shareable and bookmarkable with no server-side "saved reports" table.
function useEventExplorerFilters() {
  const { get, set, setMany } = useURLParams()

  return {
    name: get('name'),
    appVersion: get('app_version'),
    buildNumber: get('build_number'),
    platform: get('platform'),
    propertyKey: get('property_key'),
    propertyValue: get('property_value'),
    setName: (v: string) => setMany({ name: v, property_key: '', property_value: '' }),
    setAppVersion: (v: string) => set('app_version', v),
    setBuildNumber: (v: string) => set('build_number', v),
    setPlatform: (v: string) => set('platform', v),
    setPropertyKey: (v: string) => setMany({ property_key: v, property_value: '' }),
    setPropertyValue: (v: string) => set('property_value', v),
  }
}

type Filters = ReturnType<typeof useEventExplorerFilters> & { propertyOptions: string[] }

function EventExplorerFilters(f: Filters) {
  return (
    <fieldset>
      <legend>Filters</legend>
      <div className="field">
        <label htmlFor="filter-name">Event name</label>
        <EventSelect id="filter-name" value={f.name} onChange={f.setName} />
      </div>
      <div className="field">
        <label htmlFor="filter-app-version">App version</label>
        <input id="filter-app-version" value={f.appVersion} onChange={(e) => f.setAppVersion(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="filter-build">Build number</label>
        <input id="filter-build" value={f.buildNumber} onChange={(e) => f.setBuildNumber(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="filter-platform">Platform</label>
        <input id="filter-platform" value={f.platform} onChange={(e) => f.setPlatform(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="filter-property-key">Property</label>
        <input
          id="filter-property-key"
          list="filter-property-key-options"
          value={f.propertyKey}
          onChange={(e) => f.setPropertyKey(e.target.value)}
          disabled={!f.name}
          placeholder={f.name ? 'house_id' : 'pick an event first'}
        />
        <datalist id="filter-property-key-options">
          {f.propertyOptions.map((p) => (
            <option key={p} value={p} />
          ))}
        </datalist>
      </div>
    </fieldset>
  )
}

type PropertyValueInspectorProps = {
  name: string
  propertyKey: string
  propertyValue: string
  onPickValue: (value: string) => void
  from: string
  to: string
}

function PropertyValueTable({ values, propertyValue, onPickValue }: { values: PropertyValueCount[]; propertyValue: string; onPickValue: (value: string) => void }) {
  return (
    <DataTable
      caption="Top 20 values by count — spot bad or unexpected enum values here"
      columns={[
        {
          key: 'value',
          label: 'Value',
          render: (r) => (
            <button type="button" aria-pressed={r.value === propertyValue} onClick={() => onPickValue(r.value)}>
              {r.value}
              {r.value === propertyValue ? ' (filtering)' : ''}
            </button>
          ),
        },
        { key: 'count', label: 'Count' },
      ]}
      rows={values}
      getRowKey={(r) => r.value}
    />
  )
}

function PropertyValueInspector({ name, propertyKey, propertyValue, onPickValue, from, to }: PropertyValueInspectorProps) {
  const { currentProject } = useAuth()
  const fetchValues = useCallback(
    () => apiGet<PropertyValuesResult>('/api/v1/analytics/events/property-values', { project: currentProject, name, property_key: propertyKey, from, to }),
    [currentProject, name, propertyKey, from, to],
  )
  const { data, error, loading } = useApiData<PropertyValuesResult>(fetchValues)

  return (
    <section aria-labelledby="property-values-heading">
      <h2 id="property-values-heading">
        Top values for {name}.{propertyKey}
      </h2>
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && <PropertyValueTable values={data.values} propertyValue={propertyValue} onPickValue={onPickValue} />}
    </section>
  )
}

function EventExplorerResults({ data }: { data: EventExplorerResult }) {
  return (
    <>
      <StatGrid>
        <StatTile label="Total events" value={data.total_events} />
        <StatTile label="Active installations" value={data.active_installations} />
      </StatGrid>
      <TrendChart categories={data.trend.map((d) => d.day)} series={[{ name: 'Events', data: data.trend.map((d) => d.count) }]} label="Events" />
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
  )
}

export function EventExplorerPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to, timezone } = range.params
  const filters = useEventExplorerFilters()
  const { name, appVersion, buildNumber, platform, propertyKey, propertyValue } = filters

  const fetchCatalog = useCallback(() => apiGet<CatalogResult>('/api/v1/analytics/catalog', { project: currentProject }), [currentProject])
  const { data: catalog } = useApiData<CatalogResult>(fetchCatalog)
  const propertyOptions = useMemo(() => catalog?.entries.find((e) => e.name === name)?.properties.map((p) => p.name) ?? [], [catalog, name])

  const fetchEvents = useCallback(
    () =>
      apiGet<EventExplorerResult>('/api/v1/analytics/events', {
        project: currentProject,
        from,
        to,
        timezone,
        name: name || undefined,
        app_version: appVersion || undefined,
        build_number: buildNumber || undefined,
        platform: platform || undefined,
        // Only sent once a value is picked from the inspector below —
        // the API rejects property_key without a paired property_value.
        property_key: propertyValue ? propertyKey : undefined,
        property_value: propertyValue || undefined,
      }),
    [currentProject, from, to, timezone, name, appVersion, buildNumber, platform, propertyKey, propertyValue],
  )

  const { data, error, loading } = useApiData<EventExplorerResult>(fetchEvents)

  if (!currentProject) return <p>Select a project to explore its events.</p>

  return (
    <section aria-labelledby="events-heading">
      <h1 id="events-heading">Event Explorer</h1>
      <DateRangeFields range={range} />
      <EventExplorerFilters {...filters} propertyOptions={propertyOptions} />
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && <EventExplorerResults data={data} />}
      {name && propertyKey && (
        <PropertyValueInspector name={name} propertyKey={propertyKey} propertyValue={propertyValue} onPickValue={(v) => filters.setPropertyValue(v === propertyValue ? '' : v)} from={from} to={to} />
      )}
    </section>
  )
}
