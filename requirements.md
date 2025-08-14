# Requirements for k3s Tailscale Custom Subnet Router

## Summary
Each node in the k3s cluster has Tailscale installed for remote access, but when using Tailscale as a subnet router, there are cases where custom routing needs to be set up on the host side for specific routes instead of standard subnet routes.

This implementation achieves the following goal: **Create a DaemonSet that defines CIDRs to advertise in a ConfigMap and automatically sets them up on each node**.

## Details

### Environment Configuration

1. Each node in the k3s cluster has Tailscale installed and authenticated
2. Tailscale is managed at the OS level, not within k3s
3. DaemonSet is executed with `hostNetwork: true`
4. Tailscale socket is mounted from host's `/var/run/tailscale`

### Processing Steps

1. **On DaemonSet startup:**
   - Read CIDR list from ConfigMap
   - Advertise subnet routes for the specified CIDRs using Tailscale
   - Set up RPDB rules with `ip rule` for the CIDRs

2. **Idempotency:**
   - Skip if the same CIDR is already set up to avoid duplication
   - Support updates when ConfigMap changes

3. **Reconcile loop (runs periodically):**
   - Reapply settings to handle node restarts or configuration drifts

### Technical Requirements

- **Tailscale operations:** Use `tailscale` CLI (e.g., `tailscale up --advertise-routes`)
- **RPDB settings:** Use `ip rule add` (e.g., `ip rule add from all to 10.0.0.0/8 table main`)
- **ConfigMap format:** Define as a YAML list

Example:
```yaml
routes:
  - "10.0.0.0/8"
  - "172.16.0.0/12"
```

### Operational Requirements

- When a node is added, automatically apply the same settings
- Settings are updated within 1 minute when ConfigMap is changed
- Operation logs should be output for debugging purposes