package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

// runImportPuzzleGeometry attaches block shapes to an already-imported
// content revision from the command line.
//
// The HTTP route needs an interactive dashboard session, which a deploy
// step does not have. This is the same code path — analytics.ApplyPuzzleGeometry
// — reached without a browser, so the validation and the all-or-nothing
// behaviour are identical.
func runImportPuzzleGeometry(ctx context.Context, cfg config.Config, args []string) error {
	flags := flag.NewFlagSet("import-puzzle-geometry", flag.ExitOnError)
	project := flags.String("project", "", "project ID (required)")
	path := flags.String("file", "", "geometry JSON produced by tools/puzzle_geometry_export.py (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *project == "" || *path == "" {
		return fmt.Errorf("--project and --file are required")
	}
	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	raw, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var geometry analytics.PuzzleGeometry
	if err := json.Unmarshal(raw, &geometry); err != nil {
		return fmt.Errorf("parse %s: %w", *path, err)
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	updated, err := analytics.ApplyPuzzleGeometry(ctx, pool, *project, geometry)
	if err != nil {
		return err
	}
	fmt.Printf("attached geometry to %d blocks on revision %s\n", updated, geometry.ContentRevision)
	return nil
}
