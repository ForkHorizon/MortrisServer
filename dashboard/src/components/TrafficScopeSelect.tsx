import type { TrafficScope } from '../api/houseTypes'

// Stage 3's scope control (docs/puzzle-analytics-remaining-plan.md section 6):
// Natural only is the default and the only scope a verdict may come from;
// the other two are explicitly diagnostic. Never default anywhere but
// natural_only — a caller that forgets this prop still gets it via ''.
const OPTIONS: Array<{ value: TrafficScope; label: string }> = [
  { value: 'natural_only', label: 'Natural only' },
  { value: 'developer_affected', label: 'Developer affected' },
  { value: 'all', label: 'All traffic' },
]

export function TrafficScopeSelect({ scope, onChange }: { scope: TrafficScope; onChange: (scope: TrafficScope) => void }) {
  return (
    <label className="traffic-scope-select">
      Traffic
      <select value={scope} onChange={(e) => onChange(e.target.value as TrafficScope)}>
        {OPTIONS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  )
}

// The plan doc: "When Developer affected or All traffic is selected,
// remove or soften any language claiming 'players struggle' because the
// selected population is no longer ordinary play." Every page that shows
// a verdict sentence should gate it through this instead of rendering it
// unconditionally.
export function ScopeCaveat({ scope }: { scope: TrafficScope }) {
  if (scope === 'natural_only') return null
  return (
    <p className="data-quality" role="note">
      {scope === 'developer_affected'
        ? 'Showing developer/tester traffic only — this is not ordinary play and cannot support a difficulty claim.'
        : 'Showing all traffic, natural and developer-affected mixed — treat every number below as diagnostic, not a verdict.'}
    </p>
  )
}
