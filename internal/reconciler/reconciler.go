// Package reconciler coordinates Tailscale route advertisement and IP rule management.
package reconciler

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"time"

	"github.com/nomeaning777/k3s-tailscale-daemonset/internal/config"
	"github.com/nomeaning777/k3s-tailscale-daemonset/internal/iprule"
	"github.com/nomeaning777/k3s-tailscale-daemonset/internal/tailscale"
)

const (
	// DefaultHealthTimeout is the default timeout for health checks.
	defaultHealthTimeout = 90 * time.Second
)

// Reconciler coordinates between Tailscale and IP rule management.
type Reconciler struct {
	configPath        string
	tailscaleManager  *tailscale.Manager
	ruleManager       *iprule.Manager
	logger            *slog.Logger
	lastReconcileTime time.Time
	lastReconcileErr  error
}

// New creates a new Reconciler instance.
func New(configPath string, logger *slog.Logger) *Reconciler {
	return &Reconciler{
		configPath:       configPath,
		tailscaleManager: tailscale.NewManager(logger),
		ruleManager:      iprule.NewManager(logger),
		logger:           logger,
	}
}

// Reconcile synchronizes Tailscale routes and IP rules based on configuration.
func (r *Reconciler) Reconcile() error {
	startTime := time.Now()
	r.logger.Info("Starting reconciliation")

	cfg, err := config.Load(r.configPath)
	if err != nil {
		r.lastReconcileErr = err
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err = r.tailscaleManager.IsConnected(); err != nil {
		r.lastReconcileErr = err
		return fmt.Errorf("tailscale is not connected: %w", err)
	}

	currentRoutes, err := r.tailscaleManager.GetCurrentRoutes()
	if err != nil {
		r.lastReconcileErr = err
		return fmt.Errorf("failed to get current routes: %w", err)
	}
	r.logger.Info("Current advertised routes", "routes", currentRoutes)

	mergedRoutes := r.tailscaleManager.MergeRoutes(currentRoutes, cfg.Routes)
	sort.Strings(mergedRoutes)
	sort.Strings(currentRoutes)

	if !reflect.DeepEqual(currentRoutes, mergedRoutes) {
		r.logger.Info("Routes changed, updating",
			"current", currentRoutes,
			"new", mergedRoutes)
		if err := r.tailscaleManager.SetAdvertiseRoutes(mergedRoutes); err != nil {
			r.lastReconcileErr = err
			return fmt.Errorf("failed to set advertise routes: %w", err)
		}
	} else {
		r.logger.Debug("Routes unchanged, skipping Tailscale update")
	}

	if err := r.ruleManager.SyncRules(cfg.Routes); err != nil {
		r.lastReconcileErr = err
		return fmt.Errorf("failed to sync ip rules: %w", err)
	}

	r.lastReconcileTime = time.Now()
	r.lastReconcileErr = nil
	r.logger.Info("Reconciliation completed successfully",
		"duration", time.Since(startTime))
	return nil
}

// IsHealthy checks if the reconciler is healthy with default timeout.
func (r *Reconciler) IsHealthy() bool {
	return r.IsHealthyWithTimeout(defaultHealthTimeout)
}

// IsHealthyWithTimeout checks if the reconciler is healthy within the specified timeout.
func (r *Reconciler) IsHealthyWithTimeout(timeout time.Duration) bool {
	if r.lastReconcileErr != nil {
		return false
	}

	if time.Since(r.lastReconcileTime) > timeout {
		return false
	}

	if err := r.tailscaleManager.IsConnected(); err != nil {
		return false
	}

	return true
}

// GetLastError returns the last reconciliation error, if any.
func (r *Reconciler) GetLastError() error {
	return r.lastReconcileErr
}

// GetLastReconcileTime returns the timestamp of the last successful reconciliation.
func (r *Reconciler) GetLastReconcileTime() time.Time {
	return r.lastReconcileTime
}
