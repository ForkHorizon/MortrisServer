import { useCallback, useState } from 'react'
import { apiGet } from '../api/client'
import type { TimelineResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { DataTable } from '../components/DataTable'

export function InstallationTimelinePage() {
  const { currentProject } = useAuth()
  const [inputId, setInputId] = useState('')
  const [installId, setInstallId] = useState('')
  const fetchTimeline = useCallback(
    () =>
      installId && currentProject
        ? apiGet<TimelineResult>(`/api/v1/analytics/installations/${encodeURIComponent(installId)}`, {
            project: currentProject,
          })
        : Promise.resolve(null),
    [currentProject, installId],
  )

  const { data, error, loading } = useApiData<TimelineResult | null>(fetchTimeline)

  if (!currentProject) return <p>Select a project to look up an installation.</p>

  return (
    <section aria-labelledby="timeline-heading">
      <h1 id="timeline-heading">Installation timeline</h1>
      <p>Admin-only: full product and system event history for one anonymous installation ID.</p>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          setInstallId(inputId.trim())
        }}
      >
        <div className="field">
          <label htmlFor="install-id">Installation ID</label>
          <input
            id="install-id"
            value={inputId}
            onChange={(e) => setInputId(e.target.value)}
            placeholder="09ffb634-1792-40cd-bd9e-0a89938ff411"
          />
        </div>
        <button type="submit">Look up</button>
      </form>

      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <>
          <dl>
            <dt>Registered</dt>
            <dd>{new Date(data.registered_at).toLocaleString()}</dd>
            <dt>Activated</dt>
            <dd>{data.activated_at ? new Date(data.activated_at).toLocaleString() : 'Never'}</dd>
          </dl>
          {data.truncated && <p role="alert">Showing the most recent 500 events only.</p>}
          <DataTable
            caption={`Events for ${data.install_id}`}
            columns={[
              { key: 'effective_at', label: 'When', render: (r) => new Date(r.effective_at).toLocaleString() },
              { key: 'name', label: 'Event' },
              { key: 'event_kind', label: 'Kind' },
              { key: 'time_quality', label: 'Clock quality' },
              { key: 'properties', label: 'Properties', render: (r) => JSON.stringify(r.properties) },
            ]}
            rows={data.events}
            getRowKey={(r) => r.event_id}
          />
        </>
      )}
    </section>
  )
}
