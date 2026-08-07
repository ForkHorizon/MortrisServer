// Stage 2-4 control-plane types (docs/puzzle-analytics-remaining-plan.md),
// kept out of houseTypes.ts so that file stays under the 300-line gate.

// Stage 4 wave staircase/funnel (section 7) — mirrors
// internal/analytics/puzzle_wave_funnel.go.
export interface PuzzleWaveFunnelStep {
  wave_index: number
  entered: number
  completed: number
  reached_next: number
  abandoned_here: number
  recent_here: number
}

export interface PuzzleWaveFunnel {
  total_waves: number
  steps: PuzzleWaveFunnelStep[]
  lost_threshold_hours: number
}

// Stage 2 data-quality control plane (section 5) — mirrors
// internal/analytics/puzzle_quality.go's PuzzleQuality.
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
  missing_schema_version_events: number

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

// Stage 3 traffic segmentation (section 6) — mirrors
// internal/analytics/puzzle_traffic.go's TrafficScope and
// internal/analytics/puzzle_tester_impact.go's PuzzleTesterImpact.
export type TrafficScope = 'natural_only' | 'developer_affected' | 'all'

export interface PuzzleTesterCommandCount {
  command: string
  result: string
  count: number
}

export interface PuzzleTesterImpact {
  developer_actions: number
  affected_house_runs: number
  affected_attempts: number
  commands: PuzzleTesterCommandCount[]
  resets: number
  progress_completions: number
  failures: number
  noops: number
  mutations_with_before_after: number
  mutations_missing_before_after: number
}
