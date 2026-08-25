import type { PuzzleWaveFunnel } from '../api/puzzleQualityTypes'
import { percent } from './houseColors'

// Stage 4's wave staircase (docs/puzzle-analytics-remaining-plan.md
// section 7): a shrinking ribbon of natural house runs entering each
// wave, so a designer sees where a house loses people without reading a
// table. Clicking a step drives the page's existing wave filter — see
// HouseDetailPage, which threads the same `wave` state into WaveTabs and
// the attempt picker below.
export function WaveStaircase({
  funnel,
  selectedWave,
  onSelectWave,
}: {
  funnel: PuzzleWaveFunnel | null
  selectedWave: number | null
  onSelectWave: (wave: number) => void
}) {
  if (!funnel || funnel.steps.length === 0) return null
  const first = funnel.steps[0]?.entered ?? 0
  if (first === 0) return <p className="muted">No natural house runs have entered this house in this range yet.</p>
  return (
    <div className="wave-staircase" role="group" aria-label="Natural wave funnel">
      <p className="muted">
        Each bar is the share of natural house runs that reached that wave. A run open for longer than {funnel.lost_threshold_hours} hours with no
        further progress counts as lost, not still in progress.
      </p>
      <ol className="staircase-steps">
        {funnel.steps.map((step) => {
          const width = first > 0 ? Math.max((step.entered / first) * 100, step.entered > 0 ? 4 : 0) : 0
          const completedShare = step.entered > 0 ? step.completed / step.entered : 0
          const nextShare = step.entered > 0 ? step.reached_next / step.entered : 0
          return (
            <li key={step.wave_index}>
              <button
                type="button"
                className="staircase-step"
                aria-pressed={selectedWave === step.wave_index}
                onClick={() => onSelectWave(step.wave_index)}
              >
                <span className="staircase-label">
                  Wave {step.wave_index + 1} · {step.entered} entered{step.entered > 0 && ` (${percent(completedShare)} completed)`}
                </span>
                <span className="staircase-bar-track">
                  <span className="staircase-bar" style={{ width: `${width}%` }} />
                </span>
                {step.abandoned_here > 0 || step.recent_here > 0 ? (
                  <span className="muted staircase-drop">
                    {step.abandoned_here > 0 && `${step.abandoned_here} lost here`}
                    {step.abandoned_here > 0 && step.recent_here > 0 && ' · '}
                    {step.recent_here > 0 && `${step.recent_here} still recent`}
                  </span>
                ) : null}
                {step.wave_index < funnel.steps.length - 1 && <span className="muted staircase-transition">{percent(nextShare)} reached wave {step.wave_index + 2}</span>}
              </button>
            </li>
          )
        })}
      </ol>
    </div>
  )
}
