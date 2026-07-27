# Blackbox Exporter Operator

A Kubernetes operator that manages the full lifecycle of [Prometheus blackbox-exporter](https://github.com/prometheus/blackbox_exporter) instances. It provides a declarative way to deploy blackbox-exporter, define probe modules, and configure monitoring targets with automatic [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) integration.

## Features

- **Full lifecycle management** -- deploy blackbox-exporter via a single Custom Resource (Deployment, Service, ConfigMap)
- **Reusable module definitions** -- define probe modules (HTTP, TCP, DNS, ICMP, gRPC, Unix, WebSocket) as Kubernetes resources
- **Automatic Probe generation** -- creates prometheus-operator `Probe` CRs so Prometheus discovers targets automatically
- **Ingress target discovery** -- automatically probe hosts from Ingress resources via label selectors
- **Cross-namespace support** -- modules and probes can reference exporters across namespaces
- **Secret resolution** -- reference passwords, tokens, and TLS certs from Kubernetes Secrets
- **Validating webhooks** -- reject invalid configurations at admission time (cert-manager for TLS)
- **Security by default** -- `restricted` Pod Security Standard; optional `enableICMP` for ICMP probes
- **Optional ServiceMonitor** -- self-monitoring of the blackbox-exporter's `/metrics` endpoint

## Custom Resources

| CRD | Purpose |
|-----|---------|
| `BlackboxExporter` | Deploys and manages a blackbox-exporter instance |
| `BlackboxModule` | Defines a reusable probe module configuration |
| `BlackboxProbe` | Defines targets to probe, generates prometheus-operator `Probe` CRs |

All CRDs use the API group `monitoring.gaiser.bayern/v1alpha1`.

## Prerequisites

- Kubernetes 1.28+
- [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) installed (for `Probe` and `ServiceMonitor` CRDs)
- [cert-manager](https://cert-manager.io/) installed (for webhook TLS certificates)

## Installation

### Helm (OCI)

```sh
helm install blackbox-exporter-operator \
  oci://ghcr.io/sebastiangaiser/charts/blackbox-exporter-operator \
  --version 0.1.0 \
  --namespace blackbox-exporter-operator-system \
  --create-namespace
```

### Helm (local)

```sh
helm install blackbox-exporter-operator charts/blackbox-exporter-operator \
  --namespace blackbox-exporter-operator-system \
  --create-namespace
```

### From source

```sh
make install   # Install CRDs
make deploy IMG=ghcr.io/sebastiangaiser/blackbox-exporter-operator:latest
```

## Quick Start

### 1. Deploy a blackbox-exporter

```yaml
---
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxExporter
metadata:
  name: main
  namespace: monitoring
spec:
  replicas: 1
  moduleSelector:
    namespaceSelector:
      any: true
    matchLabels:
      exporter: main
```

### 2. Define a probe module

The prober type is inferred from which section is set (`http`, `tcp`, `dns`, etc.):

```yaml
---
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: http-2xx
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  http:
    method: GET
    followRedirects: true
    preferredIPProtocol: ip4
```

### 3. Create a probe

```yaml
---
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxProbe
metadata:
  name: website-check
  namespace: monitoring
spec:
  exporterRef:
    name: main
  moduleRef:
    name: http-2xx
  targets:
    - https://example.com
    - https://prometheus.io
  interval: 60s
  scrapeTimeout: 10s
```

This automatically:
- Adds the module to the blackbox-exporter's configuration
- Creates a prometheus-operator `Probe` CR pointing to the exporter
- Prometheus discovers and scrapes the targets

### Cross-namespace usage

Teams can create probes in their own namespace referencing a shared exporter:

```yaml
---
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxProbe
metadata:
  name: our-api
  namespace: team-a
spec:
  exporterRef:
    name: main
    namespace: monitoring
  moduleRef:
    name: http-2xx
    namespace: monitoring
  targets:
    - https://api.team-a.internal/health
  additionalLabels:
    team: team-a
```

### Ingress target discovery

Automatically probe all hosts from matching Ingress resources:

```yaml
---
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxProbe
metadata:
  name: ingress-monitor
  namespace: monitoring
spec:
  exporterRef:
    name: main
  moduleRef:
    name: http-2xx
  ingress:
    selector:
      matchLabels:
        monitoring: "true"
    namespaceSelector:
      any: true
  interval: 60s
```

See the [examples/](examples/) directory for more use cases (TLS validation, TCP, DNS, gRPC, ICMP, authentication, Ingress discovery).

## Documentation

- [Architecture](docs/architecture.md) -- design overview, reconciliation loops, security model
- [BlackboxExporter CRD](docs/blackboxexporter.md)
- [BlackboxModule CRD](docs/blackboxmodule.md)
- [BlackboxProbe CRD](docs/blackboxprobe.md)

## Development

### Prerequisites

- Go 1.25+
- Kubebuilder
- Docker or Podman

### Build

```sh
make build       # Build the operator binary
make manifests   # Regenerate CRD manifests and RBAC
make generate    # Regenerate deepcopy methods
```

### Test

```sh
make test        # Unit tests + envtest integration tests
```

### Run locally

```sh
make install     # Install CRDs into the cluster
make run         # Run the operator locally against the cluster
```

## License

Apache License 2.0
