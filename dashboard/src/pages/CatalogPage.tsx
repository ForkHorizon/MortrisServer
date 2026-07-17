import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { CatalogResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { DataTable } from '../components/DataTable'

export function CatalogPage() {
  const { currentProject } = useAuth()
  const fetchCatalog = useCallback(
    () => apiGet<CatalogResult>('/api/v1/analytics/catalog', { project: currentProject }),
    [currentProject],
  )

  const { data, error, loading } = useApiData<CatalogResult>(fetchCatalog)

  if (!currentProject) return <p>Select a project to view its event catalog.</p>

  return (
    <section aria-labelledby="catalog-heading">
      <h1 id="catalog-heading">Event catalog</h1>
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <DataTable
          caption="Declared and auto-discovered events"
          columns={[
            { key: 'name', label: 'Name' },
            { key: 'kind', label: 'Kind' },
            {
              key: 'known',
              label: 'Status',
              render: (r) => (r.known ? 'Declared' : 'Auto-discovered (undeclared)'),
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
          ]}
          rows={data.entries}
          getRowKey={(r) => r.name}
        />
      )}
    </section>
  )
}
