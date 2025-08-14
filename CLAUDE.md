# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes DaemonSet that automatically configures Tailscale subnet routers on k3s cluster nodes. It manages Tailscale route advertisements and corresponding IP rules (RPDB) to enable custom subnet routing.

## Core Development Commands

### Build and Run
```bash
# Build the binary
mise run build

# Run all CI tasks (fmt, lint, test, build)
mise run ci

# Build Docker image
mise run docker-build

# Build multi-arch Docker image
mise run docker-buildx
```

### Testing and Validation
```bash
# Run all tests
mise run test

# Run a single test
go test -v -run TestName ./internal/...

# Run linting (comprehensive set of linters via golangci-lint)
mise run lint

# Format code
mise run fmt

# Tidy dependencies
mise run tidy
```

## Architecture

### Component Hierarchy
1. **cmd/main.go**: Entry point, manages reconciliation loop (default 60s interval)
2. **internal/reconciler/reconciler.go**: Orchestrates the entire sync process
3. **internal/config/config.go**: Loads and validates YAML configuration from ConfigMap
4. **internal/tailscale/manager.go**: Interfaces with Tailscale CLI (`tailscale debug prefs`, `tailscale set`)
5. **internal/iprule/manager.go**: Manages IP rules using netlink library (priority 2500, main table)

### Reconciliation Flow
1. Load routes from `/config/config.yaml` (mounted from ConfigMap)
2. Query current Tailscale advertised routes via `tailscale debug prefs`
3. Merge existing routes with configured routes (preserves existing routes)
4. Update Tailscale if routes changed using `tailscale set --advertise-routes`
5. Sync IP rules to match configured CIDRs (add/remove as needed)
6. Repeat every 60 seconds (configurable via RECONCILE_INTERVAL env var)

### Key Design Decisions
- Uses `netlink` library for direct IP rule management instead of exec'ing `ip` commands
- Preserves existing Tailscale routes when adding new ones
- IP rules use fixed priority 2500 and main routing table
- Health check available at `/cmd/healthcheck` (90s default timeout)
- Structured logging with slog, configurable via LOG_LEVEL env var

## Testing Approach
- Unit tests for all managers and config loading
- Mock external dependencies (Tailscale CLI, netlink operations)
- Test files follow Go convention: `*_test.go` in same package
- No integration test framework; unit tests focus on business logic

## Linting Configuration
The project uses golangci-lint v2.4.0 with a comprehensive set of linters including security (gosec), complexity (gocyclo, gocognit), and style checks. Key limits:
- Max cyclomatic complexity: 15
- Max function length: 60 lines / 40 statements
- Max line length: 120 characters
- All linters enabled explicitly (no defaults)