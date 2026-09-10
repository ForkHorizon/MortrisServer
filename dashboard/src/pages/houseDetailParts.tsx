import { useEffect, useState } from 'react'
import type { ArtMode } from '../components/HouseCanvas'
import type { PuzzleDrop, PuzzleDropMap, PuzzleHouseBlock, PuzzleHouseDetail } from '../api/houseTypes'
import type { PuzzleWaveFunnel } from '../api/puzzleQualityTypes'
import type { HouseControls } from './useHouseControls'
import { AttemptPicker } from '../components/AttemptPicker'
import { HouseCanvas } from '../components/HouseCanvas'
import { type HouseMetric, metricTitle } from '../components/houseMetrics'
import { RetryLadderTable } from '../components/RetryLadder'
import { WaveStaircase } from '../components/WaveStaircase'
import { BlockPanel } from './BlockPanel'
import { houseVerdict } from './houseDetailText'
import { HouseToolbar, WaveTabs, ArtModeTabs, MetricTabs } from './HouseToolbar'

// The pieces of the house detail view. Split out of HouseDetailPage.tsx
// so the page itself stays a thin shell over the data hook.
export { BlockPanel, SupportKey } from './BlockPanel'

export { WaveTabs, ArtModeTabs, MetricTabs, HouseToolbar }

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
  onSelectAttempt?: (id: string) => void
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
      {p.selectedBlock ? (
        <BlockPanel
          block={p.selectedBlock}
          metric={p.metric}
          showSupport={p.showSupport}
          onSelectAttempt={p.onSelectAttempt}
        />
      ) : (
        <p className="muted">Select a detail to see what happened and what to inspect next.</p>
      )}
    </div>
  )
}

type RetryLadderSectionProps = {
  blocks: PuzzleHouseBlock[]
  wave: number | null
  selected: number | null
  onSelect: (id: number) => void
  onSelectAttempt?: (id: string) => void
}

export function RetryLadderSection({ blocks, wave, selected, onSelect, onSelectAttempt }: RetryLadderSectionProps) {
  return (
    <details className="retry-ladder-section" open>
      <summary>Retry ladder by detail</summary>
      <p className="muted">
        Distribution of natural interaction attempts before success. Consecutive retries are grouped by detail;
        chains reset when switching to a different detail. Minimum 5 samples required for confident verdicts.
      </p>
      <RetryLadderTable
        blocks={blocks}
        waveIndex={wave}
        selectedBlockId={selected}
        onSelectBlock={onSelect}
        onSelectAttempt={onSelectAttempt}
      />
    </details>
  )
}

type AttemptsProps = {
  project: string
  city: string
  house: string
  from: string
  to: string
  build?: string
  blocks: PuzzleHouseBlock[]
  label: string
  waveIndex?: number | null
  selectedAttempt?: string
  onSelectAttempt?: (id: string) => void
}

export function AttemptsSection(p: AttemptsProps) {
  const [isOpen, setIsOpen] = useState(p.waveIndex != null || Boolean(p.selectedAttempt))

  useEffect(() => {
    if (p.selectedAttempt) {
      setIsOpen(true)
      const el = document.getElementById('attempts-section')
      el?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  }, [p.selectedAttempt])

  useEffect(() => {
    if (p.waveIndex != null) {
      setIsOpen(true)
    }
  }, [p.waveIndex])

  return (
    <details
      id="attempts-section"
      className="attempts"
      open={isOpen}
      onToggle={(e) => setIsOpen(e.currentTarget.open)}
    >
      <summary>Watch one attempt play out</summary>
      <p className="muted">
        Step through what one player actually did: grey is already standing, amber is the detail in their hand, red is
        what it was waiting on.
      </p>
      <AttemptPicker {...p} />
    </details>
  )
}

type WaveFunnelSectionProps = {
  funnel: PuzzleWaveFunnel | null
  wave: number | null
  onWave: (w: number) => void
}

export function WaveFunnelSection({ funnel, wave, onWave }: WaveFunnelSectionProps) {
  return (
    <details className="wave-funnel-section" open>
      <summary>Where natural players stop</summary>
      <WaveStaircase funnel={funnel} selectedWave={wave} onSelectWave={onWave} />
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
  build?: string
  view: HouseControls
  dropMap: PuzzleDropMap | null | undefined
  waveFunnel: PuzzleWaveFunnel | null
}

function HouseOverviewHeader({ worst, dataQuality }: { worst: PuzzleHouseBlock | undefined; dataQuality: PuzzleHouseDetail['data_quality'] }) {
  return (
    <>
      <p className="verdict">{houseVerdict(worst)}</p>
      {dataQuality.coordinate_status !== 'trusted' && (
        <p className="data-quality">
          Spatial overlay is limited to corrected data.{' '}
          {dataQuality.coordinate_status === 'mixed_coordinate_spaces'
            ? 'This date range mixes old world-space and corrected house-space drops; start after the corrected build.'
            : 'These drops predate the corrected content revision and are marked legacy.'}
        </p>
      )}
    </>
  )
}

export function HouseBody({ detail, project, city, house, from, to, build, view, dropMap, waveFunnel }: BodyProps) {
  const [selectedAttempt, setSelectedAttempt] = useState<string>('')
  const blocks = detail.blocks
  const label = detail.display_label || `House ${house}`
  const worst = [...blocks].filter((b) => b.rate_is_reliable && !b.is_ground).sort((a, b) => b.fall_rate - a.fall_rate)[0]
  const defaultBlock = worst ?? [...blocks].filter((b) => !b.is_ground).sort((a, b) => b.placements - a.placements)[0]
  const { selected, setSelected } = view
  const defaultId = defaultBlock?.block_id
  useEffect(() => {
    if (selected == null && defaultId != null) setSelected(defaultId)
  }, [selected, defaultId, setSelected])
  const artUrl = `/api/v1/analytics/gameplay/houses/${city}/${house}/art?project=${encodeURIComponent(project)}&revision=${detail.content_revision}`

  return (
    <>
      <HouseOverviewHeader worst={worst} dataQuality={detail.data_quality} />
      <HouseToolbarAndStage
        detail={detail}
        view={view}
        dropMap={dropMap}
        blocks={blocks}
        artUrl={artUrl}
        label={label}
        onSelectAttempt={setSelectedAttempt}
      />
      <RetryLadderSection blocks={blocks} wave={view.wave} selected={view.selected} onSelect={view.setSelected} onSelectAttempt={setSelectedAttempt} />
      <WaveFunnelSection funnel={waveFunnel} wave={view.wave} onWave={view.setWave} />
      <AttemptsSection
        project={project}
        city={city}
        house={house}
        from={from}
        to={to}
        build={build}
        blocks={blocks}
        label={label}
        waveIndex={view.wave}
        selectedAttempt={selectedAttempt}
        onSelectAttempt={setSelectedAttempt}
      />
    </>
  )
}

type ToolbarAndStageProps = {
  detail: PuzzleHouseDetail
  view: HouseControls
  dropMap: PuzzleDropMap | null | undefined
  blocks: PuzzleHouseBlock[]
  artUrl: string
  label: string
  onSelectAttempt: (id: string) => void
}

function HouseToolbarAndStage(p: ToolbarAndStageProps) {
  const { detail, view, dropMap, blocks, artUrl, label, onSelectAttempt } = p
  return (
    <>
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
        artUrl={artUrl}
        artMode={view.artMode}
        drops={dropMap?.drops}
        label={label}
        selectedBlock={blocks.find((b) => b.block_id === view.selected) ?? null}
        showSupport={view.showSupport}
        metric={view.metric}
        onSelectAttempt={onSelectAttempt}
      />
    </>
  )
}
