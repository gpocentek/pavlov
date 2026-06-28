package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type AppConfig struct {
	ConfigFile      string
	CheckConfig     bool
	ShutdownTimeout time.Duration
	LogLevel        string
	LogFormat       string
}

func (cfg *AppConfig) String() string {
	return fmt.Sprintf("ConfigFile: %s, CheckConfig: %t, ShutdownTimeout: %s, LogLevel: %s, LogFormat: %s", cfg.ConfigFile, cfg.CheckConfig, cfg.ShutdownTimeout, cfg.LogLevel, cfg.LogFormat)
}

func ParseAppConfig() (*AppConfig, error) {
	configFile := flag.String("config", "/etc/pavlov/config.yaml", "path to config file")
	checkConfig := flag.Bool("check-config", false, "check config file and exit")
	shutdownTimeout := flag.Duration("shutdown-timeout", 10*time.Second, "max time to wait for graceful shutdown")
	logLevel := "info"
	flag.Func("log-level", "log level, \"debug\", \"info\", \"warn\", or \"error\" (default: info)", func(s string) error {
		switch s {
		case "debug", "info", "warn", "error":
			logLevel = s
			return nil
		default:
			return fmt.Errorf("must be \"debug\", \"info\", \"warn\", or \"error\"")
		}
	})
	logFormat := "json"
	flag.Func("log-format", "log format, \"json\" or \"text\" (default: json)", func(s string) error {
		switch s {
		case "text", "json":
			logFormat = s
			return nil
		default:
			return fmt.Errorf("must be \"json\" or \"text\"")
		}
	})
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	return &AppConfig{
		ConfigFile:      *configFile,
		CheckConfig:     *checkConfig,
		ShutdownTimeout: *shutdownTimeout,
		LogLevel:        logLevel,
		LogFormat:       logFormat,
	}, nil
}
