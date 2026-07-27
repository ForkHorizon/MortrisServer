import type { EventAnomaly } from '../api/types'

function anomalyLabel(a: EventAnomaly): string {
  if (a.pct_change === undefined) {
    return `${a.today_count} today, typically 0`
  }
  const sign = a.pct_change > 0 ? '+' : ''
  return `${sign}${a.pct_change.toFixed(0)}% vs typical`
}

export function AnomalyBadge({ anomaly }: { anomaly: EventAnomaly }) {
  return (
    <span
      className="anomaly-badge"
      title={`${anomaly.today_count} today vs a median of ${anomaly.median} over the trailing 14 days`}
    >
      {anomalyLabel(anomaly)}
    </span>
  )
}
