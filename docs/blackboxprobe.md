# BlackboxProbe CRD

**API Version:** `monitoring.gaiser.bayern/v1alpha1`
**Kind:** `BlackboxProbe`
**Scope:** Namespaced

Defines **what** to probe (targets) and links to a `BlackboxModule` (how to probe) and a `BlackboxExporter` (where to send probes). The operator generates a prometheus-operator `Probe` CR automatically.

## Spec

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxProbe
metadata:
  name: website-check
  namespace: team-a
spec:
  # Reference to the BlackboxExporter instance.
  # Cross-namespace reference supported.
  exporterRef:
    name: main
    namespace: monitoring  # Optional, defaults to same namespace.

  # Reference to the BlackboxModule to use.
  # Cross-namespace reference supported.
  moduleRef:
    name: http-2xx-tls
    namespace: monitoring  # Optional, defaults to same namespace.

  # Static list of targets to probe.
  # At least one of targets or ingress must be set.
  targets:
    - https://example.com
    - https://api.example.com
    - https://status.example.com:8443/health

  # Ingress-based target discovery (alternative or addition to static targets).
  # The operator configures a target for each host of matching Ingress objects.
  # At least one of targets or ingress must be set.
  # ingress:
  #   selector:
  #     matchLabels:
  #       monitoring: "true"
  #   namespaceSelector:
  #     any: true
  #   relabelConfigs: []

  # Scrape interval for the generated prometheus-operator Probe CR.
  # Default: 60s
  interval: 60s

  # Scrape timeout (must be <= interval).
  # Default: 10s
  scrapeTimeout: 10s

  # Additional labels applied to the generated Probe CR.
  # These propagate to scraped metrics.
  additionalLabels:
    team: platform
    env: production

  # Additional metric relabelings applied to the generated Probe CR.
  metricRelabelings:
    - sourceLabels: [__name__]
      regex: "probe_.*"
      action: keep
```

## Cross-Namespace Reference Resolution

```
team-a/website-check (BlackboxProbe)
  |-- exporterRef: monitoring/main (BlackboxExporter)
  |     \-- Service: monitoring/main-blackbox-exporter:9115
  \-- moduleRef: monitoring/http-2xx-tls (BlackboxModule)
        \-- Module name in config: monitoring-http-2xx-tls
```

The generated prometheus-operator `Probe` CR is created **in the same namespace as the `BlackboxProbe`**, pointing to the exporter's cross-namespace Service address.

## Generated prometheus-operator Probe CR

For the example above, the operator generates:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Probe
metadata:
  name: website-check
  namespace: team-a
  labels:
    app.kubernetes.io/managed-by: blackbox-exporter-operator
    monitoring.gaiser.bayern/exporter: main
    monitoring.gaiser.bayern/module: http-2xx-tls
    team: platform
    env: production
  ownerReferences:
    - apiVersion: monitoring.gaiser.bayern/v1alpha1
      kind: BlackboxProbe
      name: website-check
      uid: <uid>
spec:
  prober:
    url: main-blackbox-exporter.monitoring.svc.cluster.local:9115
    scheme: http
    path: /probe
  module: monitoring-http-2xx-tls
  interval: 60s
  scrapeTimeout: 10s
  targets:
    staticConfig:
      targets:
        - https://example.com
        - https://api.example.com
        - https://status.example.com:8443/health
      labels:
        team: platform
        env: production
  metricRelabelings:
    - sourceLabels: [__name__]
      regex: "probe_.*"
      action: keep
```

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `exporterRef.name` | string | required | Name of the `BlackboxExporter` |
| `exporterRef.namespace` | string | same namespace | Namespace of the `BlackboxExporter` |
| `moduleRef.name` | string | required | Name of the `BlackboxModule` |
| `moduleRef.namespace` | string | same namespace | Namespace of the `BlackboxModule` |
| `targets` | []string | `[]` | Static list of targets to probe (URLs or host:port) |
| `ingress` | IngressTargetConfig | - | Ingress-based target discovery |
| `ingress.selector` | metav1.LabelSelector | `{}` | Select Ingress objects by label |
| `ingress.namespaceSelector` | NamespaceSelector | - | Select namespaces for Ingress discovery |
| `ingress.relabelConfigs` | []RelabelConfig | `[]` | Relabeling for discovered Ingress targets |
| `interval` | string | `60s` | Scrape interval |
| `scrapeTimeout` | string | `10s` | Scrape timeout (must be <= interval) |
| `additionalLabels` | map[string]string | `{}` | Extra labels on generated Probe CR and metrics |
| `metricRelabelings` | []RelabelConfig | `[]` | Metric relabeling rules |

## Status

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: ProbeCreated
      message: "prometheus-operator Probe CR created"
  targetCount: 3
  probeRef:
    name: website-check
    namespace: team-a
  observedGeneration: 1
```

| Status Field | Type | Description |
|-------------|------|-------------|
| `conditions` | []metav1.Condition | Standard conditions (`Ready`) |
| `targetCount` | int | Number of configured targets |
| `probeRef` | NamespacedReference | Reference to the generated `Probe` CR |
| `observedGeneration` | int64 | Last observed `.metadata.generation` |

## Validation Rules

- `spec.exporterRef.name` is required
- `spec.moduleRef.name` is required
- At least one of `spec.targets` or `spec.ingress` must be set
- `spec.scrapeTimeout` must be <= `spec.interval`

## Common Types

### NamespacedReference

Used by `exporterRef` and `moduleRef`:

```go
type NamespacedReference struct {
    // Name of the referenced resource. Required.
    Name string `json:"name"`
    // Namespace of the referenced resource.
    // Defaults to the namespace of the referencing resource.
    // +optional
    Namespace string `json:"namespace,omitempty"`
}
```
