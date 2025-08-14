package iprule

import (
	"log/slog"
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := NewManager(logger)

	if m == nil {
		t.Error("expected non-nil manager")
		return
	}
	if m.logger != logger {
		t.Error("logger not properly set")
	}
}

// Integration tests that require root privileges and netlink access
// These would typically be run in a container or with mocking in production code

func TestGetCurrentRules_Integration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := NewManager(logger)

	rules, err := m.GetCurrentRules()
	if err != nil {
		t.Fatalf("GetCurrentRules() error = %v", err)
	}

	// Should return a slice (possibly empty)
	if rules == nil {
		t.Error("expected non-nil slice")
	}
}

func TestAddRemoveRule_Integration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := NewManager(logger)

	testCIDR := "203.0.113.0/24" // TEST-NET-3

	// Add rule
	err := m.AddRule(testCIDR)
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	// Verify rule exists
	rules, err := m.GetCurrentRules()
	if err != nil {
		t.Fatalf("GetCurrentRules() error = %v", err)
	}

	found := false
	for _, cidr := range rules {
		if cidr == testCIDR {
			found = true
			break
		}
	}
	if !found {
		t.Error("added rule not found")
	}

	// Remove rule
	err = m.RemoveRule(testCIDR)
	if err != nil {
		t.Fatalf("RemoveRule() error = %v", err)
	}

	// Verify rule is gone
	rules, err = m.GetCurrentRules()
	if err != nil {
		t.Fatalf("GetCurrentRules() error = %v", err)
	}

	for _, cidr := range rules {
		if cidr == testCIDR {
			t.Error("removed rule still exists")
		}
	}
}

func TestAddRule_InvalidCIDR(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := NewManager(logger)

	err := m.AddRule("invalid-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestRemoveRule_InvalidCIDR(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := NewManager(logger)

	err := m.RemoveRule("invalid-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}
