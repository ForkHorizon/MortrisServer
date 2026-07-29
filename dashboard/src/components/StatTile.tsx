import type { DayCount } from '../api/types'
import { Sparkline } from './Sparkline'

export function StatGrid({ children }: { children: React.ReactNode }) {
  return <dl className="stat-grid">{children}</dl>
}

export type Delta = { pct: number; direction: 'up' | 'down' | 'flat' }

// null when `previous` is 0 — a percent change against zero is undefined,
// not "infinite", so the tile should just omit the delta rather than lie.
export function computeDelta(current: number, previous: number): Delta | null {
  if (previous === 0) return null
  const pct = ((current - previous) / previous) * 100
  const direction = pct > 0.05 ? 'up' : pct < -0.05 ? 'down' : 'flat'
  return { pct, direction }
}

const DELTA_ARROW: Record<Delta['direction'], string> = { up: '▲', down: '▼', flat: '–' }

export function StatTile({
  label,
  value,
  trend,
  delta,
}: {
  label: string
  value: string | number
  trend?: DayCount[]
  delta?: Delta | null
}) {
  return (
    <div className="stat-tile">
      <dt>{label}</dt>
      <dd>{value}</dd>
      {delta && (
        <span className="stat-delta">
          {DELTA_ARROW[delta.direction]} {Math.abs(delta.pct).toFixed(1)}% vs previous period
        </span>
      )}
      {trend && <Sparkline data={trend} />}
    </div>
  )
}
