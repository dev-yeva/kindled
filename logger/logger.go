package logger

import (
	"kindled/config"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

func New(cfg *config.Logger) *slog.Logger {
	var incorrectLevel, incorrectFormat bool
	var level slog.Level

	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
		incorrectLevel = true
	}

	var h slog.Handler
	switch cfg.Format {
	case "json":
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: level})
	case "color":
		h = tint.NewTextHandler(os.Stdout, &tint.Options{AddSource: true, Level: level})
	case "user":
		// h = tint.NewTextHandler(os.Stdout, &tint.Options{AddSource: false, Level: level, ReplaceAttr: removeLevel, TimeFormat: "15:04:05"})
		h = NewUserHandler(os.Stdout, &Options{Level: level, TimeFormat: "15:04:05"})
	default:
		h = tint.NewTextHandler(os.Stdout, &tint.Options{AddSource: true, Level: level, NoColor: true})
		if cfg.Format != "text" {
			incorrectFormat = true
		}
	}

	logger := slog.New(h)

	if incorrectLevel {
		logger.Warn("Unknown logger level in config", "level", cfg.Level)
	}

	if incorrectFormat {
		logger.Warn("Unknown logger format in config", "format", cfg.Format)
	}

	return logger
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}
