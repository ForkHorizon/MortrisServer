package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type smokeFlags struct {
	projectID   string
	buildNumber string
	fromStr     string
	toStr       string
	jsonOutput  bool
	runFixture  bool
}

func parseSmokeFlags(args []string) (*smokeFlags, error) {
	f := &smokeFlags{}
	fs := flag.NewFlagSet("smoke-check", flag.ContinueOnError)
	fs.StringVar(&f.projectID, "project", "puzzle", "Target project ID")
	fs.StringVar(&f.projectID, "p", "puzzle", "Alias for -project")
	fs.StringVar(&f.buildNumber, "build", "", "Candidate build number to verify")
	fs.StringVar(&f.buildNumber, "b", "", "Alias for -build")
	fs.StringVar(&f.fromStr, "from", "", "Start of evaluation window (RFC3339)")
	fs.StringVar(&f.fromStr, "f", "", "Alias for -from")
	fs.StringVar(&f.toStr, "to", "", "End of evaluation window (RFC3339)")
	fs.StringVar(&f.toStr, "t", "", "Alias for -to")
	fs.BoolVar(&f.jsonOutput, "json", false, "Output results as JSON")
	fs.BoolVar(&f.runFixture, "run-fixture", false, "Seed schema-v2 master fixture and verify")
	fs.BoolVar(&f.runFixture, "fixture", false, "Alias for -run-fixture")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if !f.runFixture && f.buildNumber == "" {
		return nil, fmt.Errorf("-build is required (or specify -run-fixture to test against master fixture)")
	}
	return f, nil
}

func runSmokeCheck(ctx context.Context, cfg config.Config, args []string) error {
	fl, err := parseSmokeFlags(args)
	if err != nil {
		return err
	}
	pool, err := initSmokePool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	if fl.runFixture {
		return executeFixtureSmokeCheck(ctx, pool, fl.jsonOutput)
	}

	from, to, err := parseTimeRange(fl.fromStr, fl.toStr)
	if err != nil {
		return err
	}

	report, err := analytics.EvaluateSmokeCheck(ctx, pool, analytics.SmokeEvalParams{
		ProjectID:   fl.projectID,
		BuildNumber: fl.buildNumber,
		From:        from,
		To:          to,
	})
	if err != nil {
		return fmt.Errorf("smoke check evaluation: %w", err)
	}

	renderSmokeReport(report, fl.jsonOutput)
	if !report.Passed {
		return fmt.Errorf("smoke check failed on candidate build %s", fl.buildNumber)
	}
	return nil
}

func initSmokePool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	dsn := cfg.WriterDSN
	if dsn == "" {
		dsn = cfg.ReaderDSN
	}
	if dsn == "" {
		dsn = os.Getenv("MORTRIS_TEST_DSN")
	}
	if dsn == "" {
		return nil, fmt.Errorf("MORTRIS_WRITER_DSN or MORTRIS_READER_DSN is required")
	}
	pool, err := store.NewPool(ctx, dsn, 2)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return pool, nil
}

func parseTimeRange(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	to := now.Add(2 * time.Minute)
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse -to: %w", err)
		}
		to = parsed
	}
	from := to.Add(-24 * time.Hour)
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return from, to, fmt.Errorf("parse -from: %w", err)
		}
		from = parsed
	}
	if !from.Before(to) {
		return from, to, fmt.Errorf("-from (%s) must be before -to (%s)", from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	return from, to, nil
}

func executeFixtureSmokeCheck(ctx context.Context, pool *pgxpool.Pool, jsonOutput bool) error {
	now := time.Now().UTC().Truncate(time.Second)
	testProject := fmt.Sprintf("smoke-test-%d", now.UnixNano())
	defer analytics.CleanProjectData(context.Background(), pool, testProject)

	build := "smoke-build-v2"
	fix := analytics.NewMasterFixture(testProject, "2.0.0", build, now)

	if err := fix.Seed(ctx, pool); err != nil {
		return fmt.Errorf("seed fixture: %w", err)
	}

	report, err := analytics.EvaluateSmokeCheck(ctx, pool, analytics.SmokeEvalParams{
		ProjectID:   testProject,
		BuildNumber: build,
		From:        now.Add(-time.Hour),
		To:          now.Add(2 * time.Hour),
	})
	if err != nil {
		return err
	}

	renderSmokeReport(report, jsonOutput)
	if !report.Passed {
		return fmt.Errorf("smoke check failed on candidate build %s", build)
	}
	return nil
}

func renderSmokeReport(r *analytics.SmokeCheckReport, jsonOutput bool) {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}

	fmt.Println("================================================================================")
	fmt.Println("               PUZZLE SCHEMA-V2 RELEASE SMOKE CHECK REPORT                      ")
	fmt.Println("================================================================================")
	fmt.Printf("Project:     %s\n", r.ProjectID)
	fmt.Printf("Build:       %s\n", r.BuildNumber)
	fmt.Printf("App Version: %s\n", r.AppVersion)
	fmt.Printf("Time Window: %s to %s\n", r.From.Format(time.RFC3339), r.To.Format(time.RFC3339))
	fmt.Printf("Result:      ")
	if r.Passed {
		fmt.Println("[PASS] GREEN - Candidate build is APPROVED for tester release")
	} else {
		fmt.Println("[FAIL] RED - Candidate build is REJECTED")
	}
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("13-Point Android Smoke Checklist Verification:")

	for _, item := range r.Items {
		status := "[PASS]"
		if !item.Passed {
			status = "[FAIL]"
		}
		fmt.Printf("  %2d. %-6s %-32s (%s)\n", item.Index, status, item.Name, item.Detail)
	}

	fmt.Println("================================================================================")
	fmt.Printf("Summary: %s\n", r.Summary)
}
