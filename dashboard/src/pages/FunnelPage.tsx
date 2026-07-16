import { useState } from 'react'
import { apiGet } from '../api/client'
import type { FunnelResult } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { DataTable } from '../components/DataTable'

export function FunnelPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const [stepsInput, setStepsInput] = useState('level_start, level_end')
  const [windowSeconds, setWindowSeconds] = useState(3600)
  const steps = stepsInput
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

  const canQuery = currentProject && steps.length >= 2 && steps.length <= 5

  const { data, error, loading } = useApiData<FunnelResult>(
    () =>
      canQuery
        ? apiGet<FunnelResult>('/api/v1/analytics/funnel', {
            project: currentProject,
            from: range.params.from,
            to: range.params.to,
            steps: steps.join(','),
            window_seconds: windowSeconds,
          })
        : Promise.reject(new Error('not ready')),
    [currentProject, range.params.from, range.params.to, stepsInput, windowSeconds],
  )

  if (!currentProject) return <p>Select a project to build a funnel.</p>

  return (
    <section aria-labelledby="funnel-heading">
      <h1 id="funnel-heading">Funnel</h1>
      <DateRangeFields range={range} />
      <fieldset>
        <legend>Funnel definition</legend>
        <div className="field">
          <label htmlFor="funnel-steps">Steps (2–5, comma-separated, must be cataloged product events)</label>
          <input
            id="funnel-steps"
            value={stepsInput}
            onChange={(e) => setStepsInput(e.target.value)}
            aria-describedby="funnel-steps-hint"
          />
          <span id="funnel-steps-hint" className="hint">
            e.g. level_start, level_end
          </span>
        </div>
        <div className="field">
          <label htmlFor="funnel-window">Completion window (seconds, from first step)</label>
          <input
            id="funnel-window"
            type="number"
            min={1}
            max={86400}
            value={windowSeconds}
            onChange={(e) => setWindowSeconds(Number(e.target.value))}
          />
        </div>
      </fieldset>

      {!canQuery && <p>Enter 2 to 5 step names to run the funnel.</p>}
      {canQuery && loading && <p role="status">Loading…</p>}
      {canQuery && error && <p role="alert">{error}</p>}
      {canQuery && data && (
        <>
          {data.truncated && (
            <p role="alert">
              This result hit the internal event cap and may undercount — narrow the date range.
            </p>
          )}
          <DataTable
            caption={`Funnel conversion (completion window: ${data.completion_window_seconds}s)`}
            columns={[
              { key: 'name', label: 'Step' },
              { key: 'count', label: 'Installations reached' },
              {
                key: 'conversion_from_first',
                label: '% of step 1',
                render: (r) => `${(r.conversion_from_first * 100).toFixed(1)}%`,
              },
              {
                key: 'conversion_from_previous',
                label: '% of previous step',
                render: (r) => `${(r.conversion_from_previous * 100).toFixed(1)}%`,
              },
            ]}
            rows={data.steps}
            getRowKey={(r) => r.name}
          />
        </>
      )}
    </section>
  )
}
