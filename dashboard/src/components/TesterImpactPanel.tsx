import type { PuzzleTesterImpact } from '../api/puzzleQualityTypes'
import { StatGrid, StatTile } from './StatTile'

// Stage 3's "compact tester-impact summary" (docs/puzzle-analytics-remaining-plan.md
// section 6): what developer/tester traffic did in this range, next to —
// never blended into — the natural verdict above it.
export function TesterImpactPanel({ impact }: { impact: PuzzleTesterImpact | null }) {
  if (!impact) return null
  if (impact.developer_actions === 0) return null
  return (
    <details className="tester-impact-panel">
      <summary>
        {impact.developer_actions} developer action{impact.developer_actions === 1 ? '' : 's'} in this range
      </summary>
      <StatGrid>
        <StatTile label="Developer actions" value={impact.developer_actions} />
        <StatTile label="House runs affected" value={impact.affected_house_runs} />
        <StatTile label="Wave attempts affected" value={impact.affected_attempts} />
        <StatTile label="Progress completions" value={impact.progress_completions} />
        <StatTile label="Resets" value={impact.resets} />
        <StatTile label="Failed commands" value={impact.failures} />
        <StatTile label="No-op commands" value={impact.noops} />
      </StatGrid>
      {impact.mutations_missing_before_after > 0 && (
        <p className="data-quality">
          {impact.mutations_missing_before_after} progress mutation
          {impact.mutations_missing_before_after === 1 ? '' : 's'} missing a before/after state hash — a data-quality gap, not a product finding.
        </p>
      )}
      {impact.commands.length > 0 && (
        <table className="quality-rejections">
          <caption>Commands by name and result</caption>
          <thead>
            <tr>
              <th scope="col">Command</th>
              <th scope="col">Result</th>
              <th scope="col">Count</th>
            </tr>
          </thead>
          <tbody>
            {impact.commands.map((c) => (
              <tr key={`${c.command}:${c.result}`}>
                <td>{c.command || '(unnamed)'}</td>
                <td>{c.result.replace('developer_command_', '')}</td>
                <td>{c.count.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </details>
  )
}
