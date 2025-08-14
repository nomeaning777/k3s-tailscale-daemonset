// Package tailscale provides Tailscale management functionality.
package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

const (
	// CommandTimeout is the timeout for Tailscale command execution.
	commandTimeout = 30 * time.Second
	// StatusTimeout is the timeout for status checks.
	statusTimeout = 10 * time.Second
)

// ErrTailscaleNotRunning is returned when Tailscale daemon is not running.
var ErrTailscaleNotRunning = errors.New("tailscale is not running")

// Manager handles Tailscale operations.
type Manager struct {
	logger *slog.Logger
}

// NewManager creates a new Tailscale manager instance.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// DebugPrefs represents Tailscale debug preferences.
type DebugPrefs struct {
	AdvertiseRoutes []string `json:"AdvertiseRoutes"`
}

// GetCurrentRoutes returns the currently advertised routes from Tailscale.
func (m *Manager) GetCurrentRoutes() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "debug", "prefs")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get tailscale prefs: %w", err)
	}

	var prefs DebugPrefs
	if err := json.Unmarshal(output, &prefs); err != nil {
		return nil, fmt.Errorf("failed to parse tailscale prefs: %w", err)
	}

	return prefs.AdvertiseRoutes, nil
}

// SetAdvertiseRoutes configures Tailscale to advertise the specified routes.
func (m *Manager) SetAdvertiseRoutes(routes []string) error {
	if len(routes) == 0 {
		m.logger.Info("No routes to advertise")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	routesStr := strings.Join(routes, ",")
	//nolint:gosec // routes are validated
	cmd := exec.CommandContext(ctx, "tailscale", "set", "--advertise-routes="+routesStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set advertise routes: %w, output: %s", err, string(output))
	}

	m.logger.Info("Successfully set advertise routes", "routes", routes, "count", len(routes))
	return nil
}

// MergeRoutes combines current and new routes into a unified list.
func (m *Manager) MergeRoutes(currentRoutes, newRoutes []string) []string {
	if len(currentRoutes) == 0 && len(newRoutes) == 0 {
		return []string{}
	}

	routeMap := make(map[string]bool)

	for _, route := range currentRoutes {
		routeMap[route] = true
	}

	for _, route := range newRoutes {
		routeMap[route] = true
	}

	mergedRoutes := make([]string, 0, len(routeMap))
	for route := range routeMap {
		mergedRoutes = append(mergedRoutes, route)
	}

	return mergedRoutes
}

// IsConnected checks if Tailscale is connected and running.
func (m *Manager) IsConnected() error {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get tailscale status: %w", err)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(output, &status); err != nil {
		return fmt.Errorf("failed to parse tailscale status: %w", err)
	}

	if backendState, ok := status["BackendState"].(string); ok {
		if backendState != "Running" {
			return fmt.Errorf("%w, current state: %s", ErrTailscaleNotRunning, backendState)
		}
	}

	return nil
}
