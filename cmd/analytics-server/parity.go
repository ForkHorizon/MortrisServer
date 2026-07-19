package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

// runParityReport implements the dual-send comparison tool (section 11):
// event count by name/build/day, distinct installations, duplicate and
// rejection counts (from ingestion_stats — events alone can't answer
// this), time-quality distribution, and sequence gaps/anomalies.
func runParityReport(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("parity-report", flag.ExitOnError)
	project := fs.String("project", "", "project ID (required)")
	from := fs.String("from", "", "RFC3339 start of range, inclusive (required)")
	to := fs.String("to", "", "RFC3339 end of range, exclusive (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *from == "" || *to == "" {
		return fmt.Errorf("--project, --from, and --to are all required")
	}
	fromT, err := time.Parse(time.RFC3339, *from)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	toT, err := time.Parse(time.RFC3339, *to)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}
	if !toT.After(fromT) {
		return fmt.Errorf("--to must be after --from")
	}

	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintf(w, "Parity report: project=%s from=%s to=%s\n", *project, fromT.Format(time.RFC3339), toT.Format(time.RFC3339))

	var distinctInstalls int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT install_id) FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
	`, *project, fromT, toT).Scan(&distinctInstalls); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "\nDistinct installations:\t%d\n", distinctInstalls)

	_, _ = fmt.Fprintln(w, "\nEvent count by name / build_number / day:")
	_, _ = fmt.Fprintln(w, "name\tbuild_number\tday\tcount")
	rows, err := pool.Query(ctx, `
		SELECT name, build_number, (effective_at AT TIME ZONE 'UTC')::date AS day, COUNT(*)
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY name, build_number, day
		ORDER BY day, name, build_number
	`, *project, fromT, toT)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, build string
		var day time.Time
		var count int64
		if err := rows.Scan(&name, &build, &day, &count); err != nil {
			rows.Close()
			return err
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", name, build, day.Format("2006-01-02"), count)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(w, "\nTime-quality distribution:")
	_, _ = fmt.Fprintln(w, "time_quality\tcount")
	tqRows, err := pool.Query(ctx, `
		SELECT time_quality, COUNT(*) FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY time_quality ORDER BY time_quality
	`, *project, fromT, toT)
	if err != nil {
		return err
	}
	for tqRows.Next() {
		var quality string
		var count int64
		if err := tqRows.Scan(&quality, &count); err != nil {
			tqRows.Close()
			return err
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\n", quality, count)
	}
	tqRows.Close()
	if err := tqRows.Err(); err != nil {
		return err
	}

	var acceptedSum, dupSum, rejSum int64
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(accepted_count), 0), COALESCE(SUM(duplicate_count), 0), COALESCE(SUM(rejected_count), 0)
		FROM ingestion_stats
		WHERE project_id = $1 AND received_at >= $2 AND received_at < $3
	`, *project, fromT, toT).Scan(&acceptedSum, &dupSum, &rejSum); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(w, "\nIngestion stats (by received_at, not effective_at):")
	_, _ = fmt.Fprintf(w, "accepted\t%d\nduplicates\t%d\nrejected\t%d\n", acceptedSum, dupSum, rejSum)

	anomalyCount, examples, err := sequenceAnomalies(ctx, pool, *project, fromT, toT)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "\nSequence anomalies (gap != 1): %d\n", anomalyCount)
	if len(examples) > 0 {
		_, _ = fmt.Fprintln(w, "install_id\tsession_id\tsequence\tgap")
		for _, ex := range examples {
			_, _ = fmt.Fprintln(w, ex)
		}
		if anomalyCount > int64(len(examples)) {
			_, _ = fmt.Fprintf(w, "... showing first %d of %d\n", len(examples), anomalyCount)
		}
	}

	return nil
}

const maxAnomalyExamples = 20

// sequenceAnomalies finds (install_id, session_id) positions where
// `sequence` jumps by anything other than 1 — a gap suggests dropped
// events, a non-positive gap suggests reordering or a client-side
// sequence reset (section 15: "Duplicate, overlapping, and reordered
// event batches").
func sequenceAnomalies(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (int64, []string, error) {
	rows, err := pool.Query(ctx, `
		SELECT install_id, session_id, sequence,
		       sequence - LAG(sequence) OVER (PARTITION BY install_id, session_id ORDER BY sequence) AS gap
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
	`, projectID, from, to)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var count int64
	var examples []string
	for rows.Next() {
		var installID, sessionID string
		var sequence int64
		var gap *int64
		if err := rows.Scan(&installID, &sessionID, &sequence, &gap); err != nil {
			return count, examples, err
		}
		if gap == nil || *gap == 1 {
			continue
		}
		count++
		if len(examples) < maxAnomalyExamples {
			examples = append(examples, fmt.Sprintf("%s\t%s\t%d\t%d", installID, sessionID, sequence, *gap))
		}
	}
	return count, examples, rows.Err()
}
