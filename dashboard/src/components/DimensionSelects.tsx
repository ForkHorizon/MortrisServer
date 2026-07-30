import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { DimensionsResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'

function useDimensions() {
  const { currentProject } = useAuth()
  const fetchDimensions = useCallback(
    () => apiGet<DimensionsResult>('/api/v1/analytics/dimensions', { project: currentProject }),
    [currentProject],
  )
  return useApiData<DimensionsResult>(fetchDimensions, `dimensions:${currentProject}`)
}

// Platform and app_version are a tiny, known value set per project — a
// native <select> of observed values beats free text that only turns up
// results on an exact, un-mistyped match.
export function PlatformSelect({ id, value, onChange }: { id: string; value: string; onChange: (value: string) => void }) {
  const { data } = useDimensions()
  return (
    <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">All platforms</option>
      {data?.platforms.map((p) => (
        <option key={p} value={p}>
          {p}
        </option>
      ))}
    </select>
  )
}

export function AppVersionSelect({ id, value, onChange }: { id: string; value: string; onChange: (value: string) => void }) {
  const { data } = useDimensions()
  return (
    <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">All versions</option>
      {data?.app_versions.map((v) => (
        <option key={v} value={v}>
          {v}
        </option>
      ))}
    </select>
  )
}
