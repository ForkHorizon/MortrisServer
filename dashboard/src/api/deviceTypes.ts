// Stage 6 device and memory diagnostics types (docs/puzzle-analytics-remaining-plan.md Section 9)

export type MemorySampleStatus = 'not_expected' | 'available' | 'absent'

export interface PuzzleDeviceSummary {
  install_id: string
  last_seen_at: string
  platform: string
  os_version: string
  device_class: string
  app_version: string
  build_number: string
  locale: string
  timezone_offset_minutes: number
  device_total_memory_mb: number
  graphics_memory_mb: number
  natural_runs_count: number
  completed_runs_count: number
  falls_count: number
  placements_count: number
  active_play_ms: number
  memory_sample_count: number
  memory_sample_expected: boolean
  memory_sample_status: MemorySampleStatus
  memory_sample_status_label: string
  last_allocated_memory_mb: number
  last_reserved_memory_mb: number
  last_mono_used_memory_mb: number
}

export interface MemoryCohort {
  tier: string
  label: string
  min_memory_mb: number
  max_memory_mb: number
  device_count: number
  natural_runs_count: number
  completed_runs_count: number
  completion_rate: number
  placements_count: number
  falls_count: number
  fall_rate: number
  sample_count_sufficient: boolean
  evidence_note: string
}

export interface PuzzleDeviceListResult {
  devices: PuzzleDeviceSummary[]
  cohorts: MemoryCohort[]
}

export interface MemoryTimelinePoint {
  event_id: string
  effective_at: string
  active_time_ms: number
  wall_time_ms: number
  event_name: string
  marker_type: string
  marker_label: string
  app_allocated_mb?: number
  app_reserved_mb?: number
  mono_used_mb?: number
  city_id?: number
  house_id?: number
  wave_index?: number
  attempt_id?: string
  build_number: string
  app_version: string
}

export interface PuzzleMemoryTimeline {
  install_id: string
  device_total_memory_mb: number
  graphics_memory_mb: number
  total_active_time_ms: number
  total_wall_time_ms: number
  sample_count: number
  points: MemoryTimelinePoint[]
}
