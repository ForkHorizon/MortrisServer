package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/diskstate"
)

// Version is reported on the System Health screen (section 10.2 #7).
// Bumped by hand — build-time ldflags injection is more machinery than a
// single internal admin tool's version string needs right now.
const Version = "0.1.0"

type PoolStats struct {
	AcquiredConns int32 `json:"acquired_conns"`
	IdleConns     int32 `json:"idle_conns"`
	TotalConns    int32 `json:"total_conns"`
	MaxConns      int32 `json:"max_conns"`
}

func statsOf(pool *pgxpool.Pool) PoolStats {
	s := pool.Stat()
	return PoolStats{
		AcquiredConns: s.AcquiredConns(),
		IdleConns:     s.IdleConns(),
		TotalConns:    s.TotalConns(),
		MaxConns:      s.MaxConns(),
	}
}

type MaintenanceRunSummary struct {
	Kind         string     `json:"kind"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	RowsAffected int64      `json:"rows_affected"`
	Error        *string    `json:"error,omitempty"`
}

type SystemHealth struct {
	Version                   string                  `json:"version"`
	DBLatencyMs               float64                 `json:"db_latency_ms"`
	WriterPool                PoolStats               `json:"writer_pool"`
	ReaderPool                PoolStats               `json:"reader_pool"`
	DiskState                 diskstate.State         `json:"disk_state"`
	IngestionAcceptedLastHour int64                   `json:"ingestion_accepted_last_hour"`
	IngestionRejectedLastHour int64                   `json:"ingestion_rejected_last_hour"`
	EnabledPolicyRules        int64                   `json:"enabled_policy_rules"`
	LastMaintenanceRuns       []MaintenanceRunSummary `json:"last_maintenance_runs"`
}

// GetSystemHealth implements section 10.2 #7. projectIDs scopes the
// ingestion/policy figures to whatever the caller's session can see
// (section 10.3) — DB/pool/disk state are cluster-wide and shown to any
// authenticated session regardless of project scope, since they aren't
// project data.
func GetSystemHealth(ctx context.Context, writerPool, readerPool *pgxpool.Pool, disk diskstate.State, projectIDs []string) (*SystemHealth, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	health := &SystemHealth{
		Version:    Version,
		WriterPool: statsOf(writerPool),
		ReaderPool: statsOf(readerPool),
		DiskState:  disk,
	}

	start := time.Now()
	var one int
	if err := readerPool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return nil, err
	}
	health.DBLatencyMs = float64(time.Since(start).Microseconds()) / 1000

	if len(projectIDs) > 0 {
		if err := readerPool.QueryRow(ctx, `
			SELECT COALESCE(SUM(accepted_count), 0), COALESCE(SUM(rejected_count), 0)
			FROM ingestion_stats
			WHERE project_id = ANY($1) AND received_at >= clock_timestamp() - interval '1 hour'
		`, projectIDs).Scan(&health.IngestionAcceptedLastHour, &health.IngestionRejectedLastHour); err != nil {
			return nil, err
		}

		if err := readerPool.QueryRow(ctx, `
			SELECT COUNT(*) FROM client_policy_rules WHERE project_id = ANY($1) AND enabled
		`, projectIDs).Scan(&health.EnabledPolicyRules); err != nil {
			return nil, err
		}
	}

	rows, err := readerPool.Query(ctx, `
		SELECT DISTINCT ON (kind) kind, started_at, finished_at, rows_affected, error
		FROM maintenance_runs
		ORDER BY kind, started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m MaintenanceRunSummary
		if err := rows.Scan(&m.Kind, &m.StartedAt, &m.FinishedAt, &m.RowsAffected, &m.Error); err != nil {
			return nil, err
		}
		health.LastMaintenanceRuns = append(health.LastMaintenanceRuns, m)
	}
	return health, rows.Err()
}
