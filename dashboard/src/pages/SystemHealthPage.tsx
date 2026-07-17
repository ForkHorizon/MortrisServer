import { useCallback } from 'react'
import { apiGet } from '../api/client'
import type { SystemHealth } from '../api/types'
import { useApiData } from '../hooks/useApiData'
import { StatGrid, StatTile } from '../components/StatTile'
import { DataTable } from '../components/DataTable'

const DISK_STATE_LABEL: Record<SystemHealth['disk_state'], string> = {
  normal: 'Normal',
  warning: 'Warning',
  high: 'High',
  critical: 'Critical',
  rejecting: 'Rejecting (ingestion is being refused)',
}

export function SystemHealthPage() {
  const fetchSystemHealth = useCallback(() => apiGet<SystemHealth>('/api/v1/system'), [])
  const { data, error, loading } = useApiData<SystemHealth>(fetchSystemHealth)

  return (
    <section aria-labelledby="system-heading">
      <h1 id="system-heading">System health</h1>
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <>
          {data.disk_state !== 'normal' && (
            <p role="alert">Disk state: {DISK_STATE_LABEL[data.disk_state]}</p>
          )}
          <StatGrid>
            <StatTile label="Version" value={data.version} />
            <StatTile label="DB latency" value={`${data.db_latency_ms.toFixed(2)} ms`} />
            <StatTile label="Disk state" value={DISK_STATE_LABEL[data.disk_state]} />
            <StatTile label="Ingestion accepted (last hour)" value={data.ingestion_accepted_last_hour} />
            <StatTile label="Ingestion rejected (last hour)" value={data.ingestion_rejected_last_hour} />
            <StatTile label="Enabled policy rules" value={data.enabled_policy_rules} />
          </StatGrid>

          <h2>Connection pools</h2>
          <DataTable
            caption="Writer and reader pool state"
            columns={[
              { key: 'pool', label: 'Pool' },
              { key: 'acquired_conns', label: 'Acquired' },
              { key: 'idle_conns', label: 'Idle' },
              { key: 'total_conns', label: 'Total' },
              { key: 'max_conns', label: 'Max' },
            ]}
            rows={[
              { pool: 'Writer', ...data.writer_pool },
              { pool: 'Reader', ...data.reader_pool },
            ]}
            getRowKey={(r) => r.pool}
          />

          <h2>Maintenance</h2>
          <DataTable
            caption="Most recent maintenance run per kind"
            columns={[
              { key: 'kind', label: 'Kind' },
              { key: 'started_at', label: 'Started', render: (r) => new Date(r.started_at).toLocaleString() },
              { key: 'rows_affected', label: 'Rows affected' },
              { key: 'error', label: 'Error', render: (r) => r.error || '—' },
            ]}
            rows={data.last_maintenance_runs}
            getRowKey={(r) => r.kind}
          />
        </>
      )}
    </section>
  )
}
