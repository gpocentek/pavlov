package config

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"
)

func withAppArgs(t *testing.T, args ...string) {
	t.Helper()
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	allArgs := append([]string{"pavlov"}, args...)
	os.Args = allArgs
	flag.CommandLine = flag.NewFlagSet(allArgs[0], flag.ContinueOnError)

	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})
}

func parseAppConfigOK(t *testing.T) *AppConfig {
	t.Helper()
	cfg, err := ParseAppConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
}

func TestParseAppConfigDefaults(t *testing.T) {
	withAppArgs(t)

	cfg := parseAppConfigOK(t)

	if cfg.ConfigFile != "/etc/pavlov/config.yaml" {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, "/etc/pavlov/config.yaml")
	}
	if cfg.CheckConfig {
		t.Fatal("CheckConfig = true, want false")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}

func TestParseAppConfigFlags(t *testing.T) {
	withAppArgs(t,
		"-config", "/custom/config.yaml",
		"-check-config",
		"-shutdown-timeout", "30s",
		"-log-level", "debug",
		"-log-format", "text",
	)

	cfg := parseAppConfigOK(t)

	if cfg.ConfigFile != "/custom/config.yaml" {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, "/custom/config.yaml")
	}
	if !cfg.CheckConfig {
		t.Fatal("CheckConfig = false, want true")
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "text" {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
}

func TestParseAppConfigLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{name: "debug", level: "debug"},
		{name: "info", level: "info"},
		{name: "warn", level: "warn"},
		{name: "error", level: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAppArgs(t, "-log-level", tt.level)

			cfg := parseAppConfigOK(t)
			if cfg.LogLevel != tt.level {
				t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, tt.level)
			}
		})
	}
}

func TestParseAppConfigLogFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{name: "json", format: "json"},
		{name: "text", format: "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAppArgs(t, "-log-format", tt.format)

			cfg := parseAppConfigOK(t)
			if cfg.LogFormat != tt.format {
				t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, tt.format)
			}
		})
	}
}

func TestParseAppConfigInvalidLogLevel(t *testing.T) {
	withAppArgs(t, "-log-level", "trace")

	_, err := ParseAppConfig()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `must be "debug", "info", "warn", or "error"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAppConfigInvalidLogFormat(t *testing.T) {
	withAppArgs(t, "-log-format", "xml")

	_, err := ParseAppConfig()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `must be "json" or "text"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
