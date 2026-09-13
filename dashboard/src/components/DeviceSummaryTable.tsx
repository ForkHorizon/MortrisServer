import type { PuzzleDeviceSummary } from '../api/deviceTypes'
import { DataTable } from './DataTable'
import { shortId } from './Entity'
import { Timestamp } from './Timestamp'

interface DeviceSummaryTableProps {
  devices: PuzzleDeviceSummary[]
  selectedInstallID?: string
  onSelectDevice: (installID: string) => void
}

function formatRAM(mb: number): string {
  if (!mb || mb <= 0) return 'Unknown'
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(1)} GB (${mb} MB)`
  }
  return `${mb} MB`
}

function formatActiveTime(ms: number): string {
  if (!ms || ms <= 0) return '0s'
  const totalSeconds = Math.round(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes === 0) return `${seconds}s`
  return `${minutes}m ${seconds}s`
}

export function MemoryStatusBadge({ device }: { device: PuzzleDeviceSummary }) {
  if (device.memory_sample_status === 'available') {
    return (
      <span className="badge badge-success" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
        <span aria-hidden="true">●</span>
        <span>Recorded ({device.memory_sample_count})</span>
      </span>
    )
  }
  if (device.memory_sample_status === 'not_expected') {
    return (
      <span className="badge badge-neutral" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }} title="Client emits memory samples only after cumulative 10 minutes of active foreground play">
        <span aria-hidden="true">○</span>
        <span>Not expected yet (&lt;10m active play)</span>
      </span>
    )
  }
  return (
    <span className="badge badge-warning" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
      <span aria-hidden="true">▲</span>
      <span>Expected (≥10m active play) but not recorded</span>
    </span>
  )
}

function deviceIdentityColumns(
  selectedInstallID: string | undefined,
  onSelectDevice: (installID: string) => void
) {
  return [
    {
      key: 'install_id',
      label: 'Device (Install ID)',
      render: (r: PuzzleDeviceSummary) => (
        <button
          type="button"
          className={`entity-id ${selectedInstallID === r.install_id ? 'selected' : ''}`}
          title={`Select ${r.install_id} to view memory timeline`}
          onClick={() => onSelectDevice(r.install_id)}
          aria-pressed={selectedInstallID === r.install_id}
        >
          {shortId(r.install_id)}
        </button>
      ),
    },
    {
      key: 'device_class',
      label: 'Device Model',
      render: (r: PuzzleDeviceSummary) => `${r.device_class || 'Unknown'} (${r.platform} ${r.os_version})`,
    },
    {
      key: 'build_number',
      label: 'Build',
      render: (r: PuzzleDeviceSummary) => `${r.app_version} (${r.build_number})`,
    },
    {
      key: 'device_total_memory_mb',
      label: 'Total RAM',
      render: (r: PuzzleDeviceSummary) => formatRAM(r.device_total_memory_mb),
    },
    {
      key: 'graphics_memory_mb',
      label: 'Graphics RAM',
      render: (r: PuzzleDeviceSummary) => r.graphics_memory_mb ? `${r.graphics_memory_mb} MB` : 'Unknown',
    },
  ]
}

function deviceMetricsColumns(
  selectedInstallID: string | undefined,
  onSelectDevice: (installID: string) => void
) {
  return [
    {
      key: 'runs',
      label: 'Natural Runs',
      render: (r: PuzzleDeviceSummary) => `${r.completed_runs_count} / ${r.natural_runs_count} completed`,
    },
    {
      key: 'falls',
      label: 'Falls / Placements',
      render: (r: PuzzleDeviceSummary) => `${r.falls_count} / ${r.placements_count}`,
    },
    {
      key: 'active_play',
      label: 'Active Play',
      render: (r: PuzzleDeviceSummary) => formatActiveTime(r.active_play_ms),
    },
    {
      key: 'memory_status',
      label: 'Memory Telemetry Status',
      render: (r: PuzzleDeviceSummary) => <MemoryStatusBadge device={r} />,
    },
    {
      key: 'last_seen_at',
      label: 'Last Seen',
      render: (r: PuzzleDeviceSummary) => <Timestamp value={r.last_seen_at} mode="relative" />,
    },
    {
      key: 'actions',
      label: 'Timeline',
      render: (r: PuzzleDeviceSummary) => (
        <button
          type="button"
          onClick={() => onSelectDevice(r.install_id)}
          aria-label={`Inspect memory timeline for install ${shortId(r.install_id)}`}
        >
          {selectedInstallID === r.install_id ? 'Viewing' : 'Inspect'}
        </button>
      ),
    },
  ]
}

export function DeviceSummaryTable({ devices, selectedInstallID, onSelectDevice }: DeviceSummaryTableProps) {
  const columns = [
    ...deviceIdentityColumns(selectedInstallID, onSelectDevice),
    ...deviceMetricsColumns(selectedInstallID, onSelectDevice),
  ]

  return (
    <DataTable
      caption="Anonymous Device Summaries (deduplicated by install ID)"
      rows={devices}
      getRowKey={(r) => r.install_id}
      columns={columns}
    />
  )
}
