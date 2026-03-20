package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/api"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/handlers"
	"github.com/milosDamjanovic17/djfingerprint/internal/config"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

func main() {
	// ── 1. Load configuration ────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// ── 2. Open database connection ──────────────────────────────────
	db, err := store.Open(cfg.DSNPostgres())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	log.Printf("connected to database")

	// ── 3. Run migrations ────────────────────────────────────────────
	if err := store.RunMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Printf("migrations applied")

	// ── 4. Build stores ──────────────────────────────────────────────
	songStore := store.NewSongStore(db)
	fingerprintStore := store.NewFingerprintStore(db)

	// ── 5. Wrap stores in adapters ───────────────────────────────────
	// Adapters translate between the store's method signatures and
	// the interfaces the handlers expect.
	songAdapter := NewSongStoreAdapter(songStore)
	fingerprintAdapter := NewFingerprintStoreAdapter(fingerprintStore)

	// ── 6. Build handler and router ──────────────────────────────────
	h := handlers.NewHandler(songAdapter, fingerprintAdapter)
	router := api.NewRouter(h)

	// ── 7. Configure HTTP server ─────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ── 8. Start server in background goroutine ──────────────────────
	// ListenAndServe blocks — run it in a goroutine so the main
	// goroutine can wait for a shutdown signal below.
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("server listening on :%s", cfg.AppPort)
		serverErrors <- srv.ListenAndServe()
	}()

	// ── 9. Block until shutdown signal or server error ───────────────
	// SIGINT  — Ctrl+C from terminal
	// SIGTERM — sent by Docker, Kubernetes, and systemd on shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		// http.ErrServerClosed is returned by ListenAndServe after
		// Shutdown is called — it is not a real error.
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-quit:
		log.Printf("shutdown signal received: %v", sig)
	}

	// ── 10. Graceful shutdown ────────────────────────────────────────
	// Give in-flight requests 30 seconds to complete before
	// forcibly closing all connections.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Printf("server stopped cleanly")
}