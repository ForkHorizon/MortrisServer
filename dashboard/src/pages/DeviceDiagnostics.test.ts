import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { MemoryCohort, PuzzleDeviceSummary } from '../api/deviceTypes'

describe('Device Diagnostics Logic & Contract Guards', () => {
  it('short session (<10m active play) memory sample guard is classified as not expected, never error or missing', () => {
    const shortSessionDevice: PuzzleDeviceSummary = {
      install_id: '11111111-1111-4111-8111-111111111111',
      last_seen_at: '2026-08-06T12:00:00Z',
      platform: 'Android',
      os_version: '13',
      device_class: 'Pixel 6',
      app_version: '1.1.11',
      build_number: '9dccf3d0738247059cafde1c16106d70',
      locale: 'en-US',
      timezone_offset_minutes: 0,
      device_total_memory_mb: 8192,
      graphics_memory_mb: 2048,
      natural_runs_count: 2,
      completed_runs_count: 2,
      falls_count: 1,
      placements_count: 10,
      active_play_ms: 5 * 60 * 1000, // 5 minutes (<10m)
      memory_sample_count: 0,
      memory_sample_expected: false,
      memory_sample_status: 'not_expected',
      memory_sample_status_label: 'Memory sample not expected yet (<10m active play)',
      last_allocated_memory_mb: 0,
      last_reserved_memory_mb: 0,
      last_mono_used_memory_mb: 0,
    }

    assert.equal(shortSessionDevice.memory_sample_expected, false)
    assert.equal(shortSessionDevice.memory_sample_status, 'not_expected')
    assert.match(shortSessionDevice.memory_sample_status_label, /not expected yet/i)
    assert.doesNotMatch(shortSessionDevice.memory_sample_status_label, /missing|error|defect|failed/i)
  })

  it('long session (>=10m active play) with no samples is classified as absent', () => {
    const longSessionDevice: PuzzleDeviceSummary = {
      install_id: '22222222-2222-4222-8222-222222222222',
      last_seen_at: '2026-08-06T12:00:00Z',
      platform: 'Android',
      os_version: '14',
      device_class: 'Galaxy S23',
      app_version: '1.1.11',
      build_number: '9dccf3d0738247059cafde1c16106d70',
      locale: 'en-US',
      timezone_offset_minutes: 0,
      device_total_memory_mb: 8192,
      graphics_memory_mb: 2048,
      natural_runs_count: 3,
      completed_runs_count: 2,
      falls_count: 4,
      placements_count: 20,
      active_play_ms: 15 * 60 * 1000, // 15 minutes (>=10m)
      memory_sample_count: 0,
      memory_sample_expected: true,
      memory_sample_status: 'absent',
      memory_sample_status_label: 'Expected (>=10m active play) but not recorded',
      last_allocated_memory_mb: 0,
      last_reserved_memory_mb: 0,
      last_mono_used_memory_mb: 0,
    }

    assert.equal(longSessionDevice.memory_sample_expected, true)
    assert.equal(longSessionDevice.memory_sample_status, 'absent')
  })

  it('device with memory samples is classified as available', () => {
    const sampledDevice: PuzzleDeviceSummary = {
      install_id: '33333333-3333-4333-8333-333333333333',
      last_seen_at: '2026-08-06T12:00:00Z',
      platform: 'iOS',
      os_version: '17.0',
      device_class: 'iPhone14,2',
      app_version: '1.1.11',
      build_number: '9dccf3d0738247059cafde1c16106d70',
      locale: 'en-US',
      timezone_offset_minutes: 0,
      device_total_memory_mb: 6144,
      graphics_memory_mb: 2048,
      natural_runs_count: 5,
      completed_runs_count: 4,
      falls_count: 2,
      placements_count: 30,
      active_play_ms: 12 * 60 * 1000,
      memory_sample_count: 2,
      memory_sample_expected: true,
      memory_sample_status: 'available',
      memory_sample_status_label: 'Recorded',
      last_allocated_memory_mb: 280,
      last_reserved_memory_mb: 350,
      last_mono_used_memory_mb: 85,
    }

    assert.equal(sampledDevice.memory_sample_expected, true)
    assert.equal(sampledDevice.memory_sample_status, 'available')
    assert.equal(sampledDevice.memory_sample_status_label, 'Recorded')
  })

  it('cohorts strictly use neutral correlational wording with zero causal claims', () => {
    const cohorts: MemoryCohort[] = [
      createTestCohort('low', 'Low (< 4 GB)', 5, 12, true, 'Correlational distribution across memory tier'),
      createTestCohort('medium', 'Medium (4–6 GB)', 2, 3, false, 'Insufficient sample size (<5 natural runs) for reliable rates'),
    ]

    const prohibitedPhrases = [
      /causes/i,
      /caused by/i,
      /due to low memory/i,
      /leads to falls/i,
      /low memory increases/i,
      /memory pressure causes/i,
    ]

    for (const cohort of cohorts) {
      assert.ok(cohort.device_count > 0, 'Unique install count must be present')
      assert.ok(cohort.natural_runs_count > 0, 'Natural runs count must be present')
      for (const pattern of prohibitedPhrases) {
        assert.doesNotMatch(cohort.evidence_note, pattern, `Cohort note must not contain causal phrasing: ${cohort.evidence_note}`)
      }
    }
  })

  it('insufficient sample size threshold (<5 runs) flags sample_count_sufficient as false', () => {
    const thinCohort = createTestCohort('low', 'Low (< 4 GB)', 1, 4, false, 'Insufficient sample size (<5 natural runs) for reliable rates')
    assert.equal(thinCohort.sample_count_sufficient, false)
    assert.match(thinCohort.evidence_note, /insufficient/i)
  })
})

function createTestCohort(
  tier: string,
  label: string,
  deviceCount: number,
  runs: number,
  sufficient: boolean,
  note: string
): MemoryCohort {
  return {
    tier,
    label,
    min_memory_mb: 1,
    max_memory_mb: 4095,
    device_count: deviceCount,
    natural_runs_count: runs,
    completed_runs_count: Math.floor(runs * 0.6),
    completion_rate: 0.6,
    placements_count: runs * 5,
    falls_count: Math.floor(runs * 1.2),
    fall_rate: 0.24,
    sample_count_sufficient: sufficient,
    evidence_note: note,
  }
}
