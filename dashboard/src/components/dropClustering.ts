import type { PuzzleDrop } from '../api/houseTypes'

export interface DropCluster {
  id: string
  centroidX: number
  centroidY: number
  radius: number
  drops: PuzzleDrop[]
  outcomeCounts: Record<string, number>
  dominantOutcome: string
  exampleAttemptIds: string[]
  nearestTargetId?: number
}

export interface ClusteredDrops {
  clusters: DropCluster[]
  discreteDrops: PuzzleDrop[]
}

export const MIN_CLUSTER_THRESHOLD = 5
export const DEFAULT_PROXIMITY_RADIUS = 350 // milli-units

export function clusterDrops(
  drops: PuzzleDrop[],
  proximityRadius: number = DEFAULT_PROXIMITY_RADIUS,
  minThreshold: number = MIN_CLUSTER_THRESHOLD,
): ClusteredDrops {
  if (!drops || drops.length < minThreshold) {
    return { clusters: [], discreteDrops: drops ? [...drops] : [] }
  }

  const visited = new Set<number>()
  const clusters: DropCluster[] = []
  const discreteDrops: PuzzleDrop[] = []

  for (let i = 0; i < drops.length; i++) {
    if (visited.has(i)) continue

    const componentIndices = gatherConnected(drops, i, proximityRadius)
    for (const idx of componentIndices) {
      visited.add(idx)
    }

    if (componentIndices.length >= minThreshold) {
      const clusterDrops = componentIndices.map((idx) => drops[idx])
      clusters.push(buildCluster(`cluster-${clusters.length + 1}`, clusterDrops))
    } else {
      for (const idx of componentIndices) {
        discreteDrops.push(drops[idx])
      }
    }
  }

  return { clusters, discreteDrops }
}

export const MAX_CLUSTER_DIAMETER = 700 // milli-units maximum span

function isWithinRadiusAndSpan(
  d1: PuzzleDrop,
  d2: PuzzleDrop,
  seed: PuzzleDrop,
  rSquared: number,
  maxSpanSquared: number,
): boolean {
  if (!Number.isFinite(d2.release_x_milli) || !Number.isFinite(d2.release_y_milli)) return false
  const dx = d1.release_x_milli - d2.release_x_milli
  const dy = d1.release_y_milli - d2.release_y_milli
  const dxSeed = seed.release_x_milli - d2.release_x_milli
  const dySeed = seed.release_y_milli - d2.release_y_milli
  return dx * dx + dy * dy <= rSquared && dxSeed * dxSeed + dySeed * dySeed <= maxSpanSquared
}

function gatherConnected(drops: PuzzleDrop[], startIndex: number, radius: number): number[] {
  const seed = drops[startIndex]
  if (!Number.isFinite(seed.release_x_milli) || !Number.isFinite(seed.release_y_milli)) {
    return [startIndex]
  }
  const queue: number[] = [startIndex]
  const inComponent = new Set<number>([startIndex])
  const rSquared = radius * radius
  const maxSpanSquared = MAX_CLUSTER_DIAMETER * MAX_CLUSTER_DIAMETER

  while (queue.length > 0) {
    const curr = queue.shift()!
    const d1 = drops[curr]

    for (let j = 0; j < drops.length; j++) {
      if (inComponent.has(j)) continue
      if (isWithinRadiusAndSpan(d1, drops[j], seed, rSquared, maxSpanSquared)) {
        inComponent.add(j)
        queue.push(j)
      }
    }
  }

  return Array.from(inComponent)
}

function computeCentroidAndCounts(drops: PuzzleDrop[]) {
  let sumX = 0
  let sumY = 0
  const outcomeCounts: Record<string, number> = {}
  const attemptIdsSet = new Set<string>()

  for (const d of drops) {
    sumX += d.release_x_milli
    sumY += d.release_y_milli
    outcomeCounts[d.outcome] = (outcomeCounts[d.outcome] ?? 0) + 1
    if (d.attempt_id) attemptIdsSet.add(d.attempt_id)
  }

  return {
    centroidX: Math.round(sumX / drops.length),
    centroidY: Math.round(sumY / drops.length),
    outcomeCounts,
    exampleAttemptIds: Array.from(attemptIdsSet),
  }
}

function findDominantOutcome(counts: Record<string, number>, fallback: string): string {
  let dominant = fallback
  let maxCount = 0
  for (const [outcome, count] of Object.entries(counts)) {
    if (count > maxCount) {
      maxCount = count
      dominant = outcome
    }
  }
  return dominant
}

function maxSpreadRadius(drops: PuzzleDrop[], cx: number, cy: number): number {
  let maxDist = 0
  for (const d of drops) {
    const dist = Math.hypot(d.release_x_milli - cx, d.release_y_milli - cy)
    if (dist > maxDist) maxDist = dist
  }
  return Math.min(Math.max(220, Math.round(maxDist + 60)), 450)
}

function buildCluster(id: string, drops: PuzzleDrop[]): DropCluster {
  const { centroidX, centroidY, outcomeCounts, exampleAttemptIds } = computeCentroidAndCounts(drops)
  const radius = maxSpreadRadius(drops, centroidX, centroidY)
  const dominantOutcome = findDominantOutcome(outcomeCounts, drops[0].outcome)
  const nearestTargetId = drops.find((d) => (d.nearest_compatible_target_id ?? -1) >= 0)?.nearest_compatible_target_id

  return {
    id,
    centroidX,
    centroidY,
    radius,
    drops,
    outcomeCounts,
    dominantOutcome,
    exampleAttemptIds,
    nearestTargetId,
  }
}

export const OUTCOME_COLORS: Record<string, string> = {
  placed: '#4ade80',
  fell_missing_support: '#f87171',
  fell_no_snap_target: '#fbbf24',
  fell_missing_rule: '#c084fc',
}

export function outcomeColor(outcome: string): string {
  return OUTCOME_COLORS[outcome] ?? '#94a3b8'
}

export function formatOutcomeLabel(outcome: string): string {
  switch (outcome) {
    case 'placed':
      return 'Placed'
    case 'fell_missing_support':
      return 'Missing support'
    case 'fell_no_snap_target':
      return 'No snap target'
    case 'fell_missing_rule':
      return 'Missing rule'
    default:
      return outcome.replace(/_/g, ' ')
  }
}

export function formatDistanceMilli(dist?: number | null): string {
  if (dist == null || dist < 0) return 'unknown distance'
  return `${Math.round(dist)}mm`
}
