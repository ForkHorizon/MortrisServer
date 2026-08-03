import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleDrop, PuzzleDropMap, PuzzleHouseBlock, PuzzleHouseDetail } from '../api/houseTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { HouseCanvas } from '../components/HouseCanvas'
import type { ArtMode } from '../components/HouseCanvas'
import { outcomeWords, paintFor, percent, ruleSentence } from '../components/houseColors'
import { StatGrid, StatTile } from '../components/StatTile'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

function BlockPanel({ block }: { block: PuzzleHouseBlock }) {
  const paint = paintFor(block.fall_rate, block.rate_is_reliable, block.placements)
  const reasons = Object.entries(block.falls_by_reason).sort((a, b) => b[1] - a[1])
  return (
    <div className="block-panel">
      <h2>Detail {block.block_id}</h2>
      <p className="muted">
        wave {block.wave_index + 1}
        {block.is_ground ? ' · sits on the ground' : ''} · {block.visual_key}
      </p>
      <p className="verdict">
        {block.placements === 0
          ? 'Nobody has tried to place this detail in this range.'
          : block.rate_is_reliable
            ? `${percent(block.fall_rate)} of drops on this detail end in a fall (${block.falls} of ${block.placements}).`
            : `Only ${block.placements} ${block.placements === 1 ? 'try' : 'tries'} so far — too few to judge. ${block.falls} fell.`}
      </p>
      <StatGrid>
        <StatTile label="Drops" value={block.placements} />
        <StatTile label="Falls" value={block.falls} />
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
    </div>
  )
}

function WaveTabs({ count, wave, onChange }: { count: number; wave: number | null; onChange: (w: number | null) => void }) {
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

function ArtModeTabs({ mode, onChange }: { mode: ArtMode; onChange: (m: ArtMode) => void }) {
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

// Grey has to be explained, not just shown: with a thin sample most of a
// house is grey, and a reader who assumes grey means "fine" draws exactly
// the wrong conclusion.
function houseVerdict(worst: PuzzleHouseBlock | undefined): string {
  return worst
    ? `Detail ${worst.block_id} is the hardest part of this house — ${percent(worst.fall_rate)} of drops on it end in a fall.`
    : 'No detail here has enough plays yet to judge. Grey means too few tries, not a good result.'
}

// The note carries the sample size and the legend, because a scatter of
// eight dots looks the same as a scatter of eight hundred until someone
// says which it is.
function DropNote({ map, scoped }: { map: PuzzleDropMap | null | undefined; scoped: boolean }) {
  if (!map) return <p className="muted">Loading drops…</p>
  const scope = scoped ? 'this detail' : 'every detail in this house'
  // The client sends release points in world coordinates, so they have to
  // be shifted onto the house before they mean anything. When that shift
  // cannot be pinned down, say so rather than plotting a map that looks
  // authoritative and points at the wrong details.
  if (!map.aligned) {
    return (
      <p className="muted">
        {map.alignment_issue === 'inconsistent'
          ? "Can't place these drops on the house — the recorded positions disagree with each other, so no single alignment fits them. Nothing is shown rather than showing it in the wrong place."
          : 'Not enough successful placements in this range to work out where these drops belong on the house. Widen the date range, or wait for more play.'}
      </p>
    )
  }
  if (map.drops.length === 0) return <p className="muted">No recorded drops for {scope} in this range.</p>
  return (
    <p className="muted">
      {map.drops.length} {map.drops.length === 1 ? 'drop' : 'drops'} on {scope}. Each dot is where a player let go; the
      line runs to the slot they were aiming at.
      {map.unresolved_drops > 0 && ` ${map.unresolved_drops} could not be matched to a slot and are not shown.`}
    </p>
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
  dropMap: PuzzleDropMap | null | undefined
  scoped: boolean
}

function HouseToolbar(p: ToolbarProps) {
  return (
    <>
      <WaveTabs count={p.waveCount} wave={p.wave} onChange={p.onWave} />
      <ArtModeTabs mode={p.artMode} onChange={p.onArtMode} />
      <div className="wave-tabs" role="group" aria-label="Drop map">
        <button type="button" aria-pressed={p.showDrops} onClick={() => p.onShowDrops(!p.showDrops)}>
          {p.showDrops ? 'Hide where players let go' : 'Show where players let go'}
        </button>
      </div>
      {p.showDrops && <DropNote map={p.dropMap} scoped={p.scoped} />}
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
}

function HouseStage(p: StageProps) {
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
          title={`${p.label}, coloured by how often each detail falls`}
        />
        <p className="muted">Click any detail to inspect it. Grey means ground, or too few tries to judge.</p>
      </div>
      {p.selectedBlock ? <BlockPanel block={p.selectedBlock} /> : <p className="muted">No detail selected.</p>}
    </div>
  )
}

type ViewArgs = {
  project: string
  city: string | undefined
  house: string | undefined
  from: string
  to: string
  selected: number | null
  showDrops: boolean
}

// Drops are fetched only once asked for, and re-fetched scoped to the
// selected detail — the whole-house cloud is for spotting a pattern, the
// per-detail one for reading it.
function useHouseView({ project, city, house, from, to, selected, showDrops }: ViewArgs) {
  const base = `/api/v1/analytics/gameplay/houses/${city}/${house}`
  const fetchHouse = useCallback(() => apiGet<PuzzleHouseDetail>(base, { project, from, to }), [base, project, from, to])
  const detail = useApiData(fetchHouse, `puzzle-house:${project}:${city}:${house}:${from}:${to}`)
  const fetchDrops = useCallback(
    () =>
      showDrops
        ? apiGet<PuzzleDropMap>(`${base}/drops`, { project, from, to, ...(selected != null ? { block_id: String(selected) } : {}) })
        : Promise.resolve(null),
    [base, showDrops, project, from, to, selected],
  )
  const dropMap = useApiData<PuzzleDropMap | null>(fetchDrops, `puzzle-drops:${project}:${city}:${house}:${from}:${to}:${selected ?? 'all'}:${showDrops}`)
  return { detail, dropMap }
}

export function HouseDetailPage() {
  const { currentProject } = useAuth()
  const { city, house } = useParams()
  const range = useDateRange()
  const { from, to } = range.params
  const [wave, setWave] = useState<number | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [artMode, setArtMode] = useState<ArtMode>('both')
  const [showDrops, setShowDrops] = useState(false)
  const { detail, dropMap } = useHouseView({ project: currentProject, city, house, from, to, selected, showDrops })

  if (!currentProject) return <p>Select a project to inspect a house.</p>
  const blocks = detail.data?.blocks ?? []
  const worst = [...blocks].filter((b) => b.rate_is_reliable && !b.is_ground).sort((a, b) => b.fall_rate - a.fall_rate)[0]
  return (
    <section aria-labelledby="house-heading">
      <p><Link to="/houses">← All houses</Link></p>
      <h1 id="house-heading">{detail.data?.display_label || `House ${house}`}</h1>
      <DateRangeFields range={range} />
      <Freshness loading={detail.loading} error={detail.error} stale={detail.stale} updatedAt={detail.updatedAt} />
      {detail.data && (
        <>
          <p className="verdict">{houseVerdict(worst)}</p>
          <HouseToolbar
            waveCount={detail.data.wave_count}
            wave={wave}
            onWave={setWave}
            artMode={artMode}
            onArtMode={setArtMode}
            showDrops={showDrops}
            onShowDrops={setShowDrops}
            dropMap={dropMap.data}
            scoped={selected != null}
          />
          <HouseStage
            blocks={blocks}
            wave={wave}
            selected={selected}
            onSelect={setSelected}
            artUrl={`/api/v1/analytics/gameplay/houses/${city}/${house}/art?project=${encodeURIComponent(currentProject)}&revision=${detail.data.content_revision}`}
            artMode={artMode}
            drops={dropMap.data?.drops}
            label={detail.data.display_label || `House ${house}`}
            selectedBlock={blocks.find((b) => b.block_id === selected) ?? null}
          />
        </>
      )}
    </section>
  )
}
