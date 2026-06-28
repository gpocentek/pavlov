package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pavlov/internal/config"
	"pavlov/internal/engine"
	"pavlov/internal/logger"
)

func main() {
	configFile := flag.String("config", "/etc/pavlov/config.yaml", "path to config file")
	checkConfig := flag.Bool("check-config", false, "check config file and exit")
	shutdownTimeout := flag.Duration("shutdown-timeout", 10*time.Second, "max time to wait for graceful shutdown")
	flag.Parse()

	logger.Init()

	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		slog.Error("failed to load config", "file", *configFile, "err", err)
		os.Exit(1)
	}

	if *checkConfig {
		fmt.Println("config is valid")
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	e, err := engine.NewEngine(cfg)
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

	timeout := *shutdownTimeout
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
