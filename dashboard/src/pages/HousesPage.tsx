import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleHouseList, PuzzleHouseSummary, PuzzleOverview } from '../api/houseTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { RAMP_LEGEND, UNPLAYED, outcomeWords, paintFor, percent } from '../components/houseColors'
import { StatGrid, StatTile } from '../components/StatTile'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

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
        <span className="muted">city {house.city_id} · house {house.house_id}</span>
        <span>{house.rate_is_reliable ? `${percent(house.fall_rate)} of drops fall` : paint.label}</span>
        <span className="muted">{house.opened_attempts} opened · {house.completed_attempts} finished · {house.unique_installations} anonymous installs</span>
        <span className="muted">{house.reliable_detail_count} of {house.played_detail_count} played details are reliable{house.dominant_failure ? ` · mostly ${outcomeWords(house.dominant_failure)}` : ''}{house.has_geometry ? '' : ' · no shapes yet'}</span>
      </span>
    </Link>
  )
}

function GuidedSummary({ overview }: { overview: PuzzleOverview }) {
  const worst = overview.worst_house
  const topReason = Object.entries(overview.failure_reasons).sort((a, b) => b[1] - a[1])[0]
  return (
    <section className="guided-summary" aria-label="What to inspect first">
      <h2>Start here</h2>
      <p className="verdict">{worst ? `${worst.display_label || `House ${worst.house_id}`} needs the closest look: ${percent(worst.fall_rate)} of its drops fall.` : 'There is not enough play yet for a trustworthy difficulty finding.'}</p>
      <StatGrid>
        <StatTile label="Houses played" value={`${overview.played_houses} of ${overview.total_houses}`} />
        <StatTile label="Trustworthy houses" value={overview.reliable_houses} />
        <StatTile label="Need more data" value={overview.insufficient_houses} />
        <StatTile label="Most common problem" value={topReason ? outcomeWords(topReason[0]) : 'no falls recorded'} />
      </StatGrid>
      {overview.worst_wave && <p className="muted">The biggest current drop-off is wave {overview.worst_wave.wave_index + 1}: {percent(overview.worst_wave.fall_rate)} of its drops fall.</p>}
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

export function HousesPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to } = range.params
  const fetchHouses = useCallback(() => apiGet<PuzzleHouseList>('/api/v1/analytics/gameplay/houses', { project: currentProject, from, to }), [currentProject, from, to])
  const fetchOverview = useCallback(() => apiGet<PuzzleOverview>('/api/v1/analytics/gameplay/overview', { project: currentProject, from, to }), [currentProject, from, to])
  const houses = useApiData(fetchHouses, `puzzle-houses:${currentProject}:${from}:${to}`)
  const overview = useApiData(fetchOverview, `puzzle-overview:${currentProject}:${from}:${to}`)
  if (!currentProject) return <p>Select a project to see its houses.</p>
  const played = houses.data?.houses.filter((house) => house.placements > 0) ?? []
  const byCity = groupByCity(played)
  const untouched = houses.data?.houses.filter((house) => house.placements === 0) ?? []
  const revision = houses.data?.content_revision ?? ''
  return <section aria-labelledby="houses-heading"><h1 id="houses-heading">Puzzle health</h1><p className="muted">Start with the summary, open a house, then click the detail that needs attention.</p><DateRangeFields range={range} /><Freshness loading={houses.loading || overview.loading} error={houses.error || overview.error} stale={houses.stale || overview.stale} updatedAt={houses.updatedAt} />{overview.data && <GuidedSummary overview={overview.data} />}<Legend />{byCity.map(([city, cityHouses]) => <CityGroup key={city} city={city} houses={cityHouses} project={currentProject} revision={revision} />)}{untouched.length > 0 && <details><summary>{untouched.length} houses nobody has opened in this range</summary><div className="house-grid">{untouched.map((house) => <HouseCard key={`${house.city_id}-${house.house_id}`} house={house} project={currentProject} revision={revision} />)}</div></details>}</section>
}
