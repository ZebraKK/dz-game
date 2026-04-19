// Command gamesvr is the main entry point for the QBDZ game server.
// It loads configuration, registers a set of tables, starts each table's
// goroutine, and serves the HTTP JSON API on the configured address.
//
// Signals:
//
//	SIGTERM / SIGINT — graceful shutdown (wait for in-flight requests)
//	SIGHUP           — hot-reload of YAML configuration
//
// Debug endpoints (served on the same port):
//
//	GET /debug/vars   — expvar metrics (active tables, hands, actions, bills)
//	GET /debug/pprof/ — Go runtime profiling
//	GET /healthz      — load-balancer health probe
package main

import (
	"context"
	"expvar"
	"flag"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof/* on DefaultServeMux
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qbgames/qbdz-game/internal/config"
	"github.com/qbgames/qbdz-game/internal/metrics"
	"github.com/qbgames/qbdz-game/internal/server"
	"github.com/qbgames/qbdz-game/internal/stub"
	"github.com/qbgames/qbdz-game/internal/table"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	cfgPath := flag.String("config", "config/default.yaml", "path to YAML config file")
	flag.Parse()

	// ── Load configuration ────────────────────────────────────────────────────

	loader, err := config.NewLoader(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "path", *cfgPath, "err", err)
		os.Exit(1)
	}
	loader.WatchSIGHUP()

	cfg := loader.Get()

	// ── Build table registry ──────────────────────────────────────────────────

	registry := server.NewRegistry()

	// For the stub period, create a few demo tables backed by the TableStub.
	// In production, tables are created dynamically on demand by the lobby.
	for tableID := uint32(1); tableID <= 3; tableID++ {
		host := stub.NewTableStub()
		t := table.NewTable(tableID, &cfg.Scene, host)

		if err := registry.Add(tableID, t); err != nil {
			slog.Error("failed to register table", "tableID", tableID, "err", err)
			os.Exit(1)
		}
	}

	// ── Start table goroutines ────────────────────────────────────────────────

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry.ForEach(func(id uint32, t *table.Table) {
		go func(id uint32, tbl *table.Table) {
			slog.Info("table started", "tableID", id)
			tbl.Run(ctx)
			slog.Info("table stopped", "tableID", id)
		}(id, t)
	})

	// ── HTTP mux ──────────────────────────────────────────────────────────────

	mux := http.NewServeMux()

	// Game API
	h := server.NewHandler(registry)
	h.RegisterRoutes(mux)

	// Health probe
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// expvar metrics at /debug/vars
	mux.Handle("GET /debug/vars", expvar.Handler())

	// pprof: forward /debug/pprof/* to DefaultServeMux where net/http/pprof registered
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	// HTTP request counter middleware
	srv := &http.Server{
		Addr:         *addr,
		Handler:      countRequests(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("gamesvr listening", "addr", *addr,
			"debug_vars", *addr+"/debug/vars",
			"pprof", *addr+"/debug/pprof/")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down",
		"hands_total", metrics.HandsTotal.Value(),
		"actions_total", metrics.ActionsTotal.Value())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "err", err)
	}

	cancel()
	slog.Info("gamesvr stopped")
}

// countRequests wraps a handler to increment the HTTP request counter.
func countRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.HTTPRequests.Add(1)
		h.ServeHTTP(w, r)
	})
}
