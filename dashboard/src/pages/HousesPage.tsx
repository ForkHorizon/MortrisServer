import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { EvidenceState, PuzzleHouseList, PuzzleHouseSummary, PuzzleOverview } from '../api/houseTypes'
import type { PuzzleQuality, PuzzleTesterImpact, TrafficScope } from '../api/puzzleQualityTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { RAMP_LEGEND, UNPLAYED, outcomeWords, paintFor, percent } from '../components/houseColors'
import { QualityBanner } from '../components/QualityBanner'
import { StatGrid, StatTile } from '../components/StatTile'
import { TesterImpactPanel } from '../components/TesterImpactPanel'
import { ScopeCaveat, TrafficScopeSelect } from '../components/TrafficScopeSelect'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

const EVIDENCE_LABEL: Record<EvidenceState, string> = {
  unplayed: 'unplayed',
  thin: 'too few plays',
  usable: 'usable evidence',
  mixed_quality: 'usable, tester activity too',
}

function HouseCard({ house, project, revision }: { house: PuzzleHouseSummary; project: string; revision: string }) {
  const paint = paintFor(house.fall_rate, house.rate_is_reliable, house.placements)
  const art = `/api/v1/analytics/gameplay/houses/${house.city_id}/${house.house_id}/art?project=${encodeURIComponent(project)}&revision=${revision}`
  return (
    <Link to={`/houses/${house.city_id}/${house.house_id}`} className="house-card">
      <span className="house-card-art" style={{ background: paint.color }}>
        {house.has_art && <img src={art} alt="" loading="lazy" />}
        <span className="house-card-rate">{house.rate_is_reliable ? percent(house.fall_rate) : '—'}</span>
      </span>
      <span className="house-card-body">
        <strong>{house.display_label || `House ${house.house_id}`}</strong>
        <span className="muted">city {house.city_id} · house {house.house_id} · {EVIDENCE_LABEL[house.evidence_state]}</span>
        <span>{house.rate_is_reliable ? `${percent(house.fall_rate)} of drops fall` : paint.label}</span>
        <span className="muted">
          {house.natural_house_runs_started} natural run{house.natural_house_runs_started === 1 ? '' : 's'} started ·{' '}
          {house.natural_house_runs_started > 0 ? `${percent(house.natural_completion_rate)} completed` : 'none completed'} ·{' '}
          {house.unique_installations} anonymous installs
        </span>
        <span className="muted">wave {house.waves_reached} of {house.total_waves} reached · {house.reliable_detail_count} of {house.played_detail_count} played details are reliable{house.dominant_failure ? ` · mostly ${outcomeWords(house.dominant_failure)}` : ''}{house.has_geometry ? '' : ' · no shapes yet'}</span>
      </span>
    </Link>
  )
}

// verdict is only a designer-facing claim ("needs the closest look") in
// natural_only scope — section 6's rule against letting cheat/mixed
// traffic imply "players struggle."
function GuidedSummary({ overview, scope }: { overview: PuzzleOverview; scope: TrafficScope }) {
  const worst = overview.worst_house
  const topReason = Object.entries(overview.failure_reasons).sort((a, b) => b[1] - a[1])[0]
  const verdict =
    scope !== 'natural_only'
      ? 'Verdicts are only shown for natural traffic — switch scope back to Natural only for a difficulty finding.'
      : worst
        ? `${worst.display_label || `House ${worst.house_id}`} needs the closest look: ${percent(worst.fall_rate)} of its drops fall.`
        : 'There is not enough play yet for a trustworthy difficulty finding.'
  return (
    <section className="guided-summary" aria-label="What to inspect first">
      <h2>Start here</h2>
      <p className="verdict">{verdict}</p>
      <StatGrid>
        <StatTile label="Houses played" value={`${overview.played_houses} of ${overview.total_houses}`} />
        <StatTile label="Trustworthy houses" value={overview.reliable_houses} />
        <StatTile label="Need more data" value={overview.insufficient_houses} />
        <StatTile label="Most common problem" value={topReason ? outcomeWords(topReason[0]) : 'no falls recorded'} />
      </StatGrid>
      {scope === 'natural_only' && overview.worst_wave && <p className="muted">The biggest current drop-off is wave {overview.worst_wave.wave_index + 1}: {percent(overview.worst_wave.fall_rate)} of its drops fall.</p>}
      {overview.data_quality.coordinate_status !== 'trusted' && <p className="data-quality">Some spatial data is legacy. Open a house and use a range after the corrected build before trusting a drop map.</p>}
    </section>
  )
}

function Legend() {
  return <ul className="house-legend">{RAMP_LEGEND.map(({ color, label }) => <li key={label}><span className="house-card-swatch small" style={{ background: color }} aria-hidden="true" /> {label}</li>)}<li><span className="house-card-swatch small" style={{ background: UNPLAYED }} aria-hidden="true" /> too few plays to judge</li></ul>
}

function CityGroup({ city, houses, project, revision }: { city: number; houses: PuzzleHouseSummary[]; project: string; revision: string }) {
  return <section className="city-group"><h2>City {city}</h2><div className="house-grid">{houses.map((house) => <HouseCard key={`${house.city_id}-${house.house_id}`} house={house} project={project} revision={revision} />)}</div></section>
}

function groupByCity(houses: PuzzleHouseSummary[]): Array<[number, PuzzleHouseSummary[]]> {
  const groups = new Map<number, PuzzleHouseSummary[]>()
  for (const house of houses) groups.set(house.city_id, [...(groups.get(house.city_id) ?? []), house])
  return [...groups].sort(([a], [b]) => a - b)
}

// "Worst first" only ranks houses with sufficient evidence (plan section
// 7) — unplayed and thin houses go in the coverage-work list below
// instead, so a designer never mistakes "nobody has tried this" or "two
// people tried this" for an easy house.
const RANKABLE: EvidenceState[] = ['usable', 'mixed_quality']

function sortWorstFirst(houses: PuzzleHouseSummary[]): PuzzleHouseSummary[] {
  return [...houses].sort((a, b) => b.fall_rate - a.fall_rate)
}

export function HousesPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to } = range.params
  const [build, setBuild] = useState('')
  const [scope, setScope] = useState<TrafficScope>('natural_only')
  const fetchHouses = useCallback(() => apiGet<PuzzleHouseList>('/api/v1/analytics/gameplay/houses', { project: currentProject, from, to, scope }), [currentProject, from, to, scope])
  const fetchOverview = useCallback(() => apiGet<PuzzleOverview>('/api/v1/analytics/gameplay/overview', { project: currentProject, from, to, scope }), [currentProject, from, to, scope])
  const fetchQuality = useCallback(() => apiGet<PuzzleQuality>('/api/v1/analytics/gameplay/quality', { project: currentProject, from, to, build: build || undefined }), [currentProject, from, to, build])
  const fetchTesterImpact = useCallback(() => apiGet<PuzzleTesterImpact>('/api/v1/analytics/gameplay/tester-impact', { project: currentProject, from, to }), [currentProject, from, to])
  const houses = useApiData(fetchHouses, `puzzle-houses:${currentProject}:${from}:${to}:${scope}`)
  const overview = useApiData(fetchOverview, `puzzle-overview:${currentProject}:${from}:${to}:${scope}`)
  const quality = useApiData(fetchQuality, `puzzle-quality:${currentProject}:${from}:${to}:${build}`)
  const testerImpact = useApiData(fetchTesterImpact, `puzzle-tester-impact:${currentProject}:${from}:${to}`)
  if (!currentProject) return <p>Select a project to see its houses.</p>
  const all = houses.data?.houses ?? []
  const rankable = sortWorstFirst(all.filter((house) => RANKABLE.includes(house.evidence_state)))
  const byCity = groupByCity(rankable)
  const coverageWork = all.filter((house) => !RANKABLE.includes(house.evidence_state))
  const revision = houses.data?.content_revision ?? ''
  return <section aria-labelledby="houses-heading"><h1 id="houses-heading">Puzzle health</h1><p className="muted">Start with the summary, open a house, then click the detail that needs attention.</p><DateRangeFields range={range} /><TrafficScopeSelect scope={scope} onChange={setScope} /><Freshness loading={houses.loading || overview.loading || quality.loading} error={houses.error || overview.error || quality.error} stale={houses.stale || overview.stale || quality.stale} updatedAt={houses.updatedAt} /><QualityBanner quality={quality.data} error={quality.error} build={build} onBuildChange={setBuild} /><ScopeCaveat scope={scope} /><TesterImpactPanel impact={testerImpact.data} />{overview.data && <GuidedSummary overview={overview.data} scope={scope} />}<Legend />{byCity.map(([city, cityHouses]) => <CityGroup key={`${city}:${build}:${scope}`} city={city} houses={cityHouses} project={currentProject} revision={revision} />)}{coverageWork.length > 0 && <details><summary>{coverageWork.length} houses need more natural play before ranking ({coverageWork.filter((h) => h.evidence_state === 'unplayed').length} unplayed, {coverageWork.filter((h) => h.evidence_state === 'thin').length} too few plays)</summary><div className="house-grid">{coverageWork.map((house) => <HouseCard key={`${house.city_id}-${house.house_id}`} house={house} project={currentProject} revision={revision} />)}</div></details>}</section>
}
