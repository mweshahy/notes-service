package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/notes/internal/service"
	"example.com/notes/internal/storage"
	httpadapter "example.com/notes/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("notes API stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Initialize structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 2. Wire dependencies (Dependency Injection)
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	repo, err := storage.NewSQLiteRepository(startupCtx)
	startupCancel()
	if err != nil {
		logger.Error("failed to initialize SQLite repository", "error", err)
		return err
	}
	defer func() {
		if err := repo.Close(); err != nil {
			logger.Error("failed to close SQLite repository", "error", err)
		}
	}()
	//repo := storage.NewMemoryRepository()
	noteService := service.NewNoteService(repo)
	handler := httpadapter.NewHandler(noteService, logger)

	// 3. Setup router (Go 1.22+ enhanced routing syntax)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/notes", handler.HandleCreateNote)
	mux.HandleFunc("GET /api/v1/notes", handler.HandleListNotes)
	mux.HandleFunc("GET /api/v1/notes/{id}", handler.HandleGetNote)
	mux.HandleFunc("PUT /api/v1/notes/{id}", handler.HandleUpdateNote)
	mux.HandleFunc("DELETE /api/v1/notes/{id}", handler.HandleDeleteNote)

	// Apply middleware stack
	loggedHandler := httpadapter.RequestLogger(logger)(mux)

	// 4. Configure HTTP Server with robust timeouts to prevent slowloris attacks
	port := os.Getenv("NOTES_SERVICE_PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      loggedHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 5. Start server in a background goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTP server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// 6. Setup channel to receive OS signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		logger.Info("Shutdown signal received, draining active connections...", "signal", sig.String())
	case err := <-serverErrors:
		return err
	}

	// 7. Create a context with a timeout for graceful drain
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	logger.Info("Server stopped gracefully. Goodbye!")
	return nil
}
