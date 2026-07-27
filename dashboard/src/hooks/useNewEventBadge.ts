import { useEffect, useState } from 'react'
import { apiGet } from '../api/client'
import type { CatalogResult } from '../api/types'

const WINDOW_DAYS = 7

function isoDaysAgo(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString().slice(0, 10)
}

// Nav badge: "N new undeclared events this week." Reuses the catalog
// endpoint rather than adding a dedicated one — event_count/sparkline for
// the requested range are ignored, only known/first_seen_at (returned for
// every catalog row regardless of range) matter here.
export function useNewEventBadge(projectID: string): number {
  const [count, setCount] = useState(0)

  useEffect(() => {
    if (!projectID) {
      setCount(0)
      return
    }
    let cancelled = false
    const cutoff = Date.now() - WINDOW_DAYS * 24 * 60 * 60 * 1000
    apiGet<CatalogResult>('/api/v1/analytics/catalog', {
      project: projectID,
      from: `${isoDaysAgo(WINDOW_DAYS)}T00:00:00Z`,
      to: `${isoDaysAgo(0)}T23:59:59Z`,
    })
      .then((result) => {
        if (cancelled) return
        const fresh = result.entries.filter(
          (e) => !e.known && e.first_seen_at && new Date(e.first_seen_at).getTime() >= cutoff,
        )
        setCount(fresh.length)
      })
      .catch(() => {
        if (!cancelled) setCount(0)
      })
    return () => {
      cancelled = true
    }
  }, [projectID])

  return count
}
