import { useCallback, useEffect, useState } from 'react'
import { apiGet, ApiError } from '../api/client'
import type { RecentEvent, RecentEventsResult } from '../api/types'

const POLL_MS = 5000

export interface RecentEventsFilters {
  name: string
  platform: string
  appVersion: string
  installId: string
}

function buildParams(currentProject: string, f: RecentEventsFilters) {
  return {
    project: currentProject,
    name: f.name || undefined,
    platform: f.platform || undefined,
    app_version: f.appVersion || undefined,
    install_id: f.installId || undefined,
  }
}

// Polls the Live Event Feed endpoint every ~5s (unless paused) and exposes
// a "load older" page fetched via the keyset cursor returned by the API.
// Pulled out of EventFeedPage so that component stays render-only.
export function useRecentEventsFeed(currentProject: string, filters: RecentEventsFilters, paused: boolean) {
  const [events, setEvents] = useState<RecentEvent[]>([])
  const [cursor, setCursor] = useState<{ receivedAt?: string; eventId?: string }>({})
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const { name, platform, appVersion, installId } = filters

  const refresh = useCallback(() => {
    if (!currentProject) return Promise.resolve()
    return apiGet<RecentEventsResult>('/api/v1/analytics/events/recent', buildParams(currentProject, filters))
      .then((res) => {
        setEvents(res.events)
        setCursor({ receivedAt: res.next_before_received_at, eventId: res.next_before_event_id })
        setError(null)
      })
      .catch((e: unknown) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'))
    // eslint-disable-next-line react-hooks/exhaustive-deps -- filters destructured above so deps stay primitive
  }, [currentProject, name, platform, appVersion, installId])

  // Refetch on mount/filter change; poll every ~5s unless paused. The next
  // poll always replaces the feed with the newest page, which is the point
  // of a live tail — "load older" below is only meaningful while paused.
  useEffect(() => {
    setLoading(true)
    refresh().finally(() => setLoading(false))
  }, [refresh])
  useEffect(() => {
    if (paused) return
    const timer = setInterval(refresh, POLL_MS)
    return () => clearInterval(timer)
  }, [paused, refresh])

  const loadOlder = () => {
    if (!currentProject || !cursor.receivedAt || !cursor.eventId) return
    apiGet<RecentEventsResult>('/api/v1/analytics/events/recent', {
      ...buildParams(currentProject, filters),
      before_received_at: cursor.receivedAt,
      before_event_id: cursor.eventId,
    }).then((res) => {
      setEvents((prev) => [...prev, ...res.events])
      setCursor({ receivedAt: res.next_before_received_at, eventId: res.next_before_event_id })
    })
  }

  return { events, error, loading, hasMore: Boolean(cursor.receivedAt), loadOlder }
}
