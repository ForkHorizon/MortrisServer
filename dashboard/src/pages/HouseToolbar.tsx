import type { ArtMode } from '../components/HouseCanvas'
import type { PuzzleDropMap } from '../api/houseTypes'
import { HOUSE_METRICS, type HouseMetric } from '../components/houseMetrics'
import { DropNote } from './houseDetailText'

export function WaveTabs({ count, wave, onChange }: { count: number; wave: number | null; onChange: (w: number | null) => void }) {
  return (
    <div className="wave-tabs" role="group" aria-label="Filter by wave">
      <button type="button" aria-pressed={wave === null} onClick={() => onChange(null)}>
        Whole house
      </button>
      {Array.from({ length: count }, (_, i) => (
        <button key={i} type="button" aria-pressed={wave === i} onClick={() => onChange(i)}>
          Wave {i + 1}
        </button>
      ))}
    </div>
  )
}

const ART_MODE_LABEL: Record<ArtMode, string> = {
  both: 'Art + diagram',
  art: 'Art only',
  diagram: 'Diagram only',
}

export function ArtModeTabs({ mode, onChange }: { mode: ArtMode; onChange: (m: ArtMode) => void }) {
  return (
    <div className="wave-tabs" role="group" aria-label="What to show">
      {(Object.keys(ART_MODE_LABEL) as ArtMode[]).map((m) => (
        <button key={m} type="button" aria-pressed={mode === m} onClick={() => onChange(m)}>
          {ART_MODE_LABEL[m]}
        </button>
      ))}
    </div>
  )
}

export function MetricTabs({ metric, onChange }: { metric: HouseMetric; onChange: (m: HouseMetric) => void }) {
  return (
    <div className="control-row" role="group" aria-label="What to measure">
      <strong>Measure</strong>
      {HOUSE_METRICS.map((item) => (
        <button key={item.id} type="button" aria-pressed={metric === item.id} onClick={() => onChange(item.id)}>{item.label}</button>
      ))}
    </div>
  )
}

type ToolbarProps = {
  waveCount: number
  wave: number | null
  onWave: (w: number | null) => void
  artMode: ArtMode
  onArtMode: (m: ArtMode) => void
  showDrops: boolean
  onShowDrops: (v: boolean) => void
  showSupport: boolean
  onShowSupport: (v: boolean) => void
  dropMap: PuzzleDropMap | null | undefined
  scoped: boolean
  metric: HouseMetric
  onMetric: (metric: HouseMetric) => void
}

export function HouseToolbar(p: ToolbarProps) {
  return (
    <div className="house-toolbar">
      <div className="control-row"><strong>Wave</strong><WaveTabs count={p.waveCount} wave={p.wave} onChange={p.onWave} /></div>
      <MetricTabs metric={p.metric} onChange={p.onMetric} />
      <div className="control-row"><strong>Visual layer</strong><ArtModeTabs mode={p.artMode} onChange={p.onArtMode} /></div>
      <div className="control-row" role="group" aria-label="Drop map">
        <strong>Overlay</strong>
        <button type="button" aria-pressed={p.showDrops} onClick={() => p.onShowDrops(!p.showDrops)}>
          {p.showDrops ? 'Hide where players let go' : 'Show where players let go'}
        </button>
      </div>
      <div className="control-row" role="group" aria-label="Support graph">
        <button type="button" aria-pressed={p.showSupport} onClick={() => p.onShowSupport(!p.showSupport)}>
          {p.showSupport ? 'Hide what holds it up' : 'Show what holds it up'}
        </button>
      </div>
      {p.showDrops && <DropNote map={p.dropMap} scoped={p.scoped} />}
      {p.showSupport && !p.scoped && <p className="muted">Pick a detail to see what has to be standing before it.</p>}
    </div>
  )
}
