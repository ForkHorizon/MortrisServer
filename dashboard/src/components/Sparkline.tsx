import type { DayCount } from '../api/types'

const WIDTH = 80
const HEIGHT = 20

// Inline SVG polyline — no chart library needed for a ~80x20px trend hint
// in a table cell (TrendChart's ECharts instance is far too heavy here).
export function Sparkline({ data }: { data: DayCount[] }) {
  if (data.length < 2) return <span className="hint">—</span>

  const max = Math.max(...data.map((d) => d.count), 1)
  const points = data
    .map((d, i) => {
      const x = (i / (data.length - 1)) * WIDTH
      const y = HEIGHT - (d.count / max) * HEIGHT
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')

  return (
    <svg
      width={WIDTH}
      height={HEIGHT}
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      role="img"
      aria-label={`Trend over ${data.length} days, latest ${data[data.length - 1].count}`}
    >
      <polyline points={points} fill="none" stroke="#2563eb" strokeWidth={1.5} />
    </svg>
  )
}
