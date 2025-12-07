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

	if err := checker.LoadBatches(context.Background()); err != nil {
		logger.Fatalf("Failed to load batches from database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go checker.StartWorker(ctx)

	// Routers
	handler := handlers.NewHandler(checker, logger)
	router := handler.SetupRoutes()

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start
	go func() {
		logger.Info("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Shutdown
	gracefulShutdown(server, checker, 30*time.Second, logger)
}

func gracefulShutdown(server *http.Server, checker *service.URLChecker, shutdownTimeout time.Duration, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutdown signal received, starting graceful shutdown...")

	checker.SetShutdown(true)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	}

	logger.Info("Graceful shutdown completed")
}
