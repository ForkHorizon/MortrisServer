package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

type artManifest struct {
	SchemaVersion   int `json:"schema_version"`
	ContentRevision string
	Houses          []artManifestHouse `json:"houses"`
}

type artManifestHouse struct {
	CityID    int    `json:"city_id"`
	HouseID   int    `json:"house_id"`
	File      string `json:"file"`
	MediaType string `json:"media_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// runImportPuzzleArt loads a directory produced by
// tools/puzzle_art_export.py. Images are read from disk beside the
// manifest rather than inlined, so the payload never has to be
// base64-expanded to move through a JSON document.
func runImportPuzzleArt(ctx context.Context, cfg config.Config, args []string) error {
	flags := flag.NewFlagSet("import-puzzle-art", flag.ExitOnError)
	project := flags.String("project", "", "project ID (required)")
	dir := flags.String("dir", "", "directory containing manifest.json and the images (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *project == "" || *dir == "" {
		return fmt.Errorf("--project and --dir are required")
	}
	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	manifest, houses, err := readArtManifest(*dir)
	if err != nil {
		return err
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	count, err := analytics.ImportPuzzleHouseArt(ctx, pool, *project, manifest.ContentRevision, houses)
	if err != nil {
		return err
	}
	fmt.Printf("stored art for %d houses on revision %s\n", count, manifest.ContentRevision)
	return nil
}

func readArtManifest(dir string) (artManifest, []analytics.PuzzleHouseArt, error) {
	var manifest artManifest
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest, nil, err
	}
	// ContentRevision is read via an alias so the manifest key stays
	// snake_case like every other payload in this repo.
	var envelope struct {
		artManifest
		ContentRevision string `json:"content_revision"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return manifest, nil, fmt.Errorf("parse manifest: %w", err)
	}
	manifest = envelope.artManifest
	manifest.ContentRevision = envelope.ContentRevision
	houses := make([]analytics.PuzzleHouseArt, 0, len(manifest.Houses))
	for _, entry := range manifest.Houses {
		// filepath.Base keeps a manifest from reaching outside its own
		// directory via a crafted "file" value.
		image, err := os.ReadFile(filepath.Join(dir, filepath.Base(entry.File)))
		if err != nil {
			return manifest, nil, err
		}
		houses = append(houses, analytics.PuzzleHouseArt{
			CityID: entry.CityID, HouseID: entry.HouseID, MediaType: entry.MediaType,
			Width: entry.Width, Height: entry.Height, Image: image,
		})
	}
	return manifest, houses, nil
}
