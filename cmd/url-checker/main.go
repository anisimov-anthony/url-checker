package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"url-checker/internal/database"
	"url-checker/internal/handlers"
	"url-checker/internal/service"

	"github.com/sirupsen/logrus"
)

func main() {
	// logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// DB
	db, err := database.NewDatabase("./url-checker.db")
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// HTTP Client
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// URLChecker
	checker := service.NewURLChecker(db, logger, httpClient)

	if err := checker.LoadBatches(); err != nil {
		logger.Fatalf("Failed to load batches from database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go checker.StartWorker(ctx)

	handler := handlers.NewHandler(checker, logger)
	router := handler.SetupRoutes()

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		logger.Info("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	gracefulShutdown(server, checker, ctx, cancel, logger)
}

func gracefulShutdown(server *http.Server, checker *service.URLChecker, ctx context.Context, cancel context.CancelFunc, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutdown signal received, starting graceful shutdown...")

	checker.SetShutdown(true)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	}

	cancel()

	logger.Info("Graceful shutdown completed")
}
