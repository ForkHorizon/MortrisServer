package main

import (
	"fmt"

	"github.com/ForkHorizon/Mortris/internal/config"
)

func validateServeConfig(cfg config.Config) error {
	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	return cfg.ValidateSDKTest()
}
