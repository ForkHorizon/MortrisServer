import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'

type CacheEntry<T> = { data: T; updatedAt: number }

// Cross-navigation cache keyed by request identity: a remount or param
// change shows the last-known result immediately (with a freshness stamp)
// while revalidating in the background, instead of blanking to a spinner.
// Capped and LRU-evicted (re-set moves a key to the end; least-recently-used
// falls off first) so keys that churn per keystroke — free-text filters,
// funnel step names — don't grow this without bound over a long session.
const MAX_CACHE_ENTRIES = 200
const cache = new Map<string, CacheEntry<unknown>>()

function cacheSet<T>(key: string, entry: CacheEntry<T>) {
  cache.delete(key)
  cache.set(key, entry)
  if (cache.size > MAX_CACHE_ENTRIES) {
    const oldest = cache.keys().next().value
    if (oldest !== undefined) cache.delete(oldest)
  }
}

// Concurrent callers on the same key — e.g. several EventSelect dropdowns
// on one page, all keyed `catalog:${project}` — share one in-flight
// request instead of each firing its own.
const inFlight = new Map<string, Promise<unknown>>()

function dedupedFetch<T>(key: string, fetcher: () => Promise<T>): Promise<T> {
  const existing = inFlight.get(key) as Promise<T> | undefined
  if (existing) return existing
  const promise = fetcher().finally(() => inFlight.delete(key))
  inFlight.set(key, promise)
  return promise
}

// Every screen below fetches on mount/param-change and needs the same
// loading/error/data/freshness quartet — shared here instead of repeated.
export function useApiData<T>(fetcher: () => Promise<T>, key: string) {
  const initial = cache.get(key) as CacheEntry<T> | undefined
  const [data, setData] = useState<T | null>(initial?.data ?? null)
  const [updatedAt, setUpdatedAt] = useState<number | null>(initial?.updatedAt ?? null)
  const [error, setError] = useState<string | null>(null)
  const [stale, setStale] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const entry = cache.get(key) as CacheEntry<T> | undefined
    setData(entry?.data ?? null)
    setUpdatedAt(entry?.updatedAt ?? null)
    setError(null)
    setStale(false)
    setLoading(true)
    dedupedFetch(key, fetcher)
      .then((d) => {
        if (cancelled) return
        const now = Date.now()
        cacheSet(key, { data: d, updatedAt: now })
        setData(d)
        setUpdatedAt(now)
        setLoading(false)
      })
      .catch((e: unknown) => {
        if (cancelled) return
        setError(e instanceof ApiError ? e.message : 'Something went wrong.')
        setStale(true)
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [fetcher, key])

  return { data, error, loading, updatedAt, stale }
}

type FreshnessSource = { loading: boolean; error?: string | null; stale?: boolean; updatedAt: number | null }

// Combines freshness from several independently-fetched sources shown on
// one page (e.g. Overview's anomaly banner + its stat tiles) into a single
// honest stamp — always the OLDEST of the sources, so the page never
// implies a widget is fresher than data displayed right next to it.
export function combineFreshness(...sources: FreshnessSource[]): FreshnessSource {
  const timestamps = sources.map((s) => s.updatedAt).filter((t): t is number => t !== null)
  return {
    loading: sources.some((s) => s.loading),
    error: sources.map((s) => s.error).find(Boolean) ?? null,
    stale: sources.some((s) => s.stale),
    updatedAt: timestamps.length > 0 ? Math.min(...timestamps) : null,
  }
}
