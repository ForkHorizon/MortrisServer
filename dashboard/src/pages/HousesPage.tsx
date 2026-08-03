import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleHouseList, PuzzleHouseSummary } from '../api/houseTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { RAMP_LEGEND, paintFor, percent } from '../components/houseColors'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

function houseVerdict(houses: PuzzleHouseSummary[]): string {
  const played = houses.filter((h) => h.placements > 0)
  if (played.length === 0) return 'No house has been played in this range yet.'
  const reliable = played.filter((h) => h.rate_is_reliable)
  const worst = [...reliable].sort((a, b) => b.fall_rate - a.fall_rate)[0]
  const head = `${played.length} of ${houses.length} houses have been played.`
  if (!worst) return `${head} None has enough plays yet to judge how hard it is.`
  return `${head} ${worst.display_label || `House ${worst.house_id}`} is the hardest — ${percent(worst.fall_rate)} of placements there end in a fall.`
}

function HouseCard({ house }: { house: PuzzleHouseSummary }) {
  const paint = paintFor(house.fall_rate, house.rate_is_reliable, house.placements)
  return (
    <Link to={`/houses/${house.city_id}/${house.house_id}`} className="house-card">
      <span className="house-card-swatch" style={{ background: paint.color }} aria-hidden="true" />
      <span className="house-card-body">
        <strong>{house.display_label || `House ${house.house_id}`}</strong>
        <span className="muted">city {house.city_id} · house {house.house_id}</span>
        <span>
          {house.rate_is_reliable ? `${percent(house.fall_rate)} of drops fall` : paint.label}
        </span>
        <span className="muted">
          {house.attempts} {house.attempts === 1 ? 'attempt' : 'attempts'} · {house.placements} drops
          {house.has_geometry ? '' : ' · no shapes yet'}
        </span>
      </span>
    </Link>
  )
}

export function HousesPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to } = range.params
  const fetchHouses = useCallback(
    () => apiGet<PuzzleHouseList>('/api/v1/analytics/gameplay/houses', { project: currentProject, from, to }),
    [currentProject, from, to],
  )
  const houses = useApiData(fetchHouses, `puzzle-houses:${currentProject}:${from}:${to}`)

  if (!currentProject) return <p>Select a project to see its houses.</p>
  const played = houses.data?.houses.filter((h) => h.placements > 0) ?? []
  const untouched = houses.data?.houses.filter((h) => h.placements === 0) ?? []
  return (
    <section aria-labelledby="houses-heading">
      <h1 id="houses-heading">Houses</h1>
      <DateRangeFields range={range} />
      <Freshness loading={houses.loading} error={houses.error} stale={houses.stale} updatedAt={houses.updatedAt} />
      {houses.data && (
        <>
          <p className="verdict">{houseVerdict(houses.data.houses)}</p>
          <ul className="house-legend">
            {RAMP_LEGEND.map(({ color, label }) => (
              <li key={label}>
                <span className="house-card-swatch small" style={{ background: color }} aria-hidden="true" /> {label}
              </li>
            ))}
            <li>
              <span className="house-card-swatch small" style={{ background: '#9aa1ad' }} aria-hidden="true" /> too few
              plays to judge
            </li>
          </ul>
          <div className="house-grid">
            {played.map((house) => (
              <HouseCard key={`${house.city_id}-${house.house_id}`} house={house} />
            ))}
          </div>
          {untouched.length > 0 && (
            <details>
              <summary>{untouched.length} houses nobody has opened in this range</summary>
              <div className="house-grid">
                {untouched.map((house) => (
                  <HouseCard key={`${house.city_id}-${house.house_id}`} house={house} />
                ))}
              </div>
            </details>
          )}
        </>
      )}
    </section>
  )
}
