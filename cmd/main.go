// Package main provides the main entry point for the k3s-tailscale-daemonset.
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

	"github.com/nomeaning777/k3s-tailscale-daemonset/internal/reconciler"
)

const (
	defaultConfigPath           = "/config/config.yaml"
	defaultReconcileIntervalSec = 60
	defaultHealthPort           = 8080
	defaultHealthTimeout        = 90 * time.Second
	httpReadHeaderTimeout       = 10 * time.Second
)

func getLogLevel() slog.Level {
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "INFO", "info":
		return slog.LevelInfo
	case "WARN", "warn":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getReconcileInterval(logger *slog.Logger) time.Duration {
	interval := os.Getenv("RECONCILE_INTERVAL")
	if interval == "" {
		return time.Duration(defaultReconcileIntervalSec) * time.Second
	}

	parsed, err := time.ParseDuration(interval)
	if err != nil {
		logger.Error("Invalid RECONCILE_INTERVAL format",
			"value", interval,
			"error", err)
		os.Exit(1)
	}
	if parsed <= 0 {
		logger.Error("RECONCILE_INTERVAL must be positive",
			"value", parsed)
		os.Exit(1)
	}
	return parsed
}

func getHealthPort() int {
	portStr := os.Getenv("HEALTH_PORT")
	if portStr == "" {
		return defaultHealthPort
	}

	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return defaultHealthPort
	}
	return port
}

func getHealthTimeout(logger *slog.Logger) time.Duration {
	timeout := os.Getenv("HEALTH_TIMEOUT")
	if timeout == "" {
		return defaultHealthTimeout
	}

	parsed, err := time.ParseDuration(timeout)
	if err != nil {
		logger.Warn("Invalid HEALTH_TIMEOUT format, using default",
			"value", timeout,
			"error", err,
			"default", defaultHealthTimeout)
		return defaultHealthTimeout
	}
	if parsed <= 0 {
		logger.Warn("HEALTH_TIMEOUT must be positive, using default",
			"value", parsed,
			"default", defaultHealthTimeout)
		return defaultHealthTimeout
	}
	return parsed
}

func startHealthServer(rec *reconciler.Reconciler, port int, healthTimeout time.Duration, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if rec.IsHealthyWithTimeout(healthTimeout) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "OK - Last reconcile: %s\n", rec.GetLastReconcileTime().Format(time.RFC3339)) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		if err := rec.GetLastError(); err != nil {
			_, _ = fmt.Fprintf(w, "Unhealthy - Last error: %v\n", err) //nolint:errcheck
		} else if rec.GetLastReconcileTime().IsZero() {
			_, _ = fmt.Fprintln(w, "Unhealthy - No successful reconciliation yet") //nolint:errcheck
		} else {
			_, _ = fmt.Fprintf(w, "Unhealthy - Last reconcile too old: %s\n", //nolint:errcheck
				rec.GetLastReconcileTime().Format(time.RFC3339))
		}
	})

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting health check server", "port", port)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Health server failed", "error", err)
	}
}

func main() {
	logLevel := getLogLevel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	reconcileInterval := getReconcileInterval(logger)
	healthPort := getHealthPort()
	healthTimeout := getHealthTimeout(logger)

	logger.Info("Starting k3s-tailscale-daemonset",
		"config", configPath,
		"reconcileInterval", reconcileInterval,
		"healthPort", healthPort,
		"healthTimeout", healthTimeout)

	rec := reconciler.New(configPath, logger)

	// Start health check server in background
	go startHealthServer(rec, healthPort, healthTimeout, logger)

	if err := rec.Reconcile(); err != nil {
		logger.Error("Initial reconciliation failed", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	logger.Info("Starting reconciliation loop")

	for {
		select {
		case <-ticker.C:
			if err := rec.Reconcile(); err != nil {
				logger.Error("Reconciliation failed", "error", err)
			}
		case sig := <-sigChan:
			logger.Info("Received signal, shutting down", "signal", sig)
			return
		case <-ctx.Done():
			logger.Info("Context canceled, shutting down")
			return
		}
	}
}
