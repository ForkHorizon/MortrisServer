import type { MemoryCohort } from '../api/deviceTypes'
import { DataTable } from './DataTable'

interface MemoryCohortsCardProps {
  cohorts: MemoryCohort[]
}

const percent = (ratio: number) => `${Math.round(ratio * 100)}%`

function cohortIdentityColumns() {
  return [
    {
      key: 'label',
      label: 'Memory Tier',
      render: (r: MemoryCohort) => (
        <div>
          <strong>{r.label}</strong>
          {!r.sample_count_sufficient && (
            <span
              className="badge badge-neutral"
              style={{ marginLeft: '0.5rem', fontSize: '0.75rem' }}
              title="Fewer than 5 natural runs observed"
            >
              Insufficient sample (&lt;5 runs)
            </span>
          )}
        </div>
      ),
    },
    {
      key: 'device_count',
      label: 'Unique Installs',
      render: (r: MemoryCohort) => r.device_count,
    },
  ]
}

function cohortRunColumns() {
  return [
    {
      key: 'natural_runs',
      label: 'Natural Runs',
      render: (r: MemoryCohort) => `${r.completed_runs_count} / ${r.natural_runs_count}`,
    },
    {
      key: 'completion_rate',
      label: 'Completion Rate',
      render: (r: MemoryCohort) => {
        if (r.natural_runs_count === 0) return '—'
        return (
          <span>
            {percent(r.completion_rate)}
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginLeft: '0.25rem' }}>
              ({r.completed_runs_count}/{r.natural_runs_count})
            </span>
          </span>
        )
      },
    },
  ]
}

function cohortFrictionColumns() {
  return [
    {
      key: 'placements_and_falls',
      label: 'Falls / Placements',
      render: (r: MemoryCohort) => `${r.falls_count} / ${r.placements_count}`,
    },
    {
      key: 'fall_rate',
      label: 'Observed Fall Rate',
      render: (r: MemoryCohort) => {
        if (r.placements_count === 0) return '—'
        return (
          <span>
            {percent(r.fall_rate)}
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginLeft: '0.25rem' }}>
              ({r.falls_count}/{r.placements_count})
            </span>
          </span>
        )
      },
    },
    {
      key: 'evidence_note',
      label: 'Sample Assessment',
      render: (r: MemoryCohort) => (
        <span style={{ fontSize: '0.85rem', color: r.sample_count_sufficient ? 'inherit' : 'var(--text-muted)' }}>
          {r.evidence_note}
        </span>
      ),
    },
  ]
}

function cohortColumns() {
  return [...cohortIdentityColumns(), ...cohortRunColumns(), ...cohortFrictionColumns()]
}

export function MemoryCohortsCard({ cohorts }: MemoryCohortsCardProps) {
  return (
    <section className="card" aria-labelledby="memory-cohorts-heading" style={{ marginBottom: '1.5rem' }}>
      <h2 id="memory-cohorts-heading">Observed Correlational Rates Across Memory Tiers</h2>
      <p style={{ color: 'var(--text-muted)', fontSize: '0.9rem', marginBottom: '1rem' }}>
        Correlational performance distribution across device RAM bands.
        <strong> Note:</strong> Correlation does not imply causation. Observed variations across tiers may reflect differences in player cohorts, operating systems, or device capabilities.
      </p>

      <DataTable
        caption="Hardware Memory Cohort Comparison (Correlational)"
        rows={cohorts}
        getRowKey={(r) => r.tier}
        columns={cohortColumns()}
      />
    </section>
  )
}
