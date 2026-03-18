// Package main is the entry point for the neteye-agent.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/neteye/agent/internal/client"
	"github.com/neteye/agent/internal/config"
)

func main() {
	cfgPath := flag.String("config", "", "path to YAML config file (optional)")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	logFormat := flag.String("log-format", "text", "log format: text, json")
	flag.Parse()

	logger := buildLogger(*logLevel, *logFormat)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	logger.Info("starting",
		"hostname", cfg.Hostname,
		"os", config.OS(),
		"arch", config.Arch(),
		"center", cfg.CenterURL,
		"interval", cfg.CollectInterval)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := client.New(cfg, logger.With("component", "client"))
	c.Run(ctx)

	logger.Info("stopped")
}

func buildLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
