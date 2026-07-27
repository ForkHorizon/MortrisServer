import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { AnomaliesResult } from '../api/types'
import { useApiData } from './useApiData'

// Today vs. trailing-14-day-median event counts (Phase 5 #2) — not
// date-range scoped, so Overview and Catalog share this fetch instead of
// each wiring their own.
export function useAnomalies(projectID: string) {
  const fetchAnomalies = useCallback(
    () => apiGet<AnomaliesResult>('/api/v1/analytics/anomalies', { project: projectID }),
    [projectID],
  )
  return useApiData<AnomaliesResult>(fetchAnomalies)
}
