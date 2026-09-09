import { useEffect, useState } from 'react'
import type { ArtMode } from '../components/HouseCanvas'
import type { PuzzleDrop, PuzzleDropMap, PuzzleHouseBlock, PuzzleHouseDetail } from '../api/houseTypes'
import type { PuzzleWaveFunnel } from '../api/puzzleQualityTypes'
import type { HouseControls } from './useHouseControls'
import { AttemptPicker } from '../components/AttemptPicker'
import { HouseCanvas } from '../components/HouseCanvas'
import { outcomeWords, paintFor, ruleSentence, supportColor, supportGroupLabel } from '../components/houseColors'
import { HOUSE_METRICS, type HouseMetric, metricReading, metricTitle } from '../components/houseMetrics'
import { RetryLadderTable } from '../components/RetryLadder'
import { StatGrid, StatTile } from '../components/StatTile'
import { WaveStaircase } from '../components/WaveStaircase'
import { DropNote, houseVerdict } from './houseDetailText'

// The pieces of the house detail view. Split out of HouseDetailPage.tsx
// so the page itself stays a thin shell over the data hook.

export function BlockPanel({
  block,
  metric,
  showSupport = false,
  onSelectAttempt,
}: {
  block: PuzzleHouseBlock
  metric: HouseMetric
  showSupport?: boolean
  onSelectAttempt?: (id: string) => void
}) {
  const reading = metricReading(block, metric)
  const paint = paintFor(reading.ratio, reading.reliable, reading.sample)
  const reasons = Object.entries(block.falls_by_reason).sort((a, b) => b[1] - a[1])
  const ladder = block.retry_ladder
  const ttp = block.time_to_place

  return (
    <div className="block-panel">
      <h2>Detail {block.block_id}</h2>
      <p className="muted">
        wave {block.wave_index + 1}
        {block.is_ground ? ' · sits on the ground' : ''} · {block.visual_key}
      </p>
      <p className="verdict">
        {reading.sample === 0
          ? block.placements > 0
            ? `${metricTitle(metric)}: 0 successful placements (${block.placements} drops attempted).`
            : 'Nobody has tried to place this detail in this range.'
          : reading.reliable
            ? `${metricTitle(metric)}: ${reading.label} (${reading.sample} samples).`
            : `Only ${reading.sample} samples so far — too few to judge ${metricTitle(metric).toLowerCase()}.`}
      </p>
      <StatGrid>
        <StatTile label="Drops" value={block.placements} />
        <StatTile label={metricTitle(metric)} value={reading.label} />
        <StatTile label="Verdict" value={paint.label} />
      </StatGrid>

      <h3>Time to place distribution</h3>
      {ttp ? (
        ttp.sample_count === 0 && ttp.incomplete_interactions === 0 ? (
          <p className="muted">No timing samples recorded for this detail.</p>
        ) : (
          <>
            <StatGrid>
              <StatTile label="Completed samples" value={ttp.sample_count} />
              <StatTile label="Median active time" value={ttp.sample_count > 0 ? `${(ttp.median_ms / 1000).toFixed(1)}s` : '—'} />
              <StatTile
                label="p75 / p90"
                value={ttp.reliable ? `${(ttp.p75_ms / 1000).toFixed(1)}s / ${(ttp.p90_ms / 1000).toFixed(1)}s` : 'Not enough plays (<5)'}
              />
            </StatGrid>
            {!ttp.reliable && ttp.sample_count > 0 && (
              <p className="muted">Only {ttp.sample_count} completed placement sample(s) — minimum 5 required for confident p75/p90 percentiles.</p>
            )}
            {ttp.incomplete_interactions > 0 && (
              <p className="muted">Incomplete/abandoned interactions: {ttp.incomplete_interactions} (excluded from placement timing).</p>
            )}
          </>
        )
      ) : null}

      <h3>Retry ladder</h3>
      {ladder ? (
        ladder.sample_count === 0 ? (
          <p className="muted">No retry chains recorded for this detail.</p>
        ) : (
          <>
            <StatGrid>
              <StatTile label="Total chains" value={ladder.sample_count} />
              <StatTile label="Median tries" value={ladder.sample_count > 0 && ladder.median_tries > 0 ? ladder.median_tries : '—'} />
              <StatTile
                label="Verdict"
                value={
                  !ladder.reliable
                    ? 'Not enough plays (<5)'
                    : (ladder.never_succeeded / ladder.sample_count) >= 0.25 || ladder.median_tries >= 3
                      ? 'High friction'
                      : ladder.median_tries === 1
                        ? 'Smooth (1st try)'
                        : 'Moderate retries'
                }
              />
            </StatGrid>
            <ul className="retry-breakdown">
              <li>1st try: <strong>{ladder.success_1st_try}</strong> ({formatPercent(ladder.success_1st_try, ladder.sample_count)})</li>
              <li>2nd try: <strong>{ladder.success_2nd_try}</strong> ({formatPercent(ladder.success_2nd_try, ladder.sample_count)})</li>
              <li>3rd try: <strong>{ladder.success_3rd_try}</strong> ({formatPercent(ladder.success_3rd_try, ladder.sample_count)})</li>
              <li>4th+ try: <strong>{ladder.success_4th_plus_try}</strong> ({formatPercent(ladder.success_4th_plus_try, ladder.sample_count)})</li>
              <li>Never placed: <strong>{ladder.never_succeeded}</strong> ({formatPercent(ladder.never_succeeded, ladder.sample_count)})</li>
            </ul>
            {ladder.reliable && (ladder.p75_tries > 0 || ladder.p90_tries > 0) && (
              <p className="muted">Percentiles: p75 = {ladder.p75_tries} tries · p90 = {ladder.p90_tries} tries</p>
            )}
            {ladder.example_attempt_ids && ladder.example_attempt_ids.length > 0 && (
              <div className="example-replays-block">
                <strong>Example replays:</strong>
                <div className="ladder-replay-links">
                  {ladder.example_attempt_ids.map((attId) => (
                    <button
                      key={attId}
                      type="button"
                      className="link-button example-replay-btn"
                      title={`Watch replay for attempt ${attId}`}
                      aria-label={`Watch replay for attempt ${attId.slice(0, 8)}`}
                      onClick={() => onSelectAttempt?.(attId)}
                    >
                      #{attId.slice(0, 8)}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </>
        )
      ) : null}

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

function formatPercent(count: number, total: number): string {
  if (total <= 0) return '0%'
  return `${Math.round((count / total) * 100)}%`
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

export function HouseBody({ detail, project, city, house, from, to, build, view, dropMap, waveFunnel }: BodyProps) {
  const [selectedAttempt, setSelectedAttempt] = useState<string>('')
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
        onSelectAttempt={setSelectedAttempt}
      />
      <RetryLadderSection
        blocks={blocks}
        wave={view.wave}
        selected={view.selected}
        onSelect={view.setSelected}
        onSelectAttempt={setSelectedAttempt}
      />
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
