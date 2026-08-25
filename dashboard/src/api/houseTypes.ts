// House view types (docs/puzzle-visual-analytics-plan.md), kept out of
// types.ts so that file stays under the 300-line gate.
export interface PuzzleBlockBounds {
  min_x: number
  min_y: number
  max_x: number
  max_y: number
}

export interface PuzzleHouseBlock {
  block_id: number
  wave_index: number
  order_in_layer: number
  is_ground: boolean
  visual_key: string
  bounds_milli?: PuzzleBlockBounds
  outline_milli?: Array<[number, number]>
  required_groups: number[][]
  placements: number
  falls: number
  fall_rate: number
  falls_by_reason: Record<string, number>
  rate_is_reliable: boolean
  first_try_failures: number
  first_try_attempts: number
  first_try_failure_rate: number
  first_try_reliable: boolean
  no_snap_rate: number
  missing_support_rate: number
  hint_pressure_count: number
  hint_pressure_rate: number
  median_tries_to_success: number
  successful_placements: number
  median_time_to_place_ms: number
  time_to_place_samples: number
}

export interface PuzzleWaveSummary {
  wave_index: number
  attempts: number
  placements: number
  falls: number
  fall_rate: number
}

export interface PuzzleDataQuality {
  legacy_revision_events: number
  coordinate_status: 'trusted' | 'legacy_only' | 'mixed_coordinate_spaces'
}

export interface PuzzleHouseDetail {
  city_id: number
  house_id: number
  display_label: string
  content_revision: string
  wave_count: number
  blocks: PuzzleHouseBlock[]
  waves: PuzzleWaveSummary[]
  data_quality: PuzzleDataQuality
}

export interface PuzzleHouseSummary {
  city_id: number
  house_id: number
  display_label: string
  attempts: number
  opened_attempts: number
  completed_attempts: number
  unique_installations: number
  played_detail_count: number
  reliable_detail_count: number
  dominant_failure: string
  placements: number
  falls: number
  hints: number
  fall_rate: number
  rate_is_reliable: boolean
  has_geometry: boolean
  has_art: boolean
}

export interface PuzzleHouseList {
  content_revision: string
  houses: PuzzleHouseSummary[]
}

export interface PuzzleOverview {
  total_houses: number
  played_houses: number
  reliable_houses: number
  insufficient_houses: number
  worst_house?: PuzzleHouseSummary
  worst_wave?: PuzzleWaveSummary
  failure_reasons: Record<string, number>
  data_quality: PuzzleDataQuality
}

export interface PuzzleDrop {
  block_id: number
  target_id: number
  outcome: string
  release_x_milli: number
  release_y_milli: number
  target_x_milli: number
  target_y_milli: number
  attempt_id: string
}

export interface PuzzleDropMap {
  drops: PuzzleDrop[]
  unresolved_drops: number
  offset_x_milli: number
  offset_y_milli: number
  aligned: boolean
  alignment_issue?: 'too_few_placed' | 'inconsistent' | 'mixed_coordinate_spaces'
  offset_spread_milli: number
  trusted_coordinates: boolean
}

export interface PuzzleReplayStep {
  index: number
  name: string
  at: string
  block_id: number
  target_id: number
  outcome: string
  rule_state: string
  wave_index: number
  release_x_milli: number
  release_y_milli: number
  active_elapsed_ms: number
  interaction_id?: string
  origin?: string
  progress_origin?: string
  developer_action_id?: string
  placed: number[]
  missing_support?: number[][]
}

export interface PuzzleReplay {
  attempt_id: string
  city_id: number
  house_id: number
  install_id: string
  steps: PuzzleReplayStep[]
  truncated: boolean
  sequence_gaps: number
  sequence_duplicates: number
  orphan_interactions: number
  state_hash_mismatches: number
  revision_warning?: string
}

export interface PuzzleAttemptSummary {
  attempt_id: string
  install_id: string
  started_at: string
  wave_index: number
  placements: number
  falls: number
  hints: number
  completed: boolean
  app_version: string
  build_number: string
  active_duration_ms: number
  last_wave_index: number
  dominant_failure: string
  progress_origin: string
  developer_affected: boolean
}

export interface PuzzleAttemptList {
  attempts: PuzzleAttemptSummary[]
}

// Stage 2 data-quality control plane (docs/puzzle-analytics-remaining-plan.md
// section 5) — mirrors internal/analytics/puzzle_quality.go's PuzzleQuality.
export interface PuzzleQualityRejection {
  name: string
  code: string
  count: number
}

export interface PuzzleQualityDelay {
  samples: number
  p50_ms: number
  p90_ms: number
  p99_ms: number
  max_ms: number
}

export interface PuzzleQualityBuild {
  app_version: string
  build_number: string
  events: number
}

export type PuzzleQualityStatus = 'grey' | 'green' | 'amber' | 'red'

export interface PuzzleQuality {
  received_events: number
  accepted_events: number
  duplicate_events: number
  rejected_events: number
  rejections: PuzzleQualityRejection[]

  total_gameplay_events: number
  schema_v2_events: number
  schema_v2_share: number
  missing_schema_version_events: number
  invalid_schema_version_events: number

  unknown_revision_events: number
  invalid_coordinate_space_events: number

  distinct_installs: number
  distinct_sessions: number
  distinct_house_runs: number
  distinct_wave_attempts: number
  distinct_interactions: number

  sequence_gaps: number
  sequence_duplicates: number
  house_event_index_gaps: number
  house_event_index_duplicates: number
  attempt_event_index_gaps: number
  attempt_event_index_duplicates: number
  invalid_event_indexes: number

  orphan_interactions: number
  checkpoint_mismatches: number
  recovered_attempts: number

  open_house_runs_recent: number
  open_house_runs_stale: number
  open_attempts_recent: number
  open_attempts_stale: number

  unpaired_developer_commands: number
  unpaired_developer_mutations: number

  delivery_delay_ms: PuzzleQualityDelay
  builds: PuzzleQualityBuild[]

  status: PuzzleQualityStatus
  status_reasons: string[]
}
