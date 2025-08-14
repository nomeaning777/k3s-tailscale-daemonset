// Package iprule provides IP rule management for routing configuration.
package iprule

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"

	"github.com/vishvananda/netlink"
)

const (
	// RulePriority is the priority for IP rules managed by this package.
	RulePriority = 2500
	// RuleTable is the routing table for IP rules (main table).
	RuleTable = syscall.RT_TABLE_MAIN
)

// Manager handles IP rule operations for network routing.
type Manager struct {
	logger *slog.Logger
}

// NewManager creates a new IP rule manager instance.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// GetCurrentRules returns the list of current IP rules managed by this package.
func (m *Manager) GetCurrentRules() ([]string, error) {
	rules, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list ip rules: %w", err)
	}

	var ourRules []string
	for _, rule := range rules {
		// Filter for our rules (priority 2500, table main)
		if rule.Priority == RulePriority && rule.Table == RuleTable && rule.Dst != nil {
			ourRules = append(ourRules, rule.Dst.String())
		}
	}

	return ourRules, nil
}

// AddRule adds a new IP rule for the specified CIDR.
func (m *Manager) AddRule(cidr string) error {
	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	rule := netlink.NewRule()
	rule.Priority = RulePriority
	rule.Table = RuleTable
	rule.Dst = dst

	err = netlink.RuleAdd(rule)
	if err != nil {
		// Check if rule already exists
		if strings.Contains(err.Error(), "exists") {
			m.logger.Debug("Rule already exists", "cidr", cidr)
			return nil
		}
		return fmt.Errorf("failed to add ip rule for %s: %w", cidr, err)
	}

	m.logger.Info("Added ip rule",
		"cidr", cidr,
		"priority", RulePriority,
		"table", "main")
	return nil
}

// RemoveRule removes an IP rule for the specified CIDR.
func (m *Manager) RemoveRule(cidr string) error {
	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	rule := netlink.NewRule()
	rule.Priority = RulePriority
	rule.Table = RuleTable
	rule.Dst = dst

	err = netlink.RuleDel(rule)
	if err != nil {
		// Check if rule doesn't exist
		if strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not exist") {
			m.logger.Debug("Rule does not exist", "cidr", cidr)
			return nil
		}
		return fmt.Errorf("failed to remove ip rule for %s: %w", cidr, err)
	}

	m.logger.Info("Removed ip rule", "cidr", cidr)
	return nil
}

// SyncRules synchronizes IP rules to match the desired state.
func (m *Manager) SyncRules(desiredCIDRs []string) error {
	currentCIDRs, err := m.GetCurrentRules()
	if err != nil {
		return fmt.Errorf("failed to get current rules: %w", err)
	}

	desiredMap := make(map[string]bool)
	for _, cidr := range desiredCIDRs {
		desiredMap[cidr] = true
	}

	currentMap := make(map[string]bool)
	for _, cidr := range currentCIDRs {
		currentMap[cidr] = true
	}

	// Add missing rules
	for cidr := range desiredMap {
		if !currentMap[cidr] {
			if err := m.AddRule(cidr); err != nil {
				m.logger.Error("Failed to add rule", "cidr", cidr, "error", err)
			}
		}
	}

	// Remove obsolete rules
	for cidr := range currentMap {
		if !desiredMap[cidr] {
			if err := m.RemoveRule(cidr); err != nil {
				m.logger.Error("Failed to remove rule", "cidr", cidr, "error", err)
			}
		}
	}

	return nil
}
