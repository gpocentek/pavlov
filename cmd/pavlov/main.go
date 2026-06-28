package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"pavlov/internal/config"
	"pavlov/internal/engine"
	"pavlov/internal/logger"
)

func main() {
	cfg, err := config.ParseAppConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	logger.Init(cfg)

	slog.Debug("parsed app config", "config", cfg.String())

	rulesCfg, err := config.LoadFromFile(cfg.ConfigFile)
	if err != nil {
		slog.Error("failed to load config", "file", cfg.ConfigFile, "err", err)
		os.Exit(1)
	}

	if cfg.CheckConfig {
		fmt.Println("config is valid")
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	e, err := engine.NewEngine(rulesCfg)
	if err != nil {
		slog.Error("failed to create engine", "err", err)
		os.Exit(1)
	}

	done := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(done)
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	timeout := cfg.ShutdownTimeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()
	select {
	case <-done:
		slog.Info("shutdown complete")
	case <-shutdownCtx.Done():
		slog.Error(
			"graceful shutdown timed out, forcing exit",
			"timeout", timeout,
			"component", "engine",
		)
		os.Exit(1)
	}
}
