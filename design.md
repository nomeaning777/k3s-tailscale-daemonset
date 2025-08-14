# System Design Document — k3s-tailscale-daemonset

## 1. System Overview

A system that runs as a DaemonSet on a k3s cluster, configuring Tailscale custom subnet routers and managing RPDB rules on each node.

### Key Features
- Add new routes while preserving existing Tailscale advertise routes
- Automatically configure ip rules for specified CIDRs (priority: 2500, table: main)
- Ensure idempotency and execute periodic reconciliation

## 2. Architecture

### 2.1 Component Structure

```
┌─────────────────────────────────────┐
│         k3s Cluster                 │
│                                     │
│  ┌─────────────────────────────┐   │
│  │     ConfigMap               │   │
│  │   (routes configuration)    │   │
│  └──────────┬──────────────────┘   │
│             │ VolumeMount          │
│  ┌──────────▼──────────────────┐   │
│  │     DaemonSet Pod           │   │
│  │  ┌────────────────────┐     │   │
│  │  │  Go Application     │     │   │
│  │  │                     │     │   │
│  │  │  - Config Loader    │     │   │
│  │  │  - Route Manager    │     │   │
│  │  │  - Rule Manager     │     │   │
│  │  │  - Reconciler       │     │   │
│  │  └──────┬──────────────┘     │   │
│  │         │                    │   │
│  │  hostNetwork: true           │   │
│  │  hostPath: /var/run/tailscale│   │
│  └─────────┬────────────────────┘   │
│            │                        │
│  ┌─────────▼────────────────────┐   │
│  │     Node (Host)              │   │
│  │  - tailscaled socket         │   │
│  │  - iproute2 (ip rule)        │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
```

### 2.2 Data Flow

1. **Configuration Loading**: Read YAML configuration from ConfigMap as a file
2. **Current State Retrieval**: Get existing advertise routes using `tailscale debug prefs`
3. **Route Merging**: Merge existing and new routes (deduplication)
4. **Tailscale Configuration**: Apply using `tailscale set --advertise-routes`
5. **IP Rule Configuration**: Execute `ip rule add` for each CIDR
6. **Reconcile**: Repeat the above process periodically (every 60 seconds)

## 3. Detailed Design

### 3.1 Configuration File Format

```yaml
# /config/config.yaml (mounted from ConfigMap)
routes:
  - "10.0.0.0/8"
  - "192.168.0.0/16"
  - "172.16.0.0/12"
```

### 3.2 Module Structure

#### 3.2.1 Main Loop
- Initialization process
- Reconcile loop management (60-second interval)
- Signal handling (SIGTERM/SIGINT)

#### 3.2.2 Config Loader
- Read YAML configuration file
- Validate configuration values (CIDR format check)

#### 3.2.3 Route Manager
- Execute `tailscale debug prefs` and parse JSON
- Extract existing advertise routes
- Merge with new routes
- Execute `tailscale set --advertise-routes`

#### 3.2.4 Rule Manager
- Get current ip rules (`ip rule list`)
- Add necessary rules (`ip rule add from all to <CIDR> priority 2500 table main`)
- Remove obsolete rules (CIDRs removed from configuration)

#### 3.2.5 Reconciler
- Coordinate Route Manager and Rule Manager
- Error handling and retry
- Status management

### 3.3 Processing Flow

```go
// Pseudo code
func reconcile() error {
    // 1. Load configuration
    config := loadConfig("/config/config.yaml")
    
    // 2. Get current Tailscale routes
    currentRoutes := getTailscaleRoutes() // tailscale debug prefs
    
    // 3. Merge routes
    mergedRoutes := mergeRoutes(currentRoutes, config.Routes)
    
    // 4. Update Tailscale configuration
    if routesChanged(currentRoutes, mergedRoutes) {
        setTailscaleRoutes(mergedRoutes) // tailscale set --advertise-routes
    }
    
    // 5. Configure IP rules
    currentRules := getIPRules() // ip rule list
    for _, route := range config.Routes {
        if !ruleExists(currentRules, route) {
            addIPRule(route, 2500) // ip rule add
        }
    }
    
    // 6. Remove obsolete rules
    removeObsoleteRules(currentRules, config.Routes)
    
    return nil
}
```

### 3.4 Error Handling

- **Configuration file not found**: Log error, retry on next reconcile
- **Tailscale connection error**: Log error, retry with exponential backoff
- **ip rule error**: Handle individually, continue processing other rules
- **Panic**: Capture with recover, log and continue processing

## 4. Kubernetes Manifest Design

### 4.1 DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: tailscale-subnet-router
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: tailscale-subnet-router
  template:
    metadata:
      labels:
        app: tailscale-subnet-router
    spec:
      hostNetwork: true
      containers:
      - name: subnet-router
        image: ghcr.io/nomeaning777/k3s-tailscale-daemonset:latest
        securityContext:
          capabilities:
            add: ["NET_ADMIN"]
        volumeMounts:
        - name: tailscale-socket
          mountPath: /var/run/tailscale
        - name: config
          mountPath: /config
        readinessProbe:
          exec:
            command: ["/app/healthcheck"]
          initialDelaySeconds: 10
          periodSeconds: 30
      volumes:
      - name: tailscale-socket
        hostPath:
          path: /var/run/tailscale
          type: Directory
      - name: config
        configMap:
          name: tailscale-routes
```

### 4.2 ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-routes
  namespace: kube-system
data:
  config.yaml: |
    routes:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
```

## 5. Build and Deployment Design

### 5.1 Docker Image

- Base image: `alpine:3.19` (lightweight)
- Required packages: `tailscale`, `iproute2`
- Multi-architecture: linux/amd64, linux/arm64, linux/arm/v7

### 5.2 Build Process

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.19
RUN apk add --no-cache tailscale iproute2
COPY --from=builder /build/app /app/main
COPY --from=builder /build/healthcheck /app/healthcheck
ENTRYPOINT ["/app/main"]
```

## 6. Logging Design

### Log Levels
- **INFO**: Important operations (route addition, ip rule configuration)
- **ERROR**: When errors occur
- **DEBUG**: Detailed logs only when environment variable `DEBUG=true`

### Log Format
```
2024-01-15T10:30:45Z INFO: Applied advertise routes: [10.0.0.0/8, 192.168.0.0/16]
2024-01-15T10:30:46Z INFO: Added ip rule for 10.0.0.0/8 (priority: 2500)
2024-01-15T10:31:45Z INFO: Reconcile completed successfully
```

## 7. Health Check Design

### Readiness Probe
- Within 90 seconds since last successful reconcile
- Tailscale socket connection is available
- Configuration file is readable

## 8. Security Considerations

- **Least Privilege**: Grant only NET_ADMIN capability
- **No Secrets**: No authentication credentials included
- **Read-only Mount**: ConfigMap is read-only
- **Namespace Isolation**: Run in kube-system

## 9. Operational Considerations

### Monitoring
- Monitor Pod restart count
- Monitor error rate in logs

### Troubleshooting
- Check logs with `kubectl logs -n kube-system ds/tailscale-subnet-router`
- Check internal Pod state with `kubectl exec`

### Upgrade
- Reflect ConfigMap updates by Pod restart
- Update image with `kubectl set image`

## 10. Future Extensibility

- Expose Prometheus metrics
- Webhook notifications
- Support for multiple configuration profiles
- CRD for fine-grained control