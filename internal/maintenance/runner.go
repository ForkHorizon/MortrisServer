package maintenance

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// runInterval is fixed at hourly rather than switching between "daily
// normally, more frequently under disk pressure" (section 8.5): hourly
// already satisfies both — it's more frequent than daily, and each run is
// cheap (bounded batches, near no-op when nothing is due) — so a
// disk-pressure-aware scheduler would add complexity without changing
// behavior. Revisit if a real deployment shows retention lagging disk
// growth at this cadence.
const runInterval = time.Hour

type Runner struct {
	Pool *pgxpool.Pool
	Log  *slog.Logger
}

func (r *Runner) Run(ctx context.Context) {
	r.runOnce(ctx)
	ticker := time.NewTicker(runInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	r.runAndRecord(ctx, "unactivated_cleanup", DeleteUnactivatedInstallations)
	r.runAndRecord(ctx, "retention_delete", DeleteExpiredEvents)
}

func (r *Runner) runAndRecord(ctx context.Context, kind string, fn func(context.Context, *pgxpool.Pool) (int64, error)) {
	started := time.Now().UTC()
	rows, err := fn(ctx, r.Pool)
	finished := time.Now().UTC()

	var errMsg *string
	if err != nil {
		msg := err.Error()
		errMsg = &msg
		r.Log.Error("maintenance run failed", "kind", kind, "error", err)
	} else {
		r.Log.Info("maintenance run", "kind", kind, "rows_affected", rows, "duration_ms", finished.Sub(started).Milliseconds())
	}

	if _, recErr := r.Pool.Exec(ctx, `
		INSERT INTO maintenance_runs (kind, started_at, finished_at, rows_affected, error)
		VALUES ($1, $2, $3, $4, $5)
	`, kind, started, finished, rows, errMsg); recErr != nil {
		r.Log.Error("failed to record maintenance run", "kind", kind, "error", recErr)
	}
}
