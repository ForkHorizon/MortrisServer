package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/httpapi"
	"github.com/ForkHorizon/Mortris/internal/ingest"
	"github.com/ForkHorizon/Mortris/internal/maintenance"
	"github.com/ForkHorizon/Mortris/internal/store"
)

func runServe(ctx context.Context, cfg config.Config) error {
	if err := validateServeConfig(cfg); err != nil {
		return err
	}

	pool, err := store.NewPool(ctx, cfg.WriterDSN, cfg.WriterMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	readerPool, err := store.NewPool(ctx, cfg.ReaderDSN, cfg.ReaderMaxConns)
	if err != nil {
		return err
	}
	defer readerPool.Close()

	ingestSvc := ingest.NewService(pool)
	server := httpapi.NewServer(ingestSvc, pool, readerPool)
	if cfg.SDKTest.Enabled {
		server.EnableSDKTest(cfg.SDKTest.ProjectID, cfg.SDKTest.Token)
	}
	httpServer := server.NewHTTPServer(cfg.ListenAddr)

	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	diskMonitor := maintenance.NewDiskMonitor(cfg.DiskPath, 30*time.Second, server.Log)
	ingestSvc.Disk = diskMonitor
	go diskMonitor.Run(stopCtx)

	maintRunner := &maintenance.Runner{Pool: pool, Log: server.Log}
	go maintRunner.Run(stopCtx)

	errCh := make(chan error, 1)
	go func() {
		server.Log.Info("listening", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-stopCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Log.Info("shutting down")
	return httpServer.Shutdown(shutdownCtx)
}
