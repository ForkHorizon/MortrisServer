import type { DayCount } from '../api/types'
import { Sparkline } from './Sparkline'

export function StatGrid({ children }: { children: React.ReactNode }) {
  return <dl className="stat-grid">{children}</dl>
}

export function StatTile({ label, value, trend }: { label: string; value: string | number; trend?: DayCount[] }) {
  return (
    <div className="stat-tile">
      <dt>{label}</dt>
      <dd>{value}</dd>
      {trend && <Sparkline data={trend} />}
    </div>
  )
}
