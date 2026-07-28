import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { CatalogEntry, CatalogResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'

const relativeTime = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })

function lastSeenHint(entry: CatalogEntry): string {
  if (!entry.last_seen_at) return 'never seen'
  const ms = new Date(entry.last_seen_at).getTime() - Date.now()
  const hours = Math.round(ms / 3_600_000)
  if (Math.abs(hours) < 1) return 'last seen just now'
  if (Math.abs(hours) < 24) return `last seen ${relativeTime.format(hours, 'hour')}`
  return `last seen ${relativeTime.format(Math.round(hours / 24), 'day')}`
}

// Searchable event-name picker fed by the existing /catalog endpoint —
// zero backend work (per the plan). A native <input list> + <datalist>
// gives free typeahead/search from the browser, so there's no custom
// dropdown/keyboard-nav code to write or maintain.
export function EventSelect({
  id,
  value,
  onChange,
  placeholder,
}: {
  id: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  const { currentProject } = useAuth()
  const fetchCatalog = useCallback(
    () => apiGet<CatalogResult>('/api/v1/analytics/catalog', { project: currentProject }),
    [currentProject],
  )
  const { data } = useApiData<CatalogResult>(fetchCatalog, `catalog:${currentProject}`)
  const listId = `${id}-options`

  return (
    <>
      <input
        id={id}
        list={listId}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder ?? 'level_start'}
      />
      <datalist id={listId}>
        {data?.entries.map((entry) => (
          <option key={entry.name} value={entry.name}>
            {entry.name} — {entry.known ? 'declared' : 'auto-discovered'} · {lastSeenHint(entry)}
          </option>
        ))}
      </datalist>
    </>
  )
}
