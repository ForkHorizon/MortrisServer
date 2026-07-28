import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { CatalogResult } from '../api/types'
import { useApiData } from './useApiData'

const WINDOW_DAYS = 7

function isoDaysAgo(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString().slice(0, 10)
}

// Nav badge: "N new undeclared events this week." Reuses the catalog
// endpoint rather than adding a dedicated one — event_count/sparkline for
// the requested range are ignored, only known/first_seen_at (returned for
// every catalog row regardless of range) matter here. Routed through
// useApiData so it shares the cache/key namespace with every other catalog
// fetch instead of hitting the endpoint on its own timer.
export function useNewEventBadge(projectID: string): number {
  const from = `${isoDaysAgo(WINDOW_DAYS)}T00:00:00Z`
  const to = `${isoDaysAgo(0)}T23:59:59Z`
  const fetchCatalog = useCallback(
    () => (projectID ? apiGet<CatalogResult>('/api/v1/analytics/catalog', { project: projectID, from, to }) : Promise.resolve(null)),
    [projectID, from, to],
  )
  const { data } = useApiData<CatalogResult | null>(fetchCatalog, `catalog:${projectID}:${from}:${to}`)
  if (!data) return 0
  const cutoff = Date.now() - WINDOW_DAYS * 24 * 60 * 60 * 1000
  return data.entries.filter((e) => !e.known && e.first_seen_at && new Date(e.first_seen_at).getTime() >= cutoff).length
}
