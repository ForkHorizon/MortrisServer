import { apiGet } from '../api/client'
import type { RetentionResult } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { DataTable } from '../components/DataTable'

function pct(retained: number, cohort: number): string {
  if (cohort === 0) return '—'
  return `${((retained / cohort) * 100).toFixed(1)}%`
}

export function RetentionPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()

  const { data, error, loading } = useApiData<RetentionResult>(
    () => apiGet<RetentionResult>('/api/v1/analytics/retention', { project: currentProject, ...range.params }),
    [currentProject, range.params.from, range.params.to, range.params.timezone],
  )

  if (!currentProject) return <p>Select a project to view retention.</p>

  return (
    <section aria-labelledby="retention-heading">
      <h1 id="retention-heading">Installation retention</h1>
      <p>
        Cohorts are anonymous <strong>installations</strong>, not people — a reinstall or cleared app data starts a
        new cohort member rather than extending an old one.
      </p>
      <DateRangeFields range={range} />
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <DataTable
          caption="D1 / D7 / D30 retention by cohort day"
          columns={[
            { key: 'cohort_day', label: 'Cohort day' },
            { key: 'cohort_size', label: 'Installations' },
            { key: 'd1', label: 'D1', render: (r) => `${r.d1} (${pct(r.d1, r.cohort_size)})` },
            { key: 'd7', label: 'D7', render: (r) => `${r.d7} (${pct(r.d7, r.cohort_size)})` },
            { key: 'd30', label: 'D30', render: (r) => `${r.d30} (${pct(r.d30, r.cohort_size)})` },
          ]}
          rows={data.cohorts}
          getRowKey={(r) => r.cohort_day}
        />
      )}
    </section>
  )
}
