package main

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	PostgresDSN   string
	MemgraphHost  string
	MemgraphPort  int
	HTTPPort      int
	LogLevel      slog.Level
}

func LoadConfig() Config {
	cfg := Config{
		PostgresDSN:  getEnv("CANOPY_POSTGRES_DSN", "postgres://canopy:canopy_dev@localhost:5432/canopy?sslmode=disable"),
		MemgraphHost: getEnv("CANOPY_MEMGRAPH_HOST", "localhost"),
		MemgraphPort: getEnvInt("CANOPY_MEMGRAPH_PORT", 7687),
		HTTPPort:     getEnvInt("CANOPY_HTTP_PORT", 8080),
		LogLevel:     parseLogLevel(getEnv("CANOPY_LOG_LEVEL", "info")),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch s {
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
