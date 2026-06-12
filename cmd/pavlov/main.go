package main

import (
	"context"
	"flag"
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
	configFile := flag.String("config", "/etc/pavlov/config.yaml", "path to config file")
	checkConfig := flag.Bool("check-config", false, "check config file and exit")
	flag.Parse()

	logger.Init()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		slog.Error("failed to load config", "file", *configFile, "err", err)
		os.Exit(1)
	}

	if *checkConfig {
		fmt.Println("config is valid")
		os.Exit(0)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	e, err := engine.NewEngine(cfg)
	if err != nil {
		slog.Error("failed to create engine", "err", err)
		os.Exit(1)
	}
	go e.Run(ctx)

	switch <-sig {
	case syscall.SIGTERM:
		slog.Info("received SIGTERM, shutting down")
	case syscall.SIGINT:
		slog.Info("received SIGINT, shutting down")
	}

	cancel()
	slog.Info("shutdown complete")
}
