import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { FunnelResult, FunnelStep } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'
import { DateRangeFields } from '../components/DateRangeFields'
import { EventSelect } from '../components/EventSelect'
import { DataTable, type Column } from '../components/DataTable'
import { TrendChart } from '../components/TrendChart'

const MIN_STEPS = 2
const MAX_STEPS = 5

const funnelColumns: Column<FunnelStep>[] = [
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
]

// Encapsulates the 2-5 step array's mutation handlers, kept out of the page
// component so its render body doesn't also carry this bookkeeping.
function useFunnelSteps(initial: string[]) {
  const [steps, setSteps] = useState<string[]>(initial)
  const setStep = (index: number, value: string) => setSteps((prev) => prev.map((s, i) => (i === index ? value : s)))
  const addStep = () => setSteps((prev) => (prev.length < MAX_STEPS ? [...prev, ''] : prev))
  const removeStep = (index: number) =>
    setSteps((prev) => (prev.length > MIN_STEPS ? prev.filter((_, i) => i !== index) : prev))
  return { steps, setStep, addStep, removeStep }
}

function FunnelStepsField({
  steps,
  setStep,
  addStep,
  removeStep,
}: {
  steps: string[]
  setStep: (index: number, value: string) => void
  addStep: () => void
  removeStep: (index: number) => void
}) {
  return (
    <div className="field">
      <span>Steps (2–5, must be cataloged product events)</span>
      {steps.map((step, i) => (
        <div key={i} className="funnel-step">
          <label htmlFor={`funnel-step-${i}`}>Step {i + 1}</label>
          <EventSelect id={`funnel-step-${i}`} value={step} onChange={(v) => setStep(i, v)} />
          {steps.length > MIN_STEPS && (
            <button type="button" onClick={() => removeStep(i)} aria-label={`Remove step ${i + 1}`}>
              Remove
            </button>
          )}
        </div>
      ))}
      {steps.length < MAX_STEPS && (
        <button type="button" onClick={addStep}>
          Add step
        </button>
      )}
    </div>
  )
}

function FunnelDefinition({
  steps,
  setStep,
  addStep,
  removeStep,
  windowSeconds,
  setWindowSeconds,
}: {
  steps: string[]
  setStep: (index: number, value: string) => void
  addStep: () => void
  removeStep: (index: number) => void
  windowSeconds: number
  setWindowSeconds: (v: number) => void
}) {
  return (
    <fieldset>
      <legend>Funnel definition</legend>
      <FunnelStepsField steps={steps} setStep={setStep} addStep={addStep} removeStep={removeStep} />
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
  )
}

function FunnelResults({ data }: { data: FunnelResult }) {
  return (
    <>
      {data.truncated && (
        <p role="alert">This result hit the internal event cap and may undercount — narrow the date range.</p>
      )}
      <TrendChart
        categories={data.steps.map((s) => s.name)}
        series={[{ name: 'Installations reached', data: data.steps.map((s) => s.count) }]}
        label="Funnel conversion"
        type="bar"
        horizontal
        height={80 + data.steps.length * 50}
        valueLabel={(value, i) => `${value} (${(data.steps[i].conversion_from_previous * 100).toFixed(1)}%)`}
      />
      <DataTable
        caption={`Funnel conversion (completion window: ${data.completion_window_seconds}s)`}
        columns={funnelColumns}
        rows={data.steps}
        getRowKey={(r) => r.name}
      />
    </>
  )
}

export function FunnelPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { steps, setStep, addStep, removeStep } = useFunnelSteps(['level_start', 'level_end'])
  const [windowSeconds, setWindowSeconds] = useState(3600)

  const canQuery = currentProject && steps.length >= MIN_STEPS && steps.every(Boolean)
  const fetchFunnel = useCallback(
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
    [canQuery, currentProject, range.params.from, range.params.to, steps, windowSeconds],
  )

  const { data, error, loading } = useApiData<FunnelResult>(fetchFunnel)

  if (!currentProject) return <p>Select a project to build a funnel.</p>

  return (
    <section aria-labelledby="funnel-heading">
      <h1 id="funnel-heading">Funnel</h1>
      <DateRangeFields range={range} />
      <FunnelDefinition
        steps={steps}
        setStep={setStep}
        addStep={addStep}
        removeStep={removeStep}
        windowSeconds={windowSeconds}
        setWindowSeconds={setWindowSeconds}
      />

      {!canQuery && <p>Enter 2 to 5 step names to run the funnel.</p>}
      {canQuery && loading && <p role="status">Loading…</p>}
      {canQuery && error && <p role="alert">{error}</p>}
      {canQuery && data && <FunnelResults data={data} />}
    </section>
  )
}
