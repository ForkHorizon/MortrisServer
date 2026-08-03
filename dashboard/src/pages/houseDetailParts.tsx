import { useEffect } from 'react'
import type { ArtMode } from '../components/HouseCanvas'
import type { PuzzleDrop, PuzzleDropMap, PuzzleHouseBlock, PuzzleHouseDetail } from '../api/houseTypes'
import type { HouseControls } from './useHouseControls'
import { AttemptPicker } from '../components/AttemptPicker'
import { HouseCanvas } from '../components/HouseCanvas'
import { outcomeWords, paintFor, ruleSentence, supportColor, supportGroupLabel } from '../components/houseColors'
import { HOUSE_METRICS, type HouseMetric, metricReading, metricTitle } from '../components/houseMetrics'
import { StatGrid, StatTile } from '../components/StatTile'
import { DropNote, houseVerdict } from './houseDetailText'

// The pieces of the house detail view. Split out of HouseDetailPage.tsx
// so the page itself stays a thin shell over the data hook.

export function BlockPanel({ block, metric, showSupport = false }: { block: PuzzleHouseBlock; metric: HouseMetric; showSupport?: boolean }) {
  const reading = metricReading(block, metric)
  const paint = paintFor(reading.ratio, reading.reliable, reading.sample)
  const reasons = Object.entries(block.falls_by_reason).sort((a, b) => b[1] - a[1])
  return (
    <div className="block-panel">
      <h2>Detail {block.block_id}</h2>
      <p className="muted">
        wave {block.wave_index + 1}
        {block.is_ground ? ' · sits on the ground' : ''} · {block.visual_key}
      </p>
      <p className="verdict">
        {reading.sample === 0
          ? 'Nobody has tried to place this detail in this range.'
          : reading.reliable
            ? `${metricTitle(metric)}: ${reading.label} (${reading.sample} samples).`
            : `Only ${reading.sample} samples so far — too few to judge ${metricTitle(metric).toLowerCase()}.`}
      </p>
      <StatGrid>
        <StatTile label="Drops" value={block.placements} />
        <StatTile label={metricTitle(metric)} value={reading.label} />
        <StatTile label="Verdict" value={paint.label} />
      </StatGrid>
      {reasons.length > 0 && (
        <>
          <h3>Why it fell</h3>
          <ul>
            {reasons.map(([outcome, count]) => (
              <li key={outcome}>
                {count}× {outcomeWords(outcome)}
              </li>
            ))}
          </ul>
        </>
      )}
      <h3>Placement rule</h3>
      <p>{ruleSentence(block.block_id, block.required_groups)}</p>
      {showSupport && block.required_groups.length > 0 && <SupportKey groups={block.required_groups} />}
    </div>
  )
}

// The colour key matches the lines drawn on the house, so the sentence
// above and the picture beside it name the same alternatives.
function SupportKey({ groups }: { groups: number[][] }) {
  return (
    <ul className="support-key">
      {groups.map((group, index) => (
        <li key={index}>
          <span className="house-card-swatch small" style={{ background: supportColor(index) }} aria-hidden="true" />
          <strong>{supportGroupLabel(index)}</strong>{' '}
          {group.length === 1 ? `detail ${group[0]}` : `details ${group.join(' and ')} together`}
        </li>
      ))}
    </ul>
  )
}

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

function MetricTabs({ metric, onChange }: { metric: HouseMetric; onChange: (m: HouseMetric) => void }) {
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
    <>
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
    </>
  )
}

type StageProps = {
  blocks: PuzzleHouseBlock[]
  wave: number | null
  selected: number | null
  onSelect: (id: number) => void
  artUrl: string
  artMode: ArtMode
  drops: PuzzleDrop[] | undefined
  label: string
  selectedBlock: PuzzleHouseBlock | null
  showSupport: boolean
  metric: HouseMetric
}

export function HouseStage(p: StageProps) {
  return (
    <div className="house-layout">
      <div className="house-stage">
        <HouseCanvas
          blocks={p.blocks}
          wave={p.wave}
          selected={p.selected}
          onSelect={p.onSelect}
          artUrl={p.artUrl}
          mode={p.artMode}
          drops={p.drops}
          support={p.showSupport && p.selectedBlock ? { targetBlockID: p.selectedBlock.block_id, groups: p.selectedBlock.required_groups } : null}
          metric={p.metric}
          title={`${p.label}, coloured by ${metricTitle(p.metric).toLowerCase()}`}
        />
        <p className="muted">Click any detail to inspect it. Grey means ground, or too few tries to judge.</p>
      </div>
      {p.selectedBlock ? <BlockPanel block={p.selectedBlock} metric={p.metric} showSupport={p.showSupport} /> : <p className="muted">Select a detail to see what happened and what to inspect next.</p>}
    </div>
  )
}

type AttemptsProps = {
  project: string
  city: string
  house: string
  from: string
  to: string
  blocks: PuzzleHouseBlock[]
  label: string
}

export function AttemptsSection(p: AttemptsProps) {
  return (
    <details className="attempts">
      <summary>Watch one attempt play out</summary>
      <p className="muted">
        Step through what one player actually did: grey is already standing, amber is the detail in their hand, red is
        what it was waiting on.
      </p>
      <AttemptPicker {...p} />
    </details>
  )
}


type BodyProps = {
  detail: PuzzleHouseDetail
  project: string
  city: string
  house: string
  from: string
  to: string
  view: HouseControls
  dropMap: PuzzleDropMap | null | undefined
}

export function HouseBody({ detail, project, city, house, from, to, view, dropMap }: BodyProps) {
  const blocks = detail.blocks
  const label = detail.display_label || `House ${house}`
  const worst = [...blocks].filter((b) => b.rate_is_reliable && !b.is_ground).sort((a, b) => b.fall_rate - a.fall_rate)[0]
  const defaultBlock = worst ?? [...blocks].filter((b) => !b.is_ground).sort((a, b) => b.placements - a.placements)[0]
  useEffect(() => { if (view.selected == null && defaultBlock) view.setSelected(defaultBlock.block_id) }, [defaultBlock, view])
  return (
    <>
      <p className="verdict">{houseVerdict(worst)}</p>
      {detail.data_quality.coordinate_status !== 'trusted' && <p className="data-quality">Spatial overlay is limited to corrected data. {detail.data_quality.coordinate_status === 'mixed_coordinate_spaces' ? 'This date range mixes old world-space and corrected house-space drops; start after the corrected build.' : 'These drops predate the corrected content revision and are marked legacy.'}</p>}
      <HouseToolbar
        waveCount={detail.wave_count}
        wave={view.wave}
        onWave={view.setWave}
        artMode={view.artMode}
        onArtMode={view.setArtMode}
        showDrops={view.showDrops}
        onShowDrops={view.setShowDrops}
        showSupport={view.showSupport}
        onShowSupport={view.setShowSupport}
        dropMap={dropMap}
        scoped={view.selected != null}
        metric={view.metric}
        onMetric={view.setMetric}
      />
      <HouseStage
        blocks={blocks}
        wave={view.wave}
        selected={view.selected}
        onSelect={view.setSelected}
        artUrl={`/api/v1/analytics/gameplay/houses/${city}/${house}/art?project=${encodeURIComponent(project)}&revision=${detail.content_revision}`}
        artMode={view.artMode}
        drops={dropMap?.drops}
        label={label}
        selectedBlock={blocks.find((b) => b.block_id === view.selected) ?? null}
        showSupport={view.showSupport}
        metric={view.metric}
      />
      <AttemptsSection project={project} city={city} house={house} from={from} to={to} blocks={blocks} label={label} />
    </>
  )
}
