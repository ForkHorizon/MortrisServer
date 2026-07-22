// Mirrors the JSON shapes from contracts/openapi.yaml's dashboard schemas.
// Keep in sync by hand — see internal/analytics and internal/policyadmin
// for the Go source of truth.

export interface SessionInfo {
  username: string
  email: string
  role: 'owner' | 'member'
  project_ids: string[]
  projects: ProjectAccess[]
}

export interface ProjectAccess {
  id: string
  display_name: string
  environment: string
  role: 'project_admin' | 'viewer'
}

export interface LoginResponse extends SessionInfo {
  expires_at: string
}

export interface ManagedProject {
  id: string
  display_name: string
  environment: string
  retention_days: number
  strict_catalog: boolean
  enabled: boolean
  archived_at?: string
  sdk_test_enabled: boolean
  sdk_test_scenario: string
}

export interface ProjectMember {
  id: number
  username: string
  email?: string
  access_role: 'project_admin' | 'viewer'
  disabled: boolean
}

export interface Overview {
  product_events: number
  new_installations: number
  daily_active_installations: number
  weekly_active_installations: number
  monthly_active_installations: number
  sessions: number
  avg_observed_session_duration_ms: number
  ingestion_accepted: number
  ingestion_duplicates: number
  ingestion_rejected: number
}

export interface DayCount {
  day: string
  count: number
}

export interface EventExplorerResult {
  total_events: number
  active_installations: number
  trend: DayCount[]
}

export interface FunnelStep {
  name: string
  count: number
  conversion_from_first: number
  conversion_from_previous: number
}

export interface FunnelResult {
  steps: FunnelStep[]
  completion_window_seconds: number
  truncated: boolean
}

export interface RetentionCohort {
  cohort_day: string
  cohort_size: number
  d1: number
  d7: number
  d30: number
}

export interface RetentionResult {
  cohorts: RetentionCohort[]
}

export interface TimelineEvent {
  event_id: string
  name: string
  event_kind: 'product' | 'system'
  effective_at: string
  time_quality: 'client' | 'batch_adjusted' | 'untrusted'
  properties: Record<string, unknown>
}

export interface TimelineResult {
  install_id: string
  registered_at: string
  activated_at?: string
  events: TimelineEvent[]
  truncated: boolean
}

export interface CatalogEntry {
  name: string
  kind: 'product' | 'system'
  description: string
  owner: string
  first_schema_version: number
  properties: Array<{ name: string; type: string; required?: boolean; description?: string }>
  known: boolean
  first_seen_at?: string
  last_seen_at?: string
}

export interface CatalogResult {
  entries: CatalogEntry[]
}

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

export interface PoolStats {
  acquired_conns: number
  idle_conns: number
  total_conns: number
  max_conns: number
}

export interface MaintenanceRunSummary {
  kind: string
  started_at: string
  finished_at?: string
  rows_affected: number
  error?: string
}

export interface SystemHealth {
  version: string
  db_latency_ms: number
  writer_pool: PoolStats
  reader_pool: PoolStats
  disk_state: 'normal' | 'warning' | 'high' | 'critical' | 'rejecting'
  ingestion_accepted_last_hour: number
  ingestion_rejected_last_hour: number
  enabled_policy_rules: number
  last_maintenance_runs: MaintenanceRunSummary[]
}

export interface PolicyRule {
  id: number
  project_id: string
  environment?: string
  app_version?: string
  build_number?: string
  sdk_version?: string
  mode: 'active' | 'pause_upload' | 'disable_collection'
  next_check_seconds: number
  discard_pending: boolean
  reason: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface PolicyCreateInput {
  project_id: string
  environment?: string
  app_version?: string
  build_number?: string
  sdk_version?: string
  mode: 'active' | 'pause_upload' | 'disable_collection'
  next_check_seconds: number
  discard_pending: boolean
  reason: string
}

export interface ApiErrorBody {
  server_time: string
  code: string
  message: string
  request_id: string
}
