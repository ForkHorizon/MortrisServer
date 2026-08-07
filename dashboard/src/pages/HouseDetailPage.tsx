import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { PuzzleDropMap, PuzzleHouseDetail } from '../api/houseTypes'
import type { PuzzleQuality, PuzzleTesterImpact, PuzzleWaveFunnel, TrafficScope } from '../api/puzzleQualityTypes'
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
  selected: number | null
  showDrops: boolean
  scope: TrafficScope
}

// Drops are fetched only once asked for, and re-fetched scoped to the
// selected detail — the whole-house cloud is for spotting a pattern, the
// per-detail one for reading it.
function useHouseView({ project, city, house, from, to, selected, showDrops, scope }: ViewArgs) {
  const base = `/api/v1/analytics/gameplay/houses/${city}/${house}`
  const fetchHouse = useCallback(() => apiGet<PuzzleHouseDetail>(base, { project, from, to, scope }), [base, project, from, to, scope])
  const detail = useApiData(fetchHouse, `puzzle-house:${project}:${city}:${house}:${from}:${to}:${scope}`)
  const fetchDrops = useCallback(
    () =>
      showDrops
        ? apiGet<PuzzleDropMap>(`${base}/drops`, { project, from, to, scope, ...(selected != null ? { block_id: String(selected) } : {}) })
        : Promise.resolve(null),
    [base, showDrops, project, from, to, selected, scope],
  )
  const dropMap = useApiData<PuzzleDropMap | null>(fetchDrops, `puzzle-drops:${project}:${city}:${house}:${from}:${to}:${selected ?? 'all'}:${showDrops}:${scope}`)
  return { detail, dropMap }
}

export function HouseDetailPage() {
  const { currentProject } = useAuth()
  const { city, house } = useParams()
  const range = useDateRange()
  const { from, to } = range.params
  const view = useHouseControls()
  const [scope, setScope] = useState<TrafficScope>('natural_only')
  const { detail, dropMap } = useHouseView({ project: currentProject, city, house, from, to, selected: view.selected, showDrops: view.showDrops, scope })
  const [build, setBuild] = useState('')
  const fetchQuality = useCallback(() => apiGet<PuzzleQuality>('/api/v1/analytics/gameplay/quality', { project: currentProject, from, to, build: build || undefined }), [currentProject, from, to, build])
  const quality = useApiData(fetchQuality, `puzzle-quality:${currentProject}:${from}:${to}:${build}`)
  const fetchTesterImpact = useCallback(() => apiGet<PuzzleTesterImpact>('/api/v1/analytics/gameplay/tester-impact', { project: currentProject, from, to }), [currentProject, from, to])
  const testerImpact = useApiData(fetchTesterImpact, `puzzle-tester-impact:${currentProject}:${from}:${to}`)
  const fetchWaveFunnel = useCallback(() => apiGet<PuzzleWaveFunnel>(`/api/v1/analytics/gameplay/houses/${city}/${house}/funnel`, { project: currentProject, from, to, scope }), [currentProject, city, house, from, to, scope])
  const waveFunnel = useApiData(fetchWaveFunnel, `puzzle-wave-funnel:${currentProject}:${city}:${house}:${from}:${to}:${scope}`)

  if (!currentProject) return <p>Select a project to inspect a house.</p>
  return (
    <section aria-labelledby="house-heading">
      <p><Link to="/houses">← All houses</Link></p>
      <h1 id="house-heading">{detail.data?.display_label || `House ${house}`}</h1>
      <DateRangeFields range={range} />
      <TrafficScopeSelect scope={scope} onChange={setScope} />
      <Freshness loading={detail.loading} error={detail.error} stale={detail.stale} updatedAt={detail.updatedAt} />
      <QualityBanner quality={quality.data} build={build} onBuildChange={setBuild} />
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
          waveFunnel={waveFunnel.data}
        />
      )}
    </section>
  )
}
