# Metric contract

Authoritative definitions per server_implementation_plan.md section 9. SQL
implementations and fixture-dataset tests land in Phase S2/S3 alongside the
query API (`internal/analytics`) — this document is the spec those
implementations must match, and what the dashboard tooltips quote from.

All time bucketing uses a validated IANA timezone (rejecting invalid zone
names, not silently falling back to UTC) and must handle daylight-saving
transitions correctly — a "day" in a DST-transition timezone is not always
24 hours.

## Definitions

- **Product events.** Accepted rows where `event_kind = 'product'`. System
  events (`sys_*`) never count toward any metric below.

- **New installations.** Installations grouped by the calendar date of
  `first_product_event_at`, in the selected timezone. Registration alone
  does not count — an installation that registers but never emits a
  product event is never "new" by this metric.

- **DAU / WAU / MAU.** Count of distinct `install_id` with at least one
  product event in the selected calendar day / trailing 7-day / trailing
  30-day interval. The UI may show "DAU"/"WAU"/"MAU" as labels but must
  expand them as "active installations" on first reference or in a tooltip
  — never "users".

- **Sessions.** Count of distinct `(project_id, install_id, session_id)`
  tuples that contain at least one product event. A session with only
  system events (e.g. only `sys_session_start` + `sys_app_background`, no
  product event) does not count as a session for this metric.

- **Observed session duration.** For each session that contains a product
  event, take the maximum `session_elapsed_ms` across all events in that
  session — including `sys_app_background` if delivered. Average those
  per-session maxima across the selected interval. Label this metric
  exactly "Average observed session duration" in the UI, with a tooltip
  noting that process death (no background event, no next-session event)
  can underestimate individual sessions. This is a deliberate consequence
  of there being no guaranteed session-end event (section 5, decision 7) —
  do not build logic that infers a "true" end time.

- **Installation retention D1/D7/D30.** Cohort installations by the
  calendar date of `first_product_event_at` in the selected timezone. An
  installation is "retained" at D*N* if the same `install_id` has a
  product event on cohort-day + N in that timezone. A reinstall or cleared
  app data creates a new `install_id` and therefore a new cohort member —
  it does not extend the old installation's retention curve. The UI must
  state this near the retention screen (section 9, last paragraph).

- **Funnel.** An ordered sequence of 2 to 5 product event names for the
  same installation, each step's event required to occur within a
  configured completion window measured from the funnel's first step.
  Each funnel definition must state explicitly whether a repeated step
  uses the first qualifying occurrence or another rule — this is not a
  single implicit default, and the SQL test suite must cover the chosen
  rule.

## Non-goals for v1

- No metric may imply data exists beyond a project's configured
  `retention_days`, or beyond the 90-day maximum raw-data window
  (section 10.1).
- No metric treats an anonymous installation as a person; "reinstall = new
  installation" is accepted behavior, not a bug to work around with
  cross-device or fingerprint-based identity resolution (section 2, out of
  scope for v1).
