# Blackbox Exporter Operator - Architecture

## Overview

The Blackbox Exporter Operator is a Kubernetes operator that manages the full lifecycle of [Prometheus blackbox-exporter](https://github.com/prometheus/blackbox_exporter) instances. It provides a declarative, Kubernetes-native way to deploy blackbox-exporter, define probe modules, and configure monitoring targets.

The operator integrates with the [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) ecosystem by generating `Probe` and `ServiceMonitor` custom resources automatically.

## Goals

- **Full lifecycle management**: Deploy and manage blackbox-exporter instances (Deployment, Service, ConfigMap) via a single CR.
- **Declarative module configuration**: Define reusable blackbox-exporter modules as Kubernetes resources.
- **Automatic prometheus-operator integration**: Generate `monitoring.coreos.com/v1` `Probe` CRs so Prometheus discovers and scrapes targets without manual configuration.
- **Self-monitoring**: Optionally generate a `ServiceMonitor` for the blackbox-exporter's own `/metrics` endpoint.
- **Security by default**: All workloads comply with the Kubernetes Pod Security Standards `restricted` profile.

## API Group & Versions

| Field | Value |
|-------|-------|
| API Group | `monitoring.gaiser.bayern` |
| Initial Version | `v1alpha1` |

All CRDs are **namespaced**. Cross-namespace references are supported via namespace + name or label selectors.

## Custom Resource Definitions

| CRD | Purpose | Documentation |
|-----|---------|---------------|
| `BlackboxExporter` | Manages the blackbox-exporter deployment lifecycle | [blackboxexporter.md](blackboxexporter.md) |
| `BlackboxModule` | Defines reusable probe module configurations | [blackboxmodule.md](blackboxmodule.md) |
| `BlackboxProbe` | Defines targets and generates prometheus-operator Probe CRs | [blackboxprobe.md](blackboxprobe.md) |

## Architecture Diagram

```
                    User creates
                         |
          +--------------+--------------+
          |              |              |
          v              v              v
  BlackboxExporter  BlackboxModule  BlackboxProbe
  (ns: monitoring)  (ns: any)       (ns: any)
          |              |              |
          |              |              |
          v              |              v
    +-----------+        |     +------------------+
    | Deployment|        |     | prom-op Probe CR |
    | Service   |        |     +------------------+
    | ConfigMap |<-------+              |
    | SvcMonitor|                       |
    +-----------+                       v
          |                     Prometheus scrapes
          v                     /probe?module=X&target=Y
    blackbox-exporter
    pods (restricted PSS)
```

## Cross-Namespace References

All CRDs are namespaced. References between resources support cross-namespace selection:

```yaml
# Option A: Explicit namespace + name
exporterRef:
  name: main
  namespace: monitoring

# Option B: Label selector (across namespaces)
moduleSelector:
  namespaceSelector:
    matchNames: [monitoring, modules]
    # or: any: true
  matchLabels:
    tier: production
```

The operator needs `ClusterRole` permissions to watch resources across namespaces. If the operator is restricted to a single namespace (via `--namespaces` flag), cross-namespace references are limited to that namespace.

## Pod Security Standards

Both the operator and the managed blackbox-exporter pods comply with the Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) `restricted` profile:

| Setting | Value |
|---------|-------|
| `runAsNonRoot` | `true` |
| `runAsUser` | `65534` (nobody) |
| `runAsGroup` | `65534` (nobody) |
| `readOnlyRootFilesystem` | `true` |
| `allowPrivilegeEscalation` | `false` |
| `seccompProfile.type` | `RuntimeDefault` |
| `capabilities.drop` | `["ALL"]` |

> **Note:** ICMP probes require `CAP_NET_RAW`. Set `enableICMP: true` on the `BlackboxExporter` to grant this capability. This adds `NET_RAW` to the container while still dropping all other capabilities, effectively moving from `restricted` to `baseline` Pod Security Standard for that deployment.

## Reconciliation Loops

### BlackboxExporter Controller

**Watches:** `BlackboxExporter`, `BlackboxModule` (to rebuild ConfigMap on module changes)

1. Validate the `BlackboxExporter` spec.
2. Collect all `BlackboxModule` resources matching `moduleSelector`.
3. Render `blackbox.yml` from collected modules and write to a `ConfigMap`.
4. Reconcile the `Deployment` with `restricted` security context.
5. Reconcile the `Service`.
6. Optionally reconcile the `ServiceMonitor` (disabled by default).
7. Update status.

### BlackboxModule Controller

**Watches:** `BlackboxModule`

1. Validate the module configuration.
2. Trigger reconciliation of referencing `BlackboxExporter`(s) to update their ConfigMaps.
3. Update status.

### BlackboxProbe Controller

**Watches:** `BlackboxProbe`

1. Validate the spec.
2. Resolve `moduleRef` and `exporterRef` (potentially cross-namespace).
3. Generate a prometheus-operator `Probe` CR in the same namespace as the `BlackboxProbe`.
4. Update status.

## Configuration Flow

```
BlackboxModule CRs ──> BlackboxExporter Controller ──> ConfigMap (blackbox.yml)
  (any namespace)                                           |
                                                            v
                                                      blackbox-exporter
                                                      (auto-reloads config)

BlackboxProbe CR ──> BlackboxProbe Controller ──> prom-op Probe CR
  (any namespace)                                      |
                                                       v
                                                  Prometheus
                                                  (scrapes /probe endpoint)
```

The blackbox-exporter supports configuration auto-reload (`--config.enable-auto-reload`), so updating the ConfigMap triggers a reload without pod restarts.

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go |
| Scaffolding | Kubebuilder |
| Controller Framework | controller-runtime |
| CRD Generation | controller-gen |
| Kubernetes Client | client-go (via controller-runtime) |
| Testing | envtest, Ginkgo/Gomega |
| CI/CD | GitHub Actions |
| Container Image | Distroless base |

## RBAC Requirements

The operator requires a `ClusterRole` to support cross-namespace resource selection:

| Resource | API Group | Verbs |
|----------|-----------|-------|
| `deployments` | `apps/v1` | get, list, watch, create, update, delete |
| `services` | `v1` | get, list, watch, create, update, delete |
| `configmaps` | `v1` | get, list, watch, create, update, delete |
| `blackboxexporters` | `monitoring.gaiser.bayern/v1alpha1` | get, list, watch, create, update, patch, delete |
| `blackboxmodules` | `monitoring.gaiser.bayern/v1alpha1` | get, list, watch, create, update, patch, delete |
| `blackboxprobes` | `monitoring.gaiser.bayern/v1alpha1` | get, list, watch, create, update, patch, delete |
| `probes` | `monitoring.coreos.com/v1` | get, list, watch, create, update, delete |
| `servicemonitors` | `monitoring.coreos.com/v1` | get, list, watch, create, update, delete |
| `events` | `v1` | create, patch |

## Status Conditions

All three CRDs report status using standard Kubernetes conditions:

| Condition | Meaning |
|-----------|---------|
| `Ready` | Resource is fully reconciled and operational |
| `Degraded` | Resource is partially functional (e.g., referenced module not found) |
| `ConfigValid` | Configuration has been validated successfully |

## Future Considerations

- **High availability**: Multiple replicas with leader election (built into controller-runtime).
