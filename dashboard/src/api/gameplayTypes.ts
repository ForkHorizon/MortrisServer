// Puzzle Gravity gameplay diagnostics — split out of types.ts to keep
// that file under the repo's line-count gate; a self-contained domain
// with no other cross-references.

export interface GameplaySummary {
  attempts: number
  placements: number
  falls: number
  hints: number
  completed_waves: number
  completed_houses: number
  active_elapsed_ms: number
  wall_elapsed_ms: number
  pause_count: number
  pause_elapsed_ms: number
}

export interface GameplayScope extends GameplaySummary {
  city_id: number
  house_id: number
  wave_index: number
}

export interface GameplayFriction {
  block_id: number
  target_id: number
  attempts: number
  placements: number
  falls: number
  hints: number
  fall_rate: number
  first_attempt_failure_rate: number
}

export interface GameplayDiagnostics {
  summary: GameplaySummary
  scopes: GameplayScope[]
  friction: GameplayFriction[]
  daily: GameplayDaily[]
}

export interface GameplayDaily {
  day: string
  city_id: number
  house_id: number
  wave_index: number
  content_revision: string
  build_number: string
  attempts: number
  placements: number
  falls: number
  hints: number
}

export interface GameplayAttemptEvent {
  event_id: string
  name: string
  effective_at: string
  properties: Record<string, unknown>
  missing_support_groups?: number[][]
}

export interface GameplayAttempt {
  attempt_id: string
  content_revision: string
  events: GameplayAttemptEvent[]
  truncated: boolean
}

export interface GameplayPlayer {
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
  last_allocated_memory_mb: number
  last_reserved_memory_mb: number
  last_mono_used_memory_mb: number
  attempts: number
  falls: number
}

export interface GameplayPlayersResult {
  players: GameplayPlayer[]
}
