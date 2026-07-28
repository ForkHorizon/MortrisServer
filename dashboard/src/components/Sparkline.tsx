import type { DayCount } from '../api/types'

const WIDTH = 80
const HEIGHT = 20

// Inline SVG polyline — no chart library needed for a ~80x20px trend hint
// in a table cell (TrendChart's ECharts instance is far too heavy here).
export function Sparkline({ data }: { data: DayCount[] }) {
  if (data.length < 2) return <span className="hint">—</span>

  const max = Math.max(...data.map((d) => d.count), 1)
  const coords = data.map((d, i) => ({
    x: (i / (data.length - 1)) * WIDTH,
    y: HEIGHT - (d.count / max) * HEIGHT,
  }))
  const points = coords.map(({ x, y }) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
  const last = data[data.length - 1]
  const lastPoint = coords[coords.length - 1]

  return (
    <svg
      width={WIDTH}
      height={HEIGHT}
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      role="img"
      aria-label={`Trend over ${data.length} days, latest ${last.count}${last.partial ? ' (partial day, still in progress)' : ''}`}
    >
      <polyline points={points} fill="none" stroke="#2563eb" strokeWidth={1.5} />
      {last.partial && (
        <circle cx={lastPoint.x} cy={lastPoint.y} r={2.5} fill="white" stroke="#2563eb" strokeWidth={1.5} />
      )}
    </svg>
  )
}
