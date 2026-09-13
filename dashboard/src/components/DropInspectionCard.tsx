import type { PuzzleDrop } from '../api/houseTypes'
import {
  type DropCluster,
  formatDistanceMilli,
  formatOutcomeLabel,
  outcomeColor,
} from './dropClustering'

type DropInspectionCardProps = {
  drop?: PuzzleDrop | null
  cluster?: DropCluster | null
  onClose: () => void
  onSelectAttempt?: (attemptId: string) => void
}

export function DropInspectionCard(props: DropInspectionCardProps) {
  if (props.cluster) {
    return <ClusterInspectionCard cluster={props.cluster} onClose={props.onClose} onSelectAttempt={props.onSelectAttempt} />
  }
  if (props.drop) {
    return <SingleDropInspectionCard drop={props.drop} onClose={props.onClose} onSelectAttempt={props.onSelectAttempt} />
  }
  return null
}

function ClusterInspectionCard({
  cluster,
  onClose,
  onSelectAttempt,
}: {
  cluster: DropCluster
  onClose: () => void
  onSelectAttempt?: (attemptId: string) => void
}) {
  return (
    <div className="drop-inspection-card" role="region" aria-label="Cluster details" onClick={(e) => e.stopPropagation()}>
      <div className="drop-card-header">
        <strong>Cluster ({cluster.drops.length} drops)</strong>
        <button type="button" className="drop-card-close" onClick={onClose} aria-label="Close callout">✕</button>
      </div>
      <div className="drop-card-body">
        <p>
          <span className="drop-outcome-badge" style={{ backgroundColor: outcomeColor(cluster.dominantOutcome) }} />
          Dominant: <strong>{formatOutcomeLabel(cluster.dominantOutcome)}</strong>
        </p>
        <div className="drop-breakdown-list">
          {Object.entries(cluster.outcomeCounts).map(([outcome, count]) => (
            <span key={outcome} className="drop-breakdown-item">
              {count} {formatOutcomeLabel(outcome).toLowerCase()}
            </span>
          ))}
        </div>
        <p className="muted">
          Centroid: ({cluster.centroidX}, {cluster.centroidY})
          {cluster.nearestTargetId != null && cluster.nearestTargetId >= 0 && ` · Near slot ${cluster.nearestTargetId}`}
        </p>
        {cluster.exampleAttemptIds.length > 0 && onSelectAttempt && (
          <div className="drop-replay-actions">
            <span className="drop-replay-label">Example replays:</span>
            <div className="drop-replay-buttons">
              {cluster.exampleAttemptIds.slice(0, 3).map((attId, idx) => (
                <button key={attId} type="button" className="drop-replay-btn" onClick={() => onSelectAttempt(attId)}>
                  Replay #{idx + 1}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function SingleDropInspectionCard({
  drop,
  onClose,
  onSelectAttempt,
}: {
  drop: PuzzleDrop
  onClose: () => void
  onSelectAttempt?: (attemptId: string) => void
}) {
  const hasNearest = (drop.nearest_compatible_target_id ?? -1) >= 0
  return (
    <div className="drop-inspection-card" role="region" aria-label="Drop details" onClick={(e) => e.stopPropagation()}>
      <div className="drop-card-header">
        <div className="drop-header-outcome">
          <span className="drop-outcome-badge" style={{ backgroundColor: outcomeColor(drop.outcome) }} />
          <strong>{formatOutcomeLabel(drop.outcome)}</strong>
        </div>
        <button type="button" className="drop-card-close" onClick={onClose} aria-label="Close callout">✕</button>
      </div>
      <div className="drop-card-body">
        <p className="drop-coord-row">
          Release: <code>({drop.release_x_milli}, {drop.release_y_milli})</code>
        </p>
        {drop.rule_state && <p className="drop-rule-row">Rule state: <em>{drop.rule_state}</em></p>}
        {hasNearest && (
          <p className="drop-target-row">
            {formatDistanceMilli(drop.nearest_distance_milli)} from slot <strong>{drop.nearest_compatible_target_id}</strong>
          </p>
        )}
        {drop.candidate_target_id != null && drop.candidate_target_id >= 0 && drop.candidate_target_id !== drop.nearest_compatible_target_id && (
          <p className="muted">Snapped to slot {drop.candidate_target_id}</p>
        )}
        {drop.attempt_id && onSelectAttempt && (
          <div className="drop-replay-actions">
            <button type="button" className="drop-replay-btn" onClick={() => onSelectAttempt(drop.attempt_id)}>
              Replay Attempt
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
