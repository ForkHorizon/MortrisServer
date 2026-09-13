import { useState } from 'react'
import type { MemoryTimelinePoint, PuzzleMemoryTimeline } from '../api/deviceTypes'
import { DataTable } from './DataTable'
import { Timestamp } from './Timestamp'

interface MemoryTimelineChartProps {
  timeline: PuzzleMemoryTimeline
}

function formatMinutes(ms: number): string {
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  return `${m}m ${s}s`
}

function renderMarkerShape(type: string, cx: number, cy: number) {
  switch (type) {
    case 'memory_sample':
      return <circle cx={cx} cy={cy} r={5} fill="#0d9488" stroke="#ffffff" strokeWidth={1.5} />
    case 'house_start':
    case 'wave_start':
      return <polygon points={`${cx},${cy - 6} ${cx + 5},${cy} ${cx},${cy + 6} ${cx - 5},${cy}`} fill="#4f46e5" stroke="#ffffff" strokeWidth={1.5} />
    case 'checkpoint':
      return <circle cx={cx} cy={cy} r={4} fill="#16a34a" stroke="#ffffff" strokeWidth={1.5} />
    case 'recovery':
      return <polygon points={`${cx},${cy - 6} ${cx + 6},${cy + 5} ${cx - 6},${cy + 5}`} fill="#ca8a04" stroke="#ffffff" strokeWidth={1.5} />
    case 'build_update':
      return <rect x={cx - 4} y={cy - 4} width={8} height={8} fill="#9333ea" stroke="#ffffff" strokeWidth={1.5} />
    case 'background':
      return <rect x={cx - 3} y={cy - 5} width={6} height={10} fill="#64748b" />
    default:
      return <circle cx={cx} cy={cy} r={3} fill="#94a3b8" />
  }
}

function calculateMaxMemory(points: MemoryTimelinePoint[]): number {
  let maxMem = 100
  points.forEach((p) => {
    if (p.app_reserved_mb && p.app_reserved_mb > maxMem) maxMem = p.app_reserved_mb
    if (p.app_allocated_mb && p.app_allocated_mb > maxMem) maxMem = p.app_allocated_mb
    if (p.mono_used_mb && p.mono_used_mb > maxMem) maxMem = p.mono_used_mb
  })
  return Math.ceil((maxMem * 1.2) / 50) * 50
}

interface TimelineSvgProps {
  points: MemoryTimelinePoint[]
  maxActive: number
  maxMem: number
  onSelectPoint: (p: MemoryTimelinePoint) => void
}

function TimelineSvg({ points, maxActive, maxMem, onSelectPoint }: TimelineSvgProps) {
  const width = 760, height = 240, padL = 60, padR = 20, padT = 20, padB = 40
  const plotW = width - padL - padR, plotH = height - padT - padB
  const getX = (activeMS: number) => padL + (activeMS / maxActive) * plotW
  const getY = (mb: number) => padT + plotH - (mb / maxMem) * plotH

  const samplePoints = points.filter((p) => p.app_reserved_mb !== undefined)
  const reservedPath = samplePoints.map((p, i) => `${i === 0 ? 'M' : 'L'} ${getX(p.active_time_ms)} ${getY(p.app_reserved_mb!)}`).join(' ')
  const allocatedPath = samplePoints.map((p, i) => `${i === 0 ? 'M' : 'L'} ${getX(p.active_time_ms)} ${getY(p.app_allocated_mb!)}`).join(' ')
  const monoPath = samplePoints.map((p, i) => `${i === 0 ? 'M' : 'L'} ${getX(p.active_time_ms)} ${getY(p.mono_used_mb!)}`).join(' ')

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', height: 'auto', display: 'block' }} role="img" aria-label="Memory timeline chart">
      <line x1={padL} y1={padT + plotH} x2={padL + plotW} y2={padT + plotH} stroke="var(--border, #cbd5e1)" strokeWidth={1} />
      <line x1={padL} y1={padT} x2={padL} y2={padT + plotH} stroke="var(--border, #cbd5e1)" strokeWidth={1} />
      {[0, 0.5, 1].map((pct) => (
        <g key={pct}>
          <line x1={padL - 4} y1={getY(Math.round(maxMem * pct))} x2={padL + plotW} y2={getY(Math.round(maxMem * pct))} stroke="var(--border, #f1f5f9)" strokeDasharray="2 2" />
          <text x={padL - 8} y={getY(Math.round(maxMem * pct)) + 4} textAnchor="end" fontSize="10" fill="var(--text-muted, #64748b)">{Math.round(maxMem * pct)} MB</text>
        </g>
      ))}
      {reservedPath && <path d={reservedPath} fill="none" stroke="#2563eb" strokeWidth={2} />}
      {allocatedPath && <path d={allocatedPath} fill="none" stroke="#0d9488" strokeWidth={2} strokeDasharray="4 2" />}
      {monoPath && <path d={monoPath} fill="none" stroke="#d97706" strokeWidth={1.5} strokeDasharray="2 2" />}
      {points.map((p) => (
        <g
          key={p.event_id}
          className="timeline-marker-group"
          tabIndex={0}
          role="button"
          aria-label={`${p.marker_label} at active time ${formatMinutes(p.active_time_ms)}`}
          onMouseEnter={() => onSelectPoint(p)}
          onFocus={() => onSelectPoint(p)}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelectPoint(p) } }}
        >
          {renderMarkerShape(p.marker_type, getX(p.active_time_ms), p.app_reserved_mb ? getY(p.app_reserved_mb) : padT + plotH - 12)}
        </g>
      ))}
      <text x={padL} y={height - 10} fontSize="10" fill="var(--text-muted, #64748b)">0m active</text>
      <text x={padL + plotW} y={height - 10} textAnchor="end" fontSize="10" fill="var(--text-muted, #64748b)">{formatMinutes(maxActive)} active</text>
    </svg>
  )
}

function TimelineLegend() {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1rem', marginTop: '0.5rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
      <span><strong style={{ color: '#2563eb' }}>—</strong> App Reserved</span>
      <span><strong style={{ color: '#0d9488' }}>- -</strong> App Allocated</span>
      <span><strong style={{ color: '#d97706' }}>···</strong> Mono Used</span>
      <span><span style={{ color: '#4f46e5' }}>◆</span> House/Wave Start</span>
      <span><span style={{ color: '#16a34a' }}>●</span> Checkpoint</span>
      <span><span style={{ color: '#64748b' }}>▮</span> Background Pause</span>
      <span><span style={{ color: '#a16207' }}>▲</span> Recovery</span>
      <span><span style={{ color: '#9333ea' }}>■</span> Build Update</span>
    </div>
  )
}

function TimelineActivePointDetails({ point }: { point: MemoryTimelinePoint | null }) {
  if (!point) return null
  return (
    <div style={{ marginTop: '0.5rem', padding: '0.5rem', background: 'var(--bg)', border: '1px solid var(--border)', borderRadius: '4px', fontSize: '0.85rem' }}>
      <strong>{point.marker_label}</strong> ({point.event_name}) | Active: {formatMinutes(point.active_time_ms)} | Wall: {formatMinutes(point.wall_time_ms)}
      {point.app_reserved_mb !== undefined && (
        <span style={{ marginLeft: '0.5rem' }}>
          Allocated: {point.app_allocated_mb} MB | Reserved: {point.app_reserved_mb} MB | Mono: {point.mono_used_mb} MB
        </span>
      )}
      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>
        Time: {point.effective_at} | Build: {point.build_number}
      </div>
    </div>
  )
}

function TimelineTable({ points }: { points: MemoryTimelinePoint[] }) {
  return (
    <div style={{ marginTop: '1rem' }}>
      <DataTable
        caption="Sequential Timeline Points & Lifecycle Markers"
        rows={points}
        getRowKey={(r) => r.event_id}
        columns={[
          { key: 'active', label: 'Active Time', render: (r) => formatMinutes(r.active_time_ms) },
          { key: 'wall', label: 'Wall Time', render: (r) => formatMinutes(r.wall_time_ms) },
          { key: 'type', label: 'Marker', render: (r) => r.marker_label },
          { key: 'event', label: 'Event', render: (r) => r.event_name },
          { key: 'reserved', label: 'Reserved MB', render: (r) => r.app_reserved_mb ?? '—' },
          { key: 'allocated', label: 'Allocated MB', render: (r) => r.app_allocated_mb ?? '—' },
          { key: 'mono', label: 'Mono MB', render: (r) => r.mono_used_mb ?? '—' },
          { key: 'build', label: 'Build', render: (r) => r.build_number },
          { key: 'time', label: 'Effective At', render: (r) => <Timestamp value={r.effective_at} mode="absolute" /> },
        ]}
      />
    </div>
  )
}

export function MemoryTimelineChart({ timeline }: MemoryTimelineChartProps) {
  const [activePoint, setActivePoint] = useState<MemoryTimelinePoint | null>(null)
  const [showTable, setShowTable] = useState(false)

  const points = timeline.points || []
  if (points.length === 0) {
    return <p>No telemetry events recorded for this device.</p>
  }

  const maxActive = Math.max(timeline.total_active_time_ms, 1)
  const maxMem = calculateMaxMemory(points)

  return (
    <div className="memory-timeline-view" style={{ marginTop: '1rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
        <div>
          <span style={{ marginRight: '1rem' }}><strong>Active:</strong> {formatMinutes(timeline.total_active_time_ms)}</span>
          <span style={{ marginRight: '1rem' }}><strong>Wall time:</strong> {formatMinutes(timeline.total_wall_time_ms)}</span>
          <span><strong>Memory Samples:</strong> {timeline.sample_count}</span>
        </div>
        <button type="button" onClick={() => setShowTable(!showTable)}>
          {showTable ? 'Hide Event Table' : 'Show Event Table'}
        </button>
      </div>

      <div style={{ position: 'relative', overflowX: 'auto', background: 'var(--bg-alt)', border: '1px solid var(--border)', borderRadius: '4px', padding: '0.5rem' }}>
        <TimelineSvg points={points} maxActive={maxActive} maxMem={maxMem} onSelectPoint={setActivePoint} />
        <TimelineLegend />
        <TimelineActivePointDetails point={activePoint} />
      </div>

      {showTable && <TimelineTable points={points} />}
    </div>
  )
}
