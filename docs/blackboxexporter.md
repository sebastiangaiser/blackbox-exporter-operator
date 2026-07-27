# BlackboxExporter CRD

**API Version:** `monitoring.gaiser.bayern/v1alpha1`
**Kind:** `BlackboxExporter`
**Scope:** Namespaced

Manages the full deployment lifecycle of a blackbox-exporter instance: Deployment, Service, ConfigMap, and optionally a ServiceMonitor.

## Spec

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxExporter
metadata:
  name: main
  namespace: monitoring
spec:
  # Number of blackbox-exporter replicas.
  # Default: 1
  replicas: 2

  # Blackbox-exporter container image.
  # Default: quay.io/prometheus/blackbox-exporter:<operator-default-version>
  image:
    repository: quay.io/prometheus/blackbox-exporter
    tag: v0.28.0
    pullPolicy: IfNotPresent

  # Resource requests and limits for the blackbox-exporter container.
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 128Mi

  # Port the blackbox-exporter listens on.
  # Default: 9115
  port: 9115

  # Additional command-line arguments passed to the blackbox-exporter.
  # --config.file and --config.enable-auto-reload are set automatically.
  additionalArgs:
    - --log.level=debug
    - --timeout-offset=0.5

  # ServiceMonitor for the exporter's own /metrics endpoint.
  serviceMonitor:
    # Whether to create a ServiceMonitor. Default: false
    enabled: false
    # Scrape interval for the exporter's own metrics.
    interval: 30s
    # Additional labels added to the ServiceMonitor.
    labels:
      prometheus: kube-prometheus

  # Select BlackboxModules to include in this exporter's configuration.
  # Modules can be selected by label across namespaces.
  moduleSelector:
    namespaceSelector:
      # Match all namespaces.
      any: true
      # OR: match specific namespaces by name.
      # matchNames:
      #   - monitoring
      #   - modules
    # Select BlackboxModules by label.
    matchLabels:
      exporter: main

  # Scheduling constraints.
  nodeSelector: {}
  tolerations: []
  affinity: {}

  # Additional labels applied to all managed resources.
  additionalLabels: {}

  # Additional annotations applied to all managed resources.
  additionalAnnotations: {}

  # Enable ICMP probing by granting CAP_NET_RAW.
  # Downgrades security context from restricted to baseline PSS.
  # Default: false
  enableICMP: false
```

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `replicas` | int | `1` | Number of exporter pod replicas |
| `image.repository` | string | `quay.io/prometheus/blackbox-exporter` | Container image repository |
| `image.tag` | string | Operator default | Container image tag |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `resources` | corev1.ResourceRequirements | none | CPU/memory requests and limits |
| `port` | int | `9115` | Exporter listen port (1-65535) |
| `additionalArgs` | []string | `[]` | Extra CLI flags |
| `serviceMonitor.enabled` | bool | `false` | Create a ServiceMonitor |
| `serviceMonitor.interval` | string | `30s` | ServiceMonitor scrape interval |
| `serviceMonitor.labels` | map[string]string | `{}` | Extra ServiceMonitor labels |
| `moduleSelector` | ModuleSelector | required | Selects BlackboxModules |
| `moduleSelector.namespaceSelector.any` | bool | `false` | Match all namespaces |
| `moduleSelector.namespaceSelector.matchNames` | []string | `[]` | Match namespaces by name |
| `moduleSelector.matchLabels` | map[string]string | `{}` | Match modules by label |
| `nodeSelector` | map[string]string | `{}` | Pod node selector |
| `tolerations` | []corev1.Toleration | `[]` | Pod tolerations |
| `affinity` | corev1.Affinity | none | Pod affinity rules |
| `additionalLabels` | map[string]string | `{}` | Extra labels on managed resources |
| `additionalAnnotations` | map[string]string | `{}` | Extra annotations on managed resources |
| `enableICMP` | bool | `false` | Grants CAP_NET_RAW for ICMP probes (downgrades to baseline PSS) |

## Owned Resources

| Resource | Name Pattern | Notes |
|----------|-------------|-------|
| `Deployment` | `<name>-blackbox-exporter` | Pod Security `restricted` compliant |
| `Service` | `<name>-blackbox-exporter` | ClusterIP on configured port |
| `ConfigMap` | `<name>-blackbox-exporter` | Rendered `blackbox.yml` |
| `ServiceMonitor` | `<name>-blackbox-exporter` | Only if `serviceMonitor.enabled: true` |

## Pod Security Context

The operator sets the following security context on all managed pods to comply with the `restricted` Pod Security Standard:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  fsGroup: 65534
  seccompProfile:
    type: RuntimeDefault
containers:
  - securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

## Status

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-01-15T10:00:00Z"
      reason: DeploymentAvailable
      message: "Deployment has minimum availability"
    - type: ConfigValid
      status: "True"
      lastTransitionTime: "2025-01-15T10:00:00Z"
      reason: ConfigRendered
      message: "Configuration rendered from 3 modules"
  moduleCount: 3
  readyReplicas: 2
  observedGeneration: 5
```

| Status Field | Type | Description |
|-------------|------|-------------|
| `conditions` | []metav1.Condition | Standard Kubernetes conditions (`Ready`, `ConfigValid`) |
| `moduleCount` | int | Number of modules in the current configuration |
| `readyReplicas` | int | Number of ready pod replicas |
| `observedGeneration` | int64 | Last observed `.metadata.generation` |

## Validation Rules

- `spec.replicas` must be >= 0
- `spec.port` must be between 1 and 65535
- `spec.image.repository` must not be empty
- `spec.moduleSelector` is required (at minimum `namespaceSelector` must be set)
