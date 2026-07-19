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

type exportOptions struct {
	project, output, operator string
	from, to                  time.Time
}

type exportedEvent struct {
	projectID, eventID, installID, sessionID                  string
	sequence, sessionElapsedMs                                int64
	name, eventKind                                           string
	occurredAtClient, sentAtClient, receivedAt, effectiveAt   time.Time
	clockSkewMs                                               int64
	timeQuality, appVersion, buildNumber, platform, osVersion string
	deviceClass, locale                                       string
	timezoneOffset                                            int
	properties                                                []byte
}

// runExportEvents implements the admin-only, local-CLI-only export
// command (section 11). Streams rows with bounded memory over a
// read-only transaction; never writes exported contents to the audit
// log, only the operator/parameters/row-count/status (section 11).
func runExportEvents(ctx context.Context, cfg config.Config, args []string) error {
	opts, err := parseExportOptions(args)
	if err != nil {
		return err
	}
	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	f, err := os.Create(opts.output)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	rowCount, exportErr := exportEventsCSV(ctx, pool, opts.project, opts.from, opts.to, f)
	auditExport(ctx, pool, opts, rowCount, exportErr)
	if exportErr != nil {
		return exportErr
	}
	fmt.Printf("exported %d rows to %s\n", rowCount, opts.output)
	return nil
}

func parseExportOptions(args []string) (exportOptions, error) {
	fs := flag.NewFlagSet("export-events", flag.ExitOnError)
	project := fs.String("project", "", "project ID (required)")
	from := fs.String("from", "", "RFC3339 start of range, inclusive (required)")
	to := fs.String("to", "", "RFC3339 end of range, exclusive (required)")
	output := fs.String("output", "", "output CSV file path (required)")
	operator := fs.String("operator", "", "identifies who ran this export, recorded in the audit log (required)")
	if err := fs.Parse(args); err != nil {
		return exportOptions{}, err
	}
	if *project == "" || *from == "" || *to == "" || *output == "" || *operator == "" {
		return exportOptions{}, fmt.Errorf("--project, --from, --to, --output, and --operator are all required")
	}
	fromT, err := time.Parse(time.RFC3339, *from)
	if err != nil {
		return exportOptions{}, fmt.Errorf("--from: %w", err)
	}
	toT, err := time.Parse(time.RFC3339, *to)
	if err != nil {
		return exportOptions{}, fmt.Errorf("--to: %w", err)
	}
	if !toT.After(fromT) {
		return exportOptions{}, fmt.Errorf("--to must be after --from")
	}
	if toT.Sub(fromT) > maxExportRange {
		return exportOptions{}, fmt.Errorf("date range cannot exceed %s", maxExportRange)
	}
	return exportOptions{project: *project, output: *output, operator: *operator, from: fromT, to: toT}, nil
}

func auditExport(ctx context.Context, pool *pgxpool.Pool, opts exportOptions, rowCount int, exportErr error) {
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
	`, opts.operator, opts.project, opts.from.Format(time.RFC3339), opts.to.Format(time.RFC3339), rowCount, status, errMsg); auditErr != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to record audit log entry:", auditErr)
	}
}

func exportEventsCSV(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, w io.Writer) (int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := exportRows(ctx, tx, projectID, from, to)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	cw := csv.NewWriter(w)
	if err := cw.Write(eventColumns); err != nil {
		return 0, err
	}

	count, err := writeExportRows(rows, cw)
	if err != nil {
		return count, err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return count, err
	}
	return count, tx.Commit(ctx)
}

func exportRows(ctx context.Context, tx pgx.Tx, projectID string, from, to time.Time) (pgx.Rows, error) {
	return tx.Query(ctx, `
		SELECT project_id, event_id, install_id, session_id, sequence, session_elapsed_ms,
		       name, event_kind, occurred_at_client, sent_at_client, received_at, effective_at,
		       clock_skew_ms, time_quality, app_version, build_number, platform, os_version,
		       device_class, locale, timezone_offset_minutes, properties
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		ORDER BY effective_at
	`, projectID, from, to)
}

func writeExportRows(rows pgx.Rows, cw *csv.Writer) (int, error) {
	count := 0
	for rows.Next() {
		event, err := scanExportEvent(rows)
		if err != nil {
			return count, err
		}
		if err := cw.Write(event.csvRecord()); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func scanExportEvent(rows pgx.Rows) (exportedEvent, error) {
	var event exportedEvent
	err := rows.Scan(&event.projectID, &event.eventID, &event.installID, &event.sessionID, &event.sequence, &event.sessionElapsedMs,
		&event.name, &event.eventKind, &event.occurredAtClient, &event.sentAtClient, &event.receivedAt, &event.effectiveAt,
		&event.clockSkewMs, &event.timeQuality, &event.appVersion, &event.buildNumber, &event.platform, &event.osVersion,
		&event.deviceClass, &event.locale, &event.timezoneOffset, &event.properties)
	return event, err
}

func (event exportedEvent) csvRecord() []string {
	record := []string{
		event.projectID, event.eventID, event.installID, event.sessionID,
		strconv.FormatInt(event.sequence, 10), strconv.FormatInt(event.sessionElapsedMs, 10), event.name, event.eventKind,
		event.occurredAtClient.UTC().Format(exportTimestampLayout), event.sentAtClient.UTC().Format(exportTimestampLayout),
		event.receivedAt.UTC().Format(exportTimestampLayout), event.effectiveAt.UTC().Format(exportTimestampLayout),
		strconv.FormatInt(event.clockSkewMs, 10), event.timeQuality, event.appVersion, event.buildNumber,
		event.platform, event.osVersion, event.deviceClass, event.locale, strconv.Itoa(event.timezoneOffset), string(event.properties),
	}
	for i, value := range record {
		record[i] = neutralizeCSVCell(value)
	}
	return record
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
