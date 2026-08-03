import { useCallback } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleDropMap, PuzzleHouseDetail } from '../api/houseTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { HouseBody } from './houseDetailParts'
import { useHouseControls } from './useHouseControls'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

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
  const view = useHouseControls()
  const { detail, dropMap } = useHouseView({ project: currentProject, city, house, from, to, selected: view.selected, showDrops: view.showDrops })

  if (!currentProject) return <p>Select a project to inspect a house.</p>
  return (
    <section aria-labelledby="house-heading">
      <p><Link to="/houses">← All houses</Link></p>
      <h1 id="house-heading">{detail.data?.display_label || `House ${house}`}</h1>
      <DateRangeFields range={range} />
      <Freshness loading={detail.loading} error={detail.error} stale={detail.stale} updatedAt={detail.updatedAt} />
      {detail.data && (
        <HouseBody
          detail={detail.data}
          project={currentProject}
          city={city ?? ''}
          house={house ?? ''}
          from={from}
          to={to}
          view={view}
          dropMap={dropMap.data}
        />
      )}
    </section>
  )
}
