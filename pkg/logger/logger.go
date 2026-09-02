package logger

import (
	"log/slog"
	"os"
)

func New(service, env string) *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler
	if env == "local" {
		handler = slog.NewTextHandler(os.Stdout, options)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, options)
	}

	return slog.New(handler).With("service", service)
}
