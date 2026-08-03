import type { PuzzleHouseBlock } from '../api/houseTypes'

export type HouseMetric = 'fall' | 'first_try' | 'no_snap' | 'support' | 'hint' | 'retries' | 'time'

export type MetricReading = { value: number; label: string; sample: number; reliable: boolean; ratio: number }

export const HOUSE_METRICS: Array<{ id: HouseMetric; label: string }> = [
  { id: 'fall', label: 'Falls' },
  { id: 'first_try', label: 'First-try failures' },
  { id: 'no_snap', label: 'Missed slots' },
  { id: 'support', label: 'Missing support' },
  { id: 'hint', label: 'Hints after trouble' },
  { id: 'retries', label: 'Retries to success' },
  { id: 'time', label: 'Time to place' },
]

export function metricReading(block: PuzzleHouseBlock, metric: HouseMetric): MetricReading {
  switch (metric) {
    case 'first_try': return rate(block.first_try_failure_rate, block.first_try_attempts, block.first_try_reliable, 'first tries')
    case 'no_snap': return rate(block.no_snap_rate, block.placements, block.rate_is_reliable, 'drops miss slots')
    case 'support': return rate(block.missing_support_rate, block.placements, block.rate_is_reliable, 'drops lack support')
    case 'hint': return rate(block.hint_pressure_rate, block.placements, block.rate_is_reliable, 'hints after trouble')
    case 'retries': return retries(block)
    case 'time': return timeToPlace(block)
    default: return rate(block.fall_rate, block.placements, block.rate_is_reliable, 'drops fall')
  }
}

function rate(value: number, sample: number, reliable: boolean, suffix: string): MetricReading {
  return { value, sample, reliable, ratio: value, label: `${Math.round(value * 100)}% of ${suffix}` }
}

function retries(block: PuzzleHouseBlock): MetricReading {
  const sample = block.successful_placements
  const value = block.median_tries_to_success
  return { value, sample, reliable: sample >= 5, ratio: Math.max(0, Math.min(1, (value - 1) / 3)), label: value ? `${value} median tries to place` : 'no successful placements yet' }
}

function timeToPlace(block: PuzzleHouseBlock): MetricReading {
  const sample = block.time_to_place_samples
  const value = block.median_time_to_place_ms
  const seconds = Math.round(value / 1000)
  return { value, sample, reliable: sample >= 5, ratio: Math.max(0, Math.min(1, seconds / 45)), label: value ? `${seconds}s median active time` : 'no timing samples yet' }
}

export function metricTitle(metric: HouseMetric): string {
  return HOUSE_METRICS.find((item) => item.id === metric)?.label ?? 'Falls'
}
