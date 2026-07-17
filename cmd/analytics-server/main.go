// Command analytics-server is the single binary described in
// server_implementation_plan.md section 4: it serves the SDK/dashboard
// HTTP API (`serve`) and the admin CLI (`export-events`, `parity-report`),
// and applies schema migrations (`migrate`).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: analytics-server <migrate|serve|export-events|parity-report|create-admin> [flags]")
		os.Exit(2)
	}

	cfg := config.Load()
	ctx := context.Background()

	var err error
	switch os.Args[1] {
	case "migrate":
		err = runMigrate(ctx, cfg)
	case "serve":
		err = runServe(ctx, cfg)
	case "export-events":
		err = runExportEvents(ctx, cfg, os.Args[2:])
	case "parity-report":
		err = runParityReport(ctx, cfg, os.Args[2:])
	case "create-admin":
		err = runCreateAdmin(ctx, cfg, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runMigrate(ctx context.Context, cfg config.Config) error {
	if cfg.MigratorDSN == "" {
		return fmt.Errorf("MORTRIS_MIGRATOR_DSN is required")
	}
	pool, err := store.NewPool(ctx, cfg.MigratorDSN, 1)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.ApplyMigrations(ctx, pool, "migrations"); err != nil {
		return err
	}
	fmt.Println("migrations applied")
	return nil
}
