import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { PuzzleDeviceListResult, PuzzleMemoryTimeline } from '../api/deviceTypes'
import { useAuth } from '../auth/useAuth'
import { DateRangeFields } from '../components/DateRangeFields'
import { DeviceSummaryTable } from '../components/DeviceSummaryTable'
import { Freshness } from '../components/Freshness'
import { MemoryCohortsCard } from '../components/MemoryCohortsCard'
import { MemoryTimelineChart } from '../components/MemoryTimelineChart'
import { shortId } from '../components/Entity'
import { useApiData } from '../hooks/useApiData'
import { useDateRange } from '../hooks/useDateRange'

function DeviceFilterBar({
  range,
  buildNumber,
  setBuildNumber,
}: {
  range: ReturnType<typeof useDateRange>
  buildNumber: string
  setBuildNumber: (val: string) => void
}) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1rem', alignItems: 'flex-end', marginBottom: '1rem' }}>
      <DateRangeFields range={range} />
      <div className="field">
        <label htmlFor="build-filter">Build filter (optional)</label>
        <input
          id="build-filter"
          type="text"
          placeholder="e.g. 9dccf3d"
          value={buildNumber}
          onChange={(e) => setBuildNumber(e.target.value)}
        />
      </div>
    </div>
  )
}

function DeviceTimelineSection({ project, installId }: { project: string; installId: string }) {
  const fetchTimeline = useCallback(() => {
    if (!installId) return Promise.resolve(null)
    return apiGet<PuzzleMemoryTimeline>(
      `/api/v1/analytics/gameplay/devices/${encodeURIComponent(installId)}/memory-timeline`,
      { project }
    )
  }, [project, installId])

  const timelineData = useApiData<PuzzleMemoryTimeline | null>(
    fetchTimeline,
    `memory-timeline:${project}:${installId}`
  )

  return (
    <section className="card" aria-labelledby="timeline-heading" style={{ marginTop: '2rem' }}>
      <h2 id="timeline-heading">Memory Progression: {shortId(installId)}</h2>
      <Freshness
        loading={timelineData.loading}
        error={timelineData.error}
        stale={timelineData.stale}
        updatedAt={timelineData.updatedAt}
        loadingLabel="Loading memory timeline…"
      />
      {timelineData.data && <MemoryTimelineChart timeline={timelineData.data} />}
    </section>
  )
}

function DeviceProfilesSection({
  data,
  selectedInstall,
  onSelectInstall,
}: {
  data: PuzzleDeviceListResult
  selectedInstall: string
  onSelectInstall: (id: string) => void
}) {
  return (
    <>
      <MemoryCohortsCard cohorts={data.cohorts} />
      <section className="card" aria-labelledby="devices-table-heading" style={{ marginTop: '1.5rem' }}>
        <h2 id="devices-table-heading">Anonymous Device Profiles</h2>
        <p style={{ fontSize: '0.875rem', color: 'var(--text-muted)', marginBottom: '0.75rem' }}>
          Select any device to inspect its progression of allocated, reserved, and Mono memory over active play time.
        </p>
        <DeviceSummaryTable
          devices={data.devices}
          selectedInstallID={selectedInstall}
          onSelectDevice={onSelectInstall}
        />
      </section>
    </>
  )
}

export function DeviceDiagnosticsPage() {
  const { currentProject } = useAuth()
  const range = useDateRange()
  const { from, to } = range.params
  const [buildNumber, setBuildNumber] = useState('')
  const [selectedInstall, setSelectedInstall] = useState('')

  const fetchDevices = useCallback(() => {
    const params: Record<string, string> = { project: currentProject, from, to }
    if (buildNumber.trim()) {
      params.build_number = buildNumber.trim()
    }
    return apiGet<PuzzleDeviceListResult>('/api/v1/analytics/gameplay/devices', params)
  }, [currentProject, from, to, buildNumber])

  const devicesData = useApiData(fetchDevices, `devices:${currentProject}:${from}:${to}:${buildNumber}`)

  if (!currentProject) return <p>Select a project to view device diagnostics.</p>

  return (
    <section aria-labelledby="device-diagnostics-heading">
      <h1 id="device-diagnostics-heading">Device & Memory Diagnostics</h1>
      <p style={{ maxWidth: '800px', lineHeight: 1.5, color: 'var(--text-muted)' }}>
        Anonymous hardware capacity and memory telemetry. Multiple scene reloads are deduplicated to the latest profile per install.
        Memory samples are recorded only after each cumulative 10 minutes of active foreground play.
      </p>

      <DeviceFilterBar range={range} buildNumber={buildNumber} setBuildNumber={setBuildNumber} />

      <Freshness
        loading={devicesData.loading}
        error={devicesData.error}
        stale={devicesData.stale}
        updatedAt={devicesData.updatedAt}
        loadingLabel="Loading device diagnostics…"
      />

      {devicesData.data && (
        <DeviceProfilesSection
          data={devicesData.data}
          selectedInstall={selectedInstall}
          onSelectInstall={(id) => setSelectedInstall(id)}
        />
      )}

      {selectedInstall && <DeviceTimelineSection project={currentProject} installId={selectedInstall} />}
    </section>
  )
}
