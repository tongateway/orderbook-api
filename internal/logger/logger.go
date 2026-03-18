package logger

import (
	"api/internal/config"
	"log/slog"
	"os"
)

func InitLogger(cfg *config.Config) {
	level := slog.LevelDebug
	if cfg.Network.Mainnet {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}
