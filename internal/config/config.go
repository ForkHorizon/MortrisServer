// Package config reads runtime configuration from the environment. No
// config file parser — env vars are enough for the knobs Phase S1 needs,
// per section 13.2 ("secrets are mounted/provided by deployment secret
// storage").
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ListenAddr string

	// MigratorDSN is used only by the `migrate` subcommand — schema
	// changes, never by the running service (section 8.1).
	MigratorDSN string

	// WriterDSN is used by `serve` (SDK endpoints, admin auth) and the
	// admin CLI subcommands (export-events, parity-report, create-admin).
	WriterDSN      string
	WriterMaxConns int32

	// ReaderDSN backs the dashboard analytics query pool (section 8.1,
	// 10.1) — a separate, smaller, read-only pool so a runaway query
	// competes with other dashboard reads, never with ingestion. Falls
	// back to WriterDSN if unset, so local dev doesn't need two DSNs.
	ReaderDSN      string
	ReaderMaxConns int32

	// DiskPath is the filesystem whose free space is evaluated for the
	// disk-pressure states in section 12.
	DiskPath string

	// SDKTest enables deliberate client fault simulation. It is accepted
	// only by a dedicated staging deployment for one test project.
	SDKTest SDKTestConfig
}

type SDKTestConfig struct {
	Enabled    bool
	Deployment string
	ProjectID  string
	Token      string
}

func Load() Config {
	cfg := Config{
		ListenAddr:     envOr("LISTEN_ADDR", ":8080"),
		MigratorDSN:    os.Getenv("MORTRIS_MIGRATOR_DSN"),
		WriterDSN:      os.Getenv("MORTRIS_WRITER_DSN"),
		WriterMaxConns: int32(envInt("MORTRIS_WRITER_MAX_CONNS", 20)),
		ReaderDSN:      os.Getenv("MORTRIS_READER_DSN"),
		ReaderMaxConns: int32(envInt("MORTRIS_READER_MAX_CONNS", 10)),
		DiskPath:       envOr("MORTRIS_DISK_PATH", "/"),
		SDKTest: SDKTestConfig{
			Enabled:    os.Getenv("MORTRIS_SDK_TEST_MODE") == "1",
			Deployment: os.Getenv("MORTRIS_DEPLOYMENT"),
			ProjectID:  os.Getenv("MORTRIS_SDK_TEST_PROJECT"),
			Token:      os.Getenv("MORTRIS_SDK_TEST_TOKEN"),
		},
	}
	if cfg.ReaderDSN == "" {
		cfg.ReaderDSN = cfg.WriterDSN
	}
	return cfg
}

func (c Config) ValidateSDKTest() error {
	if !c.SDKTest.Enabled {
		return nil
	}
	if c.SDKTest.Deployment != "staging" {
		return fmt.Errorf("MORTRIS_SDK_TEST_MODE requires MORTRIS_DEPLOYMENT=staging")
	}
	if c.SDKTest.ProjectID == "" {
		return fmt.Errorf("MORTRIS_SDK_TEST_PROJECT is required when SDK test mode is enabled")
	}
	if len(c.SDKTest.Token) < 16 {
		return fmt.Errorf("MORTRIS_SDK_TEST_TOKEN must be at least 16 bytes")
	}
	return nil
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
