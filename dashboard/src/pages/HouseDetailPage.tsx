import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleHouseBlock, PuzzleHouseDetail } from '../api/houseTypes'
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

export function HouseDetailPage() {
  const { currentProject } = useAuth()
  const { city, house } = useParams()
  const range = useDateRange()
  const { from, to } = range.params
  const [wave, setWave] = useState<number | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [artMode, setArtMode] = useState<ArtMode>('both')
  const fetchHouse = useCallback(() => apiGet<PuzzleHouseDetail>(`/api/v1/analytics/gameplay/houses/${city}/${house}`, { project: currentProject, from, to }), [currentProject, city, house, from, to])
  const detail = useApiData(fetchHouse, `puzzle-house:${currentProject}:${city}:${house}:${from}:${to}`)

  if (!currentProject) return <p>Select a project to inspect a house.</p>
  const blocks = detail.data?.blocks ?? []
  const selectedBlock = blocks.find((b) => b.block_id === selected) ?? null
  const worst = [...blocks].filter((b) => b.rate_is_reliable && !b.is_ground).sort((a, b) => b.fall_rate - a.fall_rate)[0]
  return (
    <section aria-labelledby="house-heading">
      <p>
        <Link to="/houses">← All houses</Link>
      </p>
      <h1 id="house-heading">{detail.data?.display_label || `House ${house}`}</h1>
      <DateRangeFields range={range} />
      <Freshness loading={detail.loading} error={detail.error} stale={detail.stale} updatedAt={detail.updatedAt} />
      {detail.data && (
        <>
          <p className="verdict">{houseVerdict(worst)}</p>
          <WaveTabs count={detail.data.wave_count} wave={wave} onChange={setWave} />
          <ArtModeTabs mode={artMode} onChange={setArtMode} />
          <div className="house-layout">
            <div className="house-stage">
              <HouseCanvas
                blocks={blocks}
                wave={wave}
                selected={selected}
                onSelect={setSelected}
                artUrl={`/api/v1/analytics/gameplay/houses/${city}/${house}/art?project=${encodeURIComponent(currentProject)}&revision=${detail.data.content_revision}`}
                mode={artMode}
                title={`${detail.data.display_label || `House ${house}`}, coloured by how often each detail falls`}
              />
              <p className="muted">Click any detail to inspect it. Grey means ground, or too few tries to judge.</p>
            </div>
            {selectedBlock ? <BlockPanel block={selectedBlock} /> : <p className="muted">No detail selected.</p>}
          </div>
        </>
      )}
    </section>
  )
}
