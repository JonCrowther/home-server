package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JonCrowther/home-server/lifx"
	"github.com/JonCrowther/home-server/nhl"
)

// defaultLogLevel is the log level used when LOG_LEVEL isn't set. A debug
// build overrides it at link time:
//
//	go build -ldflags="-X main.defaultLogLevel=DEBUG"
var defaultLogLevel = "INFO"

func main() {
	if err := loadAppOptions(appOptionsPath); err != nil {
		fmt.Fprintln(os.Stderr, "loading Home Assistant app options:", err)
		os.Exit(1)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(envOr("LOG_LEVEL", defaultLogLevel))); err != nil {
		fmt.Fprintln(os.Stderr, "invalid LOG_LEVEL:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	addr := ":" + envOr("PORT", "8080")

	lifxToken, ok := os.LookupEnv("LIFX_TOKEN")
	if !ok || lifxToken == "" {
		slog.Error("LIFX_TOKEN not set")
		os.Exit(1)
	}

	// Kubernetes sends SIGTERM when stopping a pod; shut down gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /lights", lifx.LoadLights(lifxToken))
	mux.HandleFunc("GET /scenes", lifx.GetScenes(lifxToken))
	mux.HandleFunc("PUT /scenes/{uuid}/activate", lifx.SetScene(lifxToken))
	mux.HandleFunc("PUT /lights/states", lifx.SetMultipleLights(lifxToken))
	mux.HandleFunc("PUT /lights/{selector}/state", lifx.SetLight(lifxToken))
	mux.HandleFunc("POST /lights/{selector}/breathe", lifx.Breathe(lifxToken))
	mux.HandleFunc("POST /lights/sweep", lifx.Sweep(ctx, lifxToken))
	mux.HandleFunc("POST /lights/{selector}/flames", lifx.Flame(lifxToken))
	mux.HandleFunc("POST /lights/stop", lifx.Stop(lifxToken))
	mux.HandleFunc("POST /lights/{selector}/morph", lifx.Morph(lifxToken))
	watcher := nhl.NewWatcher(ctx, lifxToken)
	mux.HandleFunc("POST /games/{id}/watch/{team}", watcher.WatchGame)
	mux.HandleFunc("POST /nhl/watch/{team}", watcher.WatchTeam)
	mux.HandleFunc("DELETE /nhl/watch/{team}", watcher.StopWatch)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("starting server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
