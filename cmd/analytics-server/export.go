package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

const exportTimestampLayout = "2006-01-02T15:04:05.000Z"
const maxExportRange = 90 * 24 * time.Hour

var eventColumns = []string{
	"project_id", "event_id", "install_id", "session_id", "sequence", "session_elapsed_ms",
	"name", "event_kind", "occurred_at_client", "sent_at_client", "received_at", "effective_at",
	"clock_skew_ms", "time_quality", "app_version", "build_number", "platform", "os_version",
	"device_class", "locale", "timezone_offset_minutes", "properties",
}

// runExportEvents implements the admin-only, local-CLI-only export
// command (section 11). Streams rows with bounded memory over a
// read-only transaction; never writes exported contents to the audit
// log, only the operator/parameters/row-count/status (section 11).
func runExportEvents(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("export-events", flag.ExitOnError)
	project := fs.String("project", "", "project ID (required)")
	from := fs.String("from", "", "RFC3339 start of range, inclusive (required)")
	to := fs.String("to", "", "RFC3339 end of range, exclusive (required)")
	output := fs.String("output", "", "output CSV file path (required)")
	operator := fs.String("operator", "", "identifies who ran this export, recorded in the audit log (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *from == "" || *to == "" || *output == "" || *operator == "" {
		return fmt.Errorf("--project, --from, --to, --output, and --operator are all required")
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
	if toT.Sub(fromT) > maxExportRange {
		return fmt.Errorf("date range cannot exceed %s", maxExportRange)
	}

	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()

	f, err := os.Create(*output)
	if err != nil {
		return err
	}
	defer f.Close()

	rowCount, exportErr := exportEventsCSV(ctx, pool, *project, fromT, toT, f)

	status, errMsg := "success", ""
	if exportErr != nil {
		status, errMsg = "failed", exportErr.Error()
	}
	if _, auditErr := pool.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES (NULL, 'export_events', jsonb_build_object(
			'operator', $1::text, 'project_id', $2::text, 'from', $3::text, 'to', $4::text,
			'row_count', $5::int, 'status', $6::text, 'error', $7::text
		))
	`, *operator, *project, fromT.Format(time.RFC3339), toT.Format(time.RFC3339), rowCount, status, errMsg); auditErr != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to record audit log entry:", auditErr)
	}

	if exportErr != nil {
		return exportErr
	}
	fmt.Printf("exported %d rows to %s\n", rowCount, *output)
	return nil
}

func exportEventsCSV(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, w io.Writer) (int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT project_id, event_id, install_id, session_id, sequence, session_elapsed_ms,
		       name, event_kind, occurred_at_client, sent_at_client, received_at, effective_at,
		       clock_skew_ms, time_quality, app_version, build_number, platform, os_version,
		       device_class, locale, timezone_offset_minutes, properties
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		ORDER BY effective_at
	`, projectID, from, to)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	cw := csv.NewWriter(w)
	if err := cw.Write(eventColumns); err != nil {
		return 0, err
	}

	count := 0
	for rows.Next() {
		var (
			rProjectID, rEventID, rInstallID, rSessionID              string
			sequence, sessionElapsedMs                                int64
			name, eventKind                                           string
			occurredAtClient, sentAtClient, receivedAt, effectiveAt   time.Time
			clockSkewMs                                               int64
			timeQuality, appVersion, buildNumber, platform, osVersion string
			deviceClass, locale                                       string
			tzOffset                                                  int
			properties                                                []byte
		)
		if err := rows.Scan(&rProjectID, &rEventID, &rInstallID, &rSessionID, &sequence, &sessionElapsedMs,
			&name, &eventKind, &occurredAtClient, &sentAtClient, &receivedAt, &effectiveAt,
			&clockSkewMs, &timeQuality, &appVersion, &buildNumber, &platform, &osVersion,
			&deviceClass, &locale, &tzOffset, &properties); err != nil {
			return count, err
		}

		record := []string{
			rProjectID, rEventID, rInstallID, rSessionID,
			strconv.FormatInt(sequence, 10), strconv.FormatInt(sessionElapsedMs, 10),
			name, eventKind,
			occurredAtClient.UTC().Format(exportTimestampLayout), sentAtClient.UTC().Format(exportTimestampLayout),
			receivedAt.UTC().Format(exportTimestampLayout), effectiveAt.UTC().Format(exportTimestampLayout),
			strconv.FormatInt(clockSkewMs, 10), timeQuality,
			appVersion, buildNumber, platform, osVersion, deviceClass, locale,
			strconv.Itoa(tzOffset), string(properties),
		}
		for i, v := range record {
			record[i] = neutralizeCSVCell(v)
		}
		if err := cw.Write(record); err != nil {
			return count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return count, err
	}
	return count, tx.Commit(ctx)
}

// neutralizeCSVCell defends against spreadsheet formula injection
// (section 11): a cell beginning with =, +, -, or @ is interpreted as a
// formula by Excel/Sheets when the CSV is opened, regardless of the
// column's semantic type — a negative clock_skew_ms is exactly as
// dangerous here as a crafted property string.
func neutralizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
