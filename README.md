# k3s-tailscale-daemonset

A DaemonSet that automatically configures Tailscale custom subnet routers on each node in a k3s cluster.

## Overview

This project provides a Kubernetes DaemonSet for automatically configuring Tailscale subnet routing on each node in a k3s cluster. It advertises CIDRs specified in a ConfigMap via Tailscale and sets up corresponding ip rules.

### Key Features

- Add new routes while preserving existing Tailscale advertise routes
- Automatically configure ip rules for specified CIDRs (priority: 2500)
- Automatic reconciliation every 60 seconds
- Built-in health check HTTP endpoint for Kubernetes probes
- Configurable logging and reconciliation intervals

## Quick Start

### Prerequisites

- A running k3s cluster
- Tailscale authenticated and running on each node
- kubectl CLI configured

### Deployment

1. Edit the ConfigMap to configure the CIDRs to advertise:

```bash
kubectl edit configmap tailscale-routes -n kube-system
```

Or edit k8s/configmap.yaml:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-routes
  namespace: kube-system
data:
  config.yaml: |
    routes:
      - "192.168.0.0/16"
      - "172.16.0.0/12"
```

2. Deploy the DaemonSet:

```bash
kubectl apply -k k8s/
```

Or apply individually:

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/daemonset.yaml
```

### Verification

Check DaemonSet status:

```bash
kubectl get ds -n kube-system tailscale-subnet-router
kubectl logs -n kube-system ds/tailscale-subnet-router
```

Check health status:

```bash
# Check pod health from inside cluster
kubectl exec -n kube-system ds/tailscale-subnet-router -- curl -s http://localhost:8080/healthz

# Check pod events for probe failures
kubectl describe pod -n kube-system -l app=tailscale-subnet-router
```

## Configuration

### ConfigMap Format

```yaml
routes:
  - "192.168.0.0/16"   # Private network
  - "172.16.0.0/12"    # Private network
```

### Environment Variables

- `CONFIG_PATH`: Path to configuration file (default: `/config/config.yaml`)
- `LOG_LEVEL`: Logging level - DEBUG, INFO, WARN, ERROR (default: `INFO`)
- `RECONCILE_INTERVAL`: Reconciliation interval (default: `60s`)
- `HEALTH_PORT`: Health check HTTP server port (default: `8080`)
- `HEALTH_TIMEOUT`: Health check timeout for considering unhealthy (default: `90s`)

## Architecture

### Components

1. **Config Loader**: Loads configuration from YAML file
2. **Tailscale Manager**: Manages routes using `tailscale debug prefs` and `tailscale set`
3. **IP Rule Manager**: Manages RPDB rules using netlink library
4. **Reconciler**: Periodically synchronizes configuration
5. **Health Server**: HTTP endpoint at `/healthz` for Kubernetes probes

### Workflow

1. Start health check HTTP server on configured port
2. Load configuration from ConfigMap
3. Get current Tailscale advertise routes
4. Merge existing and new routes
5. Apply to Tailscale if changed
6. Configure corresponding ip rules
7. Repeat every configured interval (default: 60 seconds)

## Development

### Prerequisites

Install [mise](https://mise.jdx.dev/) for managing tools and tasks:

```bash
curl https://mise.run | sh
```

### Setup

```bash
# Install required tools
mise install

# List available tasks
mise tasks
```

### Common Tasks

```bash
# Build the binary
mise run build

# Run tests
mise run test

# Run linting
mise run lint

# Format code
mise run fmt

# Run go mod tidy
mise run tidy

# Build Docker image
mise run docker-build

# Build multi-architecture Docker image
mise run docker-buildx

# Run all CI tasks (fmt, lint, test, build)
mise run ci
```

## Troubleshooting

### Check Logs

```bash
kubectl logs -n kube-system ds/tailscale-subnet-router -f
```

### Restart Pods

```bash
kubectl rollout restart -n kube-system ds/tailscale-subnet-router
```

### Debug

Execute commands inside a Pod:

```bash
kubectl exec -it -n kube-system ds/tailscale-subnet-router -- sh
```

## License

MIT License

## Contributing

Issues and Pull Requests are welcome.