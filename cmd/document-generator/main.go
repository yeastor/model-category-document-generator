package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"model-category-document-generator/internal/api/httpapi"
	"model-category-document-generator/internal/config"
	"model-category-document-generator/internal/moderation"
	filerepo "model-category-document-generator/internal/repository/file"
	"model-category-document-generator/internal/usecase"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.GetLogLevel()}))
	repository := filerepo.NewRepository(cfg)
	moderationService := moderation.NewService(cfg.ModerationDir)
	generator := usecase.NewGenerator(cfg, repository, moderationService)
	api := httpapi.NewServer(cfg, generator, logger)

	server := &http.Server{
		Addr:              cfg.Address(),
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("document generator started", "address", cfg.Address())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("document generator stopped")
}
