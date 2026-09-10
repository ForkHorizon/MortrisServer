import type { PuzzleDropMap, PuzzleHouseBlock } from '../api/houseTypes'
import { percent } from '../components/houseColors'
import { diagnoseDetail, toneLabel, type DiagnosisTone } from '../components/detailDiagnosis'

export { detailDiagnosis, detailDiagnosisTone, leadingFallReason } from '../components/detailDiagnosis'
export type { DiagnosisTone }

export function DetailDiagnosisBanner({ block }: { block: PuzzleHouseBlock }) {
  const { sentence, tone } = diagnoseDetail(block)
  const badgeText = toneLabel(tone)

  return (
    <section
      className={`detail-diagnosis-banner detail-diagnosis-banner--${tone}`}
      aria-labelledby={`detail-${block.block_id}-diagnosis-heading`}
      aria-live="polite"
      aria-atomic="true"
    >
      <div className="detail-diagnosis-banner-head">
        <h3
          id={`detail-${block.block_id}-diagnosis-heading`}
          className="detail-diagnosis-header"
        >
          Diagnosis
        </h3>
        <span className={`diagnosis-badge diagnosis-badge--${tone}`}>
          {badgeText}
        </span>
      </div>
      <p className="detail-diagnosis-sentence">{sentence}</p>
    </section>
  )
}

export function houseVerdict(worst: PuzzleHouseBlock | undefined): string {
  return worst
    ? `Detail ${worst.block_id} is the hardest part of this house — ${percent(worst.fall_rate)} of drops on it end in a fall.`
    : 'No detail here has enough plays yet to judge. Grey means too few tries, not a good result.'
}

export function DropNote({ map, scoped }: { map: PuzzleDropMap | null | undefined; scoped: boolean }) {
  if (!map) return <p className="muted">Loading drops…</p>
  const scope = scoped ? 'this detail' : 'every detail in this house'
  if (!map.aligned) {
    const message = map.alignment_issue === 'mixed_coordinate_spaces'
      ? 'This range mixes old world-space drops with corrected house-space drops. Start the range after the corrected build before reading this map.'
      : map.alignment_issue === 'inconsistent'
        ? "Can't place these drops on the house — the recorded positions disagree with each other, so no single alignment fits them. Nothing is shown rather than showing it in the wrong place."
        : 'Not enough successful placements in this range to work out where these drops belong on the house. Widen the date range, or wait for more play.'
    return <p className="muted">{message}</p>
  }
  if (map.drops.length === 0) return <p className="muted">No recorded drops for {scope} in this range.</p>
  return <p className="muted">{map.drops.length} {map.drops.length === 1 ? 'drop' : 'drops'} on {scope}. Each dot is where a player let go; the line runs to the slot they were aiming at.{map.unresolved_drops > 0 && ` ${map.unresolved_drops} could not be matched to a slot and are not shown.`}</p>
}
