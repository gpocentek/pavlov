package logger

import (
	"log/slog"
	"os"
	"pavlov/internal/config"
	"strings"
)

func Init(cfg *config.AppConfig) {
	var handler slog.Handler
	level := parseLevel(cfg.LogLevel)
	switch cfg.LogFormat {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	default:
		slog.Error("invalid log format", "format", cfg.LogFormat)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(handler))
	slog.Info("logging initialized", "level", level.String(), "format", cfg.LogFormat)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
