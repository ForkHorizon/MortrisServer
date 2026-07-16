// Package config reads runtime configuration from the environment. No
// config file parser — env vars are enough for the knobs Phase S1 needs,
// per section 13.2 ("secrets are mounted/provided by deployment secret
// storage").
package config

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr string

	// MigratorDSN is used only by the `migrate` subcommand — schema
	// changes, never by the running service (section 8.1).
	MigratorDSN string

	// WriterDSN is used by `serve` and the admin CLI subcommands
	// (export-events, parity-report). A dedicated analytics_reader pool
	// is added in Phase S2 once the dashboard query API exists.
	WriterDSN      string
	WriterMaxConns int32

	// DiskPath is the filesystem whose free space is evaluated for the
	// disk-pressure states in section 12.
	DiskPath string
}

func Load() Config {
	return Config{
		ListenAddr:     envOr("LISTEN_ADDR", ":8080"),
		MigratorDSN:    os.Getenv("MORTRIS_MIGRATOR_DSN"),
		WriterDSN:      os.Getenv("MORTRIS_WRITER_DSN"),
		WriterMaxConns: int32(envInt("MORTRIS_WRITER_MAX_CONNS", 20)),
		DiskPath:       envOr("MORTRIS_DISK_PATH", "/"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
