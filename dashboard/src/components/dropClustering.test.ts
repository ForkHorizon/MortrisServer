import test from 'node:test'
import assert from 'node:assert/strict'
import type { PuzzleDrop } from '../api/houseTypes.ts'
import {
  clusterDrops,
  formatDistanceMilli,
  formatOutcomeLabel,
  outcomeColor,
  MIN_CLUSTER_THRESHOLD,
} from './dropClustering.ts'

function makeDrop(
  id: string,
  x: number,
  y: number,
  outcome: string = 'placed',
  nearestId: number = 10,
  distance: number = 45,
): PuzzleDrop {
  return {
    block_id: 1,
    target_id: 10,
    candidate_target_id: 10,
    nearest_compatible_target_id: nearestId,
    nearest_distance_milli: distance,
    outcome,
    release_x_milli: x,
    release_y_milli: y,
    target_x_milli: 100,
    target_y_milli: 200,
    attempt_id: `att-${id}`,
  }
}

test('threshold guard: < 5 points never fabricate a cluster density halo', () => {
  // Even if drops are in the exact same spot, 1 to 4 drops must not form a cluster
  for (let count = 1; count < MIN_CLUSTER_THRESHOLD; count++) {
    const drops = Array.from({ length: count }, (_, i) => makeDrop(`drop-${i}`, 100, 200))
    const result = clusterDrops(drops)
    assert.equal(result.clusters.length, 0, `count ${count} should have 0 clusters`)
    assert.equal(result.discreteDrops.length, count, `all ${count} drops should remain discrete`)
  }
})

test('cluster formation: >= 5 points within proximity radius form a cluster', () => {
  const drops: PuzzleDrop[] = [
    makeDrop('1', 100, 200, 'placed'),
    makeDrop('2', 110, 205, 'placed'),
    makeDrop('3', 95, 195, 'fell_missing_support'),
    makeDrop('4', 105, 210, 'placed'),
    makeDrop('5', 90, 190, 'placed'),
  ]

  const result = clusterDrops(drops, 350, 5)
  assert.equal(result.clusters.length, 1)
  assert.equal(result.discreteDrops.length, 0)

  const cluster = result.clusters[0]
  assert.equal(cluster.drops.length, 5)
  assert.equal(cluster.centroidX, 100)
  assert.equal(cluster.centroidY, 200)
  assert.equal(cluster.dominantOutcome, 'placed')
  assert.equal(cluster.outcomeCounts.placed, 4)
  assert.equal(cluster.outcomeCounts.fell_missing_support, 1)
  assert.equal(cluster.exampleAttemptIds.length, 5)
})

test('mixed spatial layout: isolates distant drops from tight clusters', () => {
  const clusterDropsInput: PuzzleDrop[] = [
    makeDrop('1', 500, 500),
    makeDrop('2', 510, 500),
    makeDrop('3', 500, 510),
    makeDrop('4', 520, 510),
    makeDrop('5', 490, 490),
  ]
  const distantDrops: PuzzleDrop[] = [
    makeDrop('distant-1', 2000, 2000),
    makeDrop('distant-2', -1000, -1000),
  ]

  const result = clusterDrops([...clusterDropsInput, ...distantDrops], 350, 5)
  assert.equal(result.clusters.length, 1)
  assert.equal(result.clusters[0].drops.length, 5)
  assert.equal(result.discreteDrops.length, 2)
  assert.ok(result.discreteDrops.some((d) => d.attempt_id === 'att-distant-1'))
  assert.ok(result.discreteDrops.some((d) => d.attempt_id === 'att-distant-2'))
})

test('handles negative coordinates properly in centroid and radius', () => {
  const negDrops: PuzzleDrop[] = [
    makeDrop('1', -400, -600),
    makeDrop('2', -410, -590),
    makeDrop('3', -390, -610),
    makeDrop('4', -405, -605),
    makeDrop('5', -395, -595),
  ]

  const result = clusterDrops(negDrops, 350, 5)
  assert.equal(result.clusters.length, 1)
  const cluster = result.clusters[0]
  assert.equal(cluster.centroidX, -400)
  assert.equal(cluster.centroidY, -600)
  assert.ok(cluster.radius >= 220)
})

test('formatting helpers: outcome label, color, and distance formatting', () => {
  assert.equal(formatOutcomeLabel('placed'), 'Placed')
  assert.equal(formatOutcomeLabel('fell_missing_support'), 'Missing support')
  assert.equal(formatOutcomeLabel('fell_no_snap_target'), 'No snap target')
  assert.equal(formatOutcomeLabel('fell_missing_rule'), 'Missing rule')

  assert.equal(outcomeColor('placed'), '#4ade80')
  assert.equal(outcomeColor('fell_missing_support'), '#f87171')

  assert.equal(formatDistanceMilli(45.2), '45mm')
  assert.equal(formatDistanceMilli(0), '0mm')
  assert.equal(formatDistanceMilli(null), 'unknown distance')
})

test('span bounding: collinear drops spanning beyond MAX_CLUSTER_DIAMETER do not chain infinitely', () => {
  // 6 collinear drops spaced 200mm apart: total span = 1000mm (> MAX_CLUSTER_DIAMETER = 700mm)
  const collinearDrops: PuzzleDrop[] = [
    makeDrop('1', 0, 0),
    makeDrop('2', 200, 0),
    makeDrop('3', 400, 0),
    makeDrop('4', 600, 0),
    makeDrop('5', 800, 0),
    makeDrop('6', 1000, 0),
  ]

  const result = clusterDrops(collinearDrops, 350, 5)
  // Cannot chain all 6 drops across 1000mm into a single cluster
  if (result.clusters.length > 0) {
    for (const cluster of result.clusters) {
      assert.ok(cluster.radius <= 450, `radius ${cluster.radius} should be capped at 450`)
      for (const d of cluster.drops) {
        // All drops in cluster must be within MAX_CLUSTER_DIAMETER of the seed
        assert.ok(Math.abs(d.release_x_milli - cluster.drops[0].release_x_milli) <= 700)
      }
    }
  }
})

test('gracefully handles non-finite and NaN coordinates', () => {
  const badDrops: PuzzleDrop[] = [
    makeDrop('1', NaN, 100),
    makeDrop('2', 100, Infinity),
    makeDrop('3', 200, 200),
  ]
  const result = clusterDrops(badDrops, 350, 2)
  assert.ok(result)
  // NaN/Infinity should not crash
})
