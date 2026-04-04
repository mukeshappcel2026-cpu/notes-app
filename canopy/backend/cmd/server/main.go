package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canopy-ai/canopy/internal/api"
	"github.com/canopy-ai/canopy/internal/changes"
	"github.com/canopy-ai/canopy/internal/graph"
	"github.com/canopy-ai/canopy/internal/ingestion"
	"github.com/canopy-ai/canopy/internal/storage"
)

func main() {
	cfg := LoadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to Postgres
	pg, err := storage.NewPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()
	slog.Info("connected to postgres")

	// Connect to Memgraph
	mg, err := graph.NewMemgraph(cfg.MemgraphHost, cfg.MemgraphPort)
	if err != nil {
		slog.Error("failed to connect to memgraph", "error", err)
		os.Exit(1)
	}
	defer mg.Close()
	slog.Info("connected to memgraph")

	// Initialize services
	ingestor := ingestion.NewService(pg, mg)
	changeDetector := changes.NewDetector(pg, mg)
	_ = changeDetector // used by ingestion pipeline

	// Wire up change detection into ingestion pipeline
	ingestor.OnLLMCall(changeDetector.CheckForChanges)

	// Set up HTTP server
	router := api.NewRouter(pg, mg, ingestor)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		server.Shutdown(shutdownCtx)
		cancel()
	}()

	slog.Info("starting canopy server", "port", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
