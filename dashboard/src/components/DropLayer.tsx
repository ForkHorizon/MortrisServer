import { useMemo } from 'react'
import type { PuzzleDrop } from '../api/houseTypes'
import {
  type DropCluster,
  clusterDrops,
  formatDistanceMilli,
  formatOutcomeLabel,
  outcomeColor,
} from './dropClustering'

type DropLayerProps = {
  drops: PuzzleDrop[]
  scale: number
  activeDrop?: PuzzleDrop | null
  activeCluster?: DropCluster | null
  onSelectDrop?: (drop: PuzzleDrop | null) => void
  onSelectCluster?: (cluster: DropCluster | null) => void
}

export function DropLayer(props: DropLayerProps) {
  const { drops, scale, activeDrop, activeCluster, onSelectDrop, onSelectCluster } = props
  const { clusters, discreteDrops } = useMemo(() => clusterDrops(drops), [drops])

  const visibleDiscrete = useMemo(() => {
    if (!activeCluster) return discreteDrops
    const existing = new Set(discreteDrops)
    for (const d of activeCluster.drops) existing.add(d)
    return Array.from(existing)
  }, [discreteDrops, activeCluster])

  return (
    <g className="drop-map-layer" aria-label="Drop Map Overlay">
      {clusters.map((cluster) => (
        <ClusterMarker
          key={cluster.id}
          cluster={cluster}
          scale={scale}
          color={outcomeColor(cluster.dominantOutcome)}
          isSelected={activeCluster?.id === cluster.id}
          onSelect={() => {
            onSelectDrop?.(null)
            onSelectCluster?.(activeCluster?.id === cluster.id ? null : cluster)
          }}
        />
      ))}
      {visibleDiscrete.map((drop, idx) => (
        <DiscreteDropMarker
          key={`${drop.attempt_id}-${drop.block_id}-${idx}`}
          drop={drop}
          scale={scale}
          isSelected={activeDrop === drop}
          onSelect={() => {
            onSelectCluster?.(null)
            onSelectDrop?.(activeDrop === drop ? null : drop)
          }}
        />
      ))}
    </g>
  )
}

function DropTooltip({ drop }: { drop: PuzzleDrop }) {
  const hasNearest = (drop.nearest_compatible_target_id ?? -1) >= 0
  return (
    <title>
      {formatOutcomeLabel(drop.outcome)}: ({drop.release_x_milli}, {drop.release_y_milli})
      {hasNearest &&
        ` · ${formatDistanceMilli(drop.nearest_distance_milli)} from slot ${drop.nearest_compatible_target_id}`}
    </title>
  )
}

function DiscreteDropMarker({
  drop,
  scale,
  isSelected,
  onSelect,
}: {
  drop: PuzzleDrop
  scale: number
  isSelected: boolean
  onSelect: () => void
}) {
  if (!Number.isFinite(drop.release_x_milli) || !Number.isFinite(drop.release_y_milli)) return null
  const color = outcomeColor(drop.outcome)
  const hasTarget = (drop.target_id ?? -1) >= 0 && Number.isFinite(drop.target_x_milli) && Number.isFinite(drop.target_y_milli)

  return (
    <g
      className={`drop-marker-group ${isSelected ? 'drop-marker--selected' : ''}`}
      onClick={(e) => { e.stopPropagation(); onSelect() }}
      tabIndex={0}
      role="button"
      aria-label={`Drop: ${formatOutcomeLabel(drop.outcome)} at (${drop.release_x_milli}, ${drop.release_y_milli})`}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect() } }}
      style={{ cursor: 'pointer' }}
    >
      {hasTarget && (
        <AimVector
          releaseX={drop.release_x_milli}
          releaseY={-drop.release_y_milli}
          targetX={drop.target_x_milli}
          targetY={-drop.target_y_milli}
          outcome={drop.outcome}
          color={color}
          scale={scale}
          highlighted={isSelected}
        />
      )}
      <DropShape
        outcome={drop.outcome}
        x={drop.release_x_milli}
        y={-drop.release_y_milli}
        scale={scale}
        color={color}
        highlighted={isSelected}
      />
      <DropTooltip drop={drop} />
    </g>
  )
}

function ClusterMarker({
  cluster,
  scale,
  color,
  isSelected,
  onSelect,
}: {
  cluster: DropCluster
  scale: number
  color: string
  isSelected: boolean
  onSelect: () => void
}) {
  const cx = cluster.centroidX
  const cy = -cluster.centroidY
  const badgeR = Math.max(scale * 3.5, 140)

  return (
    <g
      className={`drop-cluster-group ${isSelected ? 'drop-cluster--selected' : ''}`}
      onClick={(e) => { e.stopPropagation(); onSelect() }}
      tabIndex={0}
      role="button"
      aria-label={`Cluster: ${cluster.drops.length} drops, dominant ${formatOutcomeLabel(cluster.dominantOutcome)}`}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect() } }}
      style={{ cursor: 'pointer' }}
    >
      <circle
        cx={cx}
        cy={cy}
        r={cluster.radius}
        fill={color}
        fillOpacity={isSelected ? 0.32 : 0.18}
        stroke={color}
        strokeWidth={scale * (isSelected ? 1.4 : 0.8)}
        strokeDasharray={`${scale * 4} ${scale * 2}`}
      />
      <ClusterBadge cx={cx} cy={cy} r={badgeR} color={color} count={cluster.drops.length} isSelected={isSelected} scale={scale} />
      <title>Cluster: {cluster.drops.length} drops ({formatOutcomeLabel(cluster.dominantOutcome)})</title>
    </g>
  )
}

function ClusterBadge({ cx, cy, r, color, count, isSelected, scale }: {
  cx: number; cy: number; r: number; color: string; count: number; isSelected: boolean; scale: number
}) {
  return (
    <>
      <circle cx={cx} cy={cy} r={r} fill="#0b0d10" fillOpacity={0.88} stroke={isSelected ? '#ffffff' : color} strokeWidth={scale * 0.9} />
      <text x={cx} y={cy} textAnchor="middle" dominantBaseline="central" fill="#ffffff" fontSize={r * 1.1} fontWeight="bold" pointerEvents="none">
        {count}
      </text>
    </>
  )
}

function aimDashArray(outcome: string, scale: number): string | undefined {
  if (outcome === 'fell_missing_support') return `${scale * 3} ${scale * 2}`
  if (outcome === 'fell_no_snap_target') return `${scale * 1.2} ${scale * 2}`
  if (outcome === 'fell_missing_rule') return `${scale * 4} ${scale * 2} ${scale * 1} ${scale * 2}`
  return undefined
}

function AimVector({ releaseX, releaseY, targetX, targetY, outcome, color, scale, highlighted }: {
  releaseX: number; releaseY: number; targetX: number; targetY: number; outcome: string; color: string; scale: number; highlighted: boolean
}) {
  return (
    <line
      x1={releaseX}
      y1={releaseY}
      x2={targetX}
      y2={targetY}
      stroke={color}
      strokeWidth={scale * (highlighted ? 1.2 : 0.6)}
      strokeOpacity={highlighted ? 0.95 : 0.6}
      strokeDasharray={aimDashArray(outcome, scale)}
    />
  )
}

function DropShape({ outcome, x, y, scale, color, highlighted }: {
  outcome: string; x: number; y: number; scale: number; color: string; highlighted: boolean
}) {
  const r = scale * (highlighted ? 2.8 : 2.0)
  return (
    <g>
      {highlighted && <circle cx={x} cy={y} r={r * 1.7} fill="none" stroke="#ffffff" strokeWidth={scale * 0.8} />}
      <DropShapeGeometry outcome={outcome} x={x} y={y} r={r} scale={scale} color={color} />
    </g>
  )
}

function DropShapeGeometry({ outcome, x, y, r, scale, color }: {
  outcome: string; x: number; y: number; r: number; scale: number; color: string
}) {
  if (outcome === 'fell_missing_support') {
    return <rect x={x - r} y={y - r} width={r * 2} height={r * 2} rx={scale * 0.3} fill={color} stroke="#0b0d10" strokeWidth={scale * 0.4} />
  }
  if (outcome === 'fell_no_snap_target') {
    return <polygon points={`${x},${y - r * 1.2} ${x + r * 1.2},${y} ${x},${y + r * 1.2} ${x - r * 1.2},${y}`} fill={color} stroke="#0b0d10" strokeWidth={scale * 0.4} />
  }
  if (outcome === 'fell_missing_rule') {
    return (
      <g>
        <circle cx={x} cy={y} r={r * 1.25} fill="none" stroke={color} strokeWidth={scale * 0.7} />
        <circle cx={x} cy={y} r={r * 0.6} fill={color} stroke="#0b0d10" strokeWidth={scale * 0.2} />
      </g>
    )
  }
  return <circle cx={x} cy={y} r={r} fill={color} stroke="#0b0d10" strokeWidth={scale * 0.4} />
}
