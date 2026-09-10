import type { PuzzleHouseBlock, PuzzleRetryLadder, PuzzleTimeToPlaceDistribution } from '../api/houseTypes'
import { outcomeWords, paintFor, ruleSentence, supportColor, supportGroupLabel } from '../components/houseColors'
import { type HouseMetric, metricReading, metricTitle } from '../components/houseMetrics'
import { StatGrid, StatTile } from '../components/StatTile'
import { DetailDiagnosisBanner } from './houseDetailText'

export function BlockPanel({
  block,
  metric,
  showSupport = false,
  onSelectAttempt,
}: {
  block: PuzzleHouseBlock
  metric: HouseMetric
  showSupport?: boolean
  onSelectAttempt?: (id: string) => void
}) {
  const reading = metricReading(block, metric)
  const paint = paintFor(reading.ratio, reading.reliable, reading.sample)
  const reasons = Object.entries(block.falls_by_reason ?? {})
    .filter(([, count]) => count > 0)
    .sort((a, b) => b[1] - a[1])

  return (
    <div className="block-panel">
      <h2>Detail {block.block_id}</h2>
      <p className="muted">
        wave {block.wave_index + 1}
        {block.is_ground ? ' · sits on the ground' : ''} · {block.visual_key}
      </p>
      <StatGrid>
        <StatTile label="Drops" value={block.placements} />
        <StatTile label={metricTitle(metric)} value={reading.label} />
        <StatTile label="Metric rating" value={paint.label} />
      </StatGrid>

      <TimeDistributionSubpanel ttp={block.time_to_place} />
      <RetryLadderSubpanel ladder={block.retry_ladder} onSelectAttempt={onSelectAttempt} />
      <WhyItFellSubpanel reasons={reasons} />

      <h3>Placement rule</h3>
      <p>{ruleSentence(block.block_id, block.required_groups)}</p>
      {showSupport && block.required_groups.length > 0 && <SupportKey groups={block.required_groups} />}

      <DetailDiagnosisBanner block={block} />
    </div>
  )
}

function WhyItFellSubpanel({ reasons }: { reasons: Array<[string, number]> }) {
  if (reasons.length === 0) return null
  return (
    <>
      <h3>Why it fell</h3>
      <ul>
        {reasons.map(([outcome, count]) => (
          <li key={outcome}>
            {count}× {outcomeWords(outcome)}
          </li>
        ))}
      </ul>
    </>
  )
}

function TimeDistributionSubpanel({ ttp }: { ttp: PuzzleTimeToPlaceDistribution | undefined }) {
  if (!ttp) return null
  if (ttp.sample_count === 0 && ttp.incomplete_interactions === 0) {
    return (
      <>
        <h3>Time to place distribution</h3>
        <p className="muted">No timing samples recorded for this detail.</p>
      </>
    )
  }
  return (
    <>
      <h3>Time to place distribution</h3>
      <StatGrid>
        <StatTile label="Completed samples" value={ttp.sample_count} />
        <StatTile label="Median active time" value={ttp.sample_count > 0 ? `${(ttp.median_ms / 1000).toFixed(1)}s` : '—'} />
        <StatTile
          label="p75 / p90"
          value={ttp.reliable ? `${(ttp.p75_ms / 1000).toFixed(1)}s / ${(ttp.p90_ms / 1000).toFixed(1)}s` : 'Not enough plays (<5)'}
        />
      </StatGrid>
      {!ttp.reliable && ttp.sample_count > 0 && (
        <p className="muted">Only {ttp.sample_count} completed placement sample(s) — minimum 5 required for confident p75/p90 percentiles.</p>
      )}
      {ttp.incomplete_interactions > 0 && (
        <p className="muted">Incomplete/abandoned interactions: {ttp.incomplete_interactions} (excluded from placement timing).</p>
      )}
    </>
  )
}

function RetryLadderSubpanel({
  ladder,
  onSelectAttempt,
}: {
  ladder: PuzzleRetryLadder | undefined
  onSelectAttempt?: (id: string) => void
}) {
  if (!ladder) return null
  if (ladder.sample_count === 0) {
    return (
      <>
        <h3>Retry ladder</h3>
        <p className="muted">No retry chains recorded for this detail.</p>
      </>
    )
  }
  return (
    <>
      <h3>Retry ladder</h3>
      <StatGrid>
        <StatTile label="Total chains" value={ladder.sample_count} />
        <StatTile label="Median tries" value={ladder.sample_count > 0 && ladder.median_tries > 0 ? ladder.median_tries : '—'} />
        <StatTile
          label="Retry rating"
          value={
            !ladder.reliable
              ? 'Not enough plays (<5)'
              : (ladder.never_succeeded / ladder.sample_count) >= 0.25 || ladder.median_tries >= 3
                ? 'High friction'
                : ladder.median_tries === 1
                  ? 'Smooth (1st try)'
                  : 'Moderate retries'
          }
        />
      </StatGrid>
      <RetryBreakdownList ladder={ladder} />
      {ladder.reliable && (ladder.p75_tries > 0 || ladder.p90_tries > 0) && (
        <p className="muted">Percentiles: p75 = {ladder.p75_tries} tries · p90 = {ladder.p90_tries} tries</p>
      )}
      {ladder.example_attempt_ids && ladder.example_attempt_ids.length > 0 && (
        <ExampleReplaysBlock attemptIds={ladder.example_attempt_ids} onSelectAttempt={onSelectAttempt} />
      )}
    </>
  )
}

function RetryBreakdownList({ ladder }: { ladder: PuzzleRetryLadder }) {
  return (
    <ul className="retry-breakdown">
      <li>1st try: <strong>{ladder.success_1st_try}</strong> ({formatPercent(ladder.success_1st_try, ladder.sample_count)})</li>
      <li>2nd try: <strong>{ladder.success_2nd_try}</strong> ({formatPercent(ladder.success_2nd_try, ladder.sample_count)})</li>
      <li>3rd try: <strong>{ladder.success_3rd_try}</strong> ({formatPercent(ladder.success_3rd_try, ladder.sample_count)})</li>
      <li>4th+ try: <strong>{ladder.success_4th_plus_try}</strong> ({formatPercent(ladder.success_4th_plus_try, ladder.sample_count)})</li>
      <li>Never placed: <strong>{ladder.never_succeeded}</strong> ({formatPercent(ladder.never_succeeded, ladder.sample_count)})</li>
    </ul>
  )
}

function ExampleReplaysBlock({
  attemptIds,
  onSelectAttempt,
}: {
  attemptIds: string[]
  onSelectAttempt?: (id: string) => void
}) {
  return (
    <div className="example-replays-block">
      <strong>Example replays:</strong>
      <div className="ladder-replay-links">
        {attemptIds.map((attId) => (
          <button
            key={attId}
            type="button"
            className="link-button example-replay-btn"
            title={`Watch replay for attempt ${attId}`}
            aria-label={`Watch replay for attempt ${attId.slice(0, 8)}`}
            onClick={() => onSelectAttempt?.(attId)}
          >
            #{attId.slice(0, 8)}
          </button>
        ))}
      </div>
    </div>
  )
}

function formatPercent(count: number, total: number): string {
  if (total <= 0) return '0%'
  return `${Math.round((count / total) * 100)}%`
}

export function SupportKey({ groups }: { groups: number[][] }) {
  return (
    <ul className="support-key">
      {groups.map((group, index) => (
        <li key={index}>
          <span className="house-card-swatch small" style={{ background: supportColor(index) }} aria-hidden="true" />
          <strong>{supportGroupLabel(index)}</strong>{' '}
          {group.length === 1 ? `detail ${group[0]}` : `details ${group.join(' and ')} together`}
        </li>
      ))}
    </ul>
  )
}
