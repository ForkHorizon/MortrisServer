package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ForkHorizon/Mortris/internal/codegen"
)

// runGenEvents implements the `gen-events` subcommand (Phase 4: catalog ->
// C# codegen): it's a pure file transform, no DB connection needed, so it
// deliberately doesn't take a context.Context/config.Config like the
// other subcommands.
func runGenEvents(args []string) error {
	fs := flag.NewFlagSet("gen-events", flag.ContinueOnError)
	in := fs.String("in", "events/catalog.yaml", "path to the event catalog YAML")
	out := fs.String("out", "sdk/csharp/MortrisEvents.g.cs", "path to write the generated C# file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	catalog, err := codegen.LoadCatalog(*in)
	if err != nil {
		return fmt.Errorf("load catalog %s: %w", *in, err)
	}
	source := codegen.GenerateCSharp(catalog)

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(*out, []byte(source), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *out, err)
	}
	fmt.Printf("wrote %s (%d events)\n", *out, len(catalog.Events))
	return nil
}
