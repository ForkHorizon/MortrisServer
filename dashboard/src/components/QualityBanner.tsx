import { useState } from 'react'
import type { PuzzleQuality, PuzzleQualityStatus } from '../api/houseTypes'

// Stage 2's "is this range safe to trust" card (docs/puzzle-analytics-remaining-plan.md
// section 5). Sits above every house ranking so a designer sees the
// caveat before the verdict, not after.
const STATUS_LABEL: Record<PuzzleQualityStatus, string> = {
  grey: 'No data yet',
  green: 'No known integrity problems',
  amber: 'Usable, with caveats',
  red: 'Do not trust this range yet',
}

function StatRow({ label, value }: { label: string; value: number }) {
  return (
    <li>
      <span className="muted">{label}</span> <strong>{value.toLocaleString()}</strong>
    </li>
  )
}

// Split out of QualityBanner to stay under the repo's 50-line function
// gate — the expanded stat list/rejections table isn't meant to be read
// as an independent component.
function QualityDetails({ quality }: { quality: PuzzleQuality }) {
  return (
    <div className="quality-details">
      <ul className="quality-stat-list">
        <StatRow label="Received" value={quality.received_events} />
        <StatRow label="Accepted" value={quality.accepted_events} />
        <StatRow label="Duplicates" value={quality.duplicate_events} />
        <StatRow label="Rejected" value={quality.rejected_events} />
        <StatRow label="Schema-v2 coverage (%)" value={Math.round(quality.schema_v2_share * 100)} />
        <StatRow label="Invalid schema versions" value={quality.invalid_schema_version_events} />
        <StatRow label="Installs" value={quality.distinct_installs} />
        <StatRow label="House runs" value={quality.distinct_house_runs} />
        <StatRow label="Wave attempts" value={quality.distinct_wave_attempts} />
        <StatRow label="Interactions" value={quality.distinct_interactions} />
        <StatRow label="SDK sequence gaps" value={quality.sequence_gaps} />
        <StatRow label="SDK sequence duplicates" value={quality.sequence_duplicates} />
        <StatRow label="House index gaps" value={quality.house_event_index_gaps} />
        <StatRow label="Attempt index gaps" value={quality.attempt_event_index_gaps} />
        <StatRow label="Invalid event indexes" value={quality.invalid_event_indexes} />
        <StatRow label="Unknown revisions" value={quality.unknown_revision_events} />
        <StatRow label="Invalid coordinate spaces" value={quality.invalid_coordinate_space_events} />
        <StatRow label="Orphan interactions" value={quality.orphan_interactions} />
        <StatRow label="Checkpoint mismatches" value={quality.checkpoint_mismatches} />
        <StatRow label="Recovered attempts" value={quality.recovered_attempts} />
        <StatRow label="House runs still open (recent)" value={quality.open_house_runs_recent} />
        <StatRow label="House runs still open (stale/lost)" value={quality.open_house_runs_stale} />
        <StatRow label="Attempts still open (recent)" value={quality.open_attempts_recent} />
        <StatRow label="Attempts still open (stale/lost)" value={quality.open_attempts_stale} />
        <StatRow label="Developer commands" value={quality.unpaired_developer_commands} />
        <StatRow label="Developer mutations without a start" value={quality.unpaired_developer_mutations} />
        <StatRow label="Delivery delay p50 (ms)" value={Math.round(quality.delivery_delay_ms.p50_ms)} />
        <StatRow label="Delivery delay p99 (ms)" value={Math.round(quality.delivery_delay_ms.p99_ms)} />
        <StatRow label="Delivery delay max (ms)" value={Math.round(quality.delivery_delay_ms.max_ms)} />
      </ul>
      {quality.rejections.length > 0 && (
        <table className="quality-rejections">
          <caption>Rejections by event and reason</caption>
          <thead>
            <tr>
              <th scope="col">Event</th>
              <th scope="col">Reason</th>
              <th scope="col">Count</th>
            </tr>
          </thead>
          <tbody>
            {quality.rejections.map((r) => (
              <tr key={`${r.name}:${r.code}`}>
                <td>{r.name}</td>
                <td>{r.code}</td>
                <td>{r.count.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// Per-attempt/run drill-down for each reason (the plan doc's "must link
// to the affected attempt/run") needs the backend to carry example IDs
// per bucket, which Stage 2's aggregate response doesn't yet return.
// ponytail: reasons are plain-language only for now; add example-ID
// linking once a bucket needs it enough to justify widening every query.
export function QualityBanner({
  quality,
  error,
  build,
  onBuildChange,
}: {
  quality: PuzzleQuality | null
  error?: string | null
  build?: string
  onBuildChange?: (build: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  if (!quality) {
    return error ? <section className="quality-banner quality-banner--red" aria-label="Data quality"><strong>Data quality unavailable</strong><p className="quality-reasons">Rankings are visible, but their integrity could not be checked. {error}</p></section> : null
  }
  const { status } = quality
  return (
    <section className={`quality-banner quality-banner--${status}`} aria-label="Data quality">
      <div className="quality-banner-head">
        <strong>{STATUS_LABEL[status]}</strong>
        {quality.builds.length > 1 && (
          <label className="quality-build-select">
            Build
            <select value={build ?? ''} onChange={(e) => onBuildChange?.(e.target.value)}>
              <option value="">All builds ({quality.builds.length} in range)</option>
              {quality.builds.map((b) => (
                <option key={`${b.app_version}:${b.build_number}`} value={b.build_number}>
                  {b.app_version} ({b.build_number}) · {b.events.toLocaleString()} events
                </option>
              ))}
            </select>
          </label>
        )}
        <button type="button" className="link-button" onClick={() => setExpanded((v) => !v)}>
          {expanded ? 'Hide details' : 'Show details'}
        </button>
      </div>
      {quality.status_reasons.length > 0 && (
        <ul className="quality-reasons">
          {quality.status_reasons.map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
      )}
      {expanded && <QualityDetails quality={quality} />}
    </section>
  )
}
