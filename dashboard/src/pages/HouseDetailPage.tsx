import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleDropMap, PuzzleHouseDetail, PuzzleQuality, PuzzleTesterImpact, TrafficScope } from '../api/houseTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { Freshness } from '../components/Freshness'
import { QualityBanner } from '../components/QualityBanner'
import { TesterImpactPanel } from '../components/TesterImpactPanel'
import { ScopeCaveat, TrafficScopeSelect } from '../components/TrafficScopeSelect'
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
  build: string
  selected: number | null
  showDrops: boolean
  scope: TrafficScope
}

// Drops are fetched only once asked for, and re-fetched scoped to the
// selected detail — the whole-house cloud is for spotting a pattern, the
// per-detail one for reading it.
function useHouseView({ project, city, house, from, to, build, selected, showDrops, scope }: ViewArgs) {
  const base = `/api/v1/analytics/gameplay/houses/${city}/${house}`
  const fetchHouse = useCallback(() => apiGet<PuzzleHouseDetail>(base, { project, from, to, build: build || undefined, scope }), [base, project, from, to, build, scope])
  const detail = useApiData(fetchHouse, `puzzle-house:${project}:${city}:${house}:${from}:${to}:${build}:${scope}`)
  const fetchDrops = useCallback(
    () =>
      showDrops
        ? apiGet<PuzzleDropMap>(`${base}/drops`, { project, from, to, build: build || undefined, scope, ...(selected != null ? { block_id: String(selected) } : {}) })
        : Promise.resolve(null),
    [base, showDrops, project, from, to, build, selected, scope],
  )
  const dropMap = useApiData<PuzzleDropMap | null>(fetchDrops, `puzzle-drops:${project}:${city}:${house}:${from}:${to}:${build}:${selected ?? 'all'}:${showDrops}:${scope}`)
  return { detail, dropMap }
}

export function HouseDetailPage() {
  const { currentProject } = useAuth()
  const { city, house } = useParams()
  const range = useDateRange()
  const { from, to } = range.params
  const view = useHouseControls()
  const [scope, setScope] = useState<TrafficScope>('natural_only')
  const [build, setBuild] = useState('')
  const { detail, dropMap } = useHouseView({ project: currentProject, city, house, from, to, build, selected: view.selected, showDrops: view.showDrops, scope })
  const fetchQuality = useCallback(() => apiGet<PuzzleQuality>('/api/v1/analytics/gameplay/quality', { project: currentProject, from, to, build: build || undefined }), [currentProject, from, to, build])
  const quality = useApiData(fetchQuality, `puzzle-quality:${currentProject}:${from}:${to}:${build}`)
  const fetchTesterImpact = useCallback(() => apiGet<PuzzleTesterImpact>('/api/v1/analytics/gameplay/tester-impact', { project: currentProject, from, to }), [currentProject, from, to])
  const testerImpact = useApiData(fetchTesterImpact, `puzzle-tester-impact:${currentProject}:${from}:${to}`)

  if (!currentProject) return <p>Select a project to inspect a house.</p>
  return (
    <section aria-labelledby="house-heading">
      <p><Link to="/houses">← All houses</Link></p>
      <h1 id="house-heading">{detail.data?.display_label || `House ${house}`}</h1>
      <DateRangeFields range={range} />
      <TrafficScopeSelect scope={scope} onChange={setScope} />
      <Freshness loading={detail.loading || quality.loading} error={detail.error || quality.error} stale={detail.stale || quality.stale} updatedAt={detail.updatedAt} />
      <QualityBanner quality={quality.data} error={quality.error} build={build} onBuildChange={setBuild} />
      <ScopeCaveat scope={scope} />
      <TesterImpactPanel impact={testerImpact.data} />
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
