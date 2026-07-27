# BlackboxModule CRD

**API Version:** `monitoring.gaiser.bayern/v1alpha1`
**Kind:** `BlackboxModule`
**Scope:** Namespaced

Defines a reusable blackbox-exporter module configuration. The module spec maps directly to the [blackbox-exporter module configuration](https://github.com/prometheus/blackbox_exporter/blob/master/CONFIGURATION.md).

Modules are selected by `BlackboxExporter` resources via label selectors and aggregated into the exporter's `blackbox.yml` ConfigMap.

## Module Name in blackbox.yml

The module name in the rendered configuration is derived from the CR to ensure uniqueness across namespaces:

```
<namespace>-<name>
```

Example: `BlackboxModule` named `http-2xx-tls` in namespace `monitoring` becomes `monitoring-http-2xx-tls` in `blackbox.yml`.

## Prober Type

The prober type is determined by which configuration section is present under `spec`. Exactly one of `http`, `tcp`, `dns`, `icmp`, `grpc`, `unix`, or `websocket` must be set.

## Spec Examples

### HTTP

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: http-2xx-tls
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  http:
    method: GET
    headers:
      User-Agent: BlackboxExporter
    validStatusCodes: []  # Defaults to 2xx
    validHTTPVersions:
      - "HTTP/1.1"
      - "HTTP/2.0"
    failIfSSL: false
    failIfNotSSL: true
    followRedirects: true
    preferredIPProtocol: ip4
    tlsConfig:
      insecureSkipVerify: false
    failIfBodyMatchesRegexp: []
    failIfBodyNotMatchesRegexp: []
    failIfHeaderMatchesRegexp: []
    failIfHeaderNotMatchesRegexp: []
    body: ""
    basicAuth:
      username: user
      passwordRef:
        name: blackbox-secrets
        key: password
    oauth2:
      clientID: my-client
      clientSecretRef:
        name: blackbox-secrets
        key: oauth-secret
      tokenURL: https://auth.example.com/token
      scopes: []
```

### TCP

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: ssh-banner
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  tcp:
    queryResponse:
      - send: ""
        expect: "^SSH-2.0-"
        startTLS: false
    tls: false
    tlsConfig:
      insecureSkipVerify: false
    preferredIPProtocol: ip4
```

### DNS

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: dns-a-record
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  dns:
    queryName: example.com
    queryType: A
    queryClass: IN
    recursionDesired: true
    validRcodes:
      - NOERROR
    validateAnswer:
      failIfMatchesRegexp: []
      failIfNotMatchesRegexp: []
    validateAuthority:
      failIfMatchesRegexp: []
      failIfNotMatchesRegexp: []
    validateAdditional:
      failIfMatchesRegexp: []
      failIfNotMatchesRegexp: []
    transportProtocol: udp
    dnsOverTLS: false
    preferredIPProtocol: ip4
```

### ICMP

> **Note:** ICMP requires `CAP_NET_RAW`. Set `enableICMP: true` on the `BlackboxExporter` to grant this capability. This downgrades the security context from `restricted` to `baseline` Pod Security Standard.

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: ping
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  icmp:
    preferredIPProtocol: ip4
    payloadSize: 56
    dontFragment: false
    ttl: 64
    sourceIPAddress: ""
```

### gRPC

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: grpc-health
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  grpc:
    service: my.service.Name
    tls: false
    tlsConfig:
      insecureSkipVerify: false
    preferredIPProtocol: ip4
```

### Unix Socket

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: unix-check
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  unix:
    queryResponse:
      - send: ""
        expect: ""
```

### WebSocket

```yaml
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: ws-check
  namespace: monitoring
  labels:
    exporter: main
spec:
  timeout: 5s
  websocket:
    headers:
      Authorization: Bearer token
```

## Sensitive Values

Secrets (passwords, tokens) are referenced via `SecretKeySelector`:

```yaml
passwordRef:
  name: my-secret     # Secret name (same namespace as the BlackboxModule)
  key: password        # Key within the Secret
```

The operator reads the secret value and injects it into the rendered configuration. The ConfigMap contains the resolved value -- use sealed-secrets or external-secrets for secret management.

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | string | `5s` | Probe timeout as Go duration |
| `http` | HTTPProbe | - | HTTP prober config |
| `tcp` | TCPProbe | - | TCP prober config |
| `dns` | DNSProbe | - | DNS prober config |
| `icmp` | ICMPProbe | - | ICMP prober config |
| `grpc` | GRPCProbe | - | gRPC prober config |
| `unix` | UnixProbe | - | Unix socket prober config |
| `websocket` | WebSocketProbe | - | WebSocket prober config |

## Status

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: Valid
      message: "Module configuration is valid"
    - type: ConfigValid
      status: "True"
      reason: ProberConfigValid
      message: "HTTP prober configuration validated"
  referencedByExporters:
    - namespace: monitoring
      name: main
  observedGeneration: 2
```

| Status Field | Type | Description |
|-------------|------|-------------|
| `conditions` | []metav1.Condition | Standard conditions (`Ready`, `ConfigValid`) |
| `referencedByExporters` | []NamespacedReference | Exporters that include this module |
| `observedGeneration` | int64 | Last observed `.metadata.generation` |

## Validation Rules

- Exactly one of `http`, `tcp`, `dns`, `icmp`, `grpc`, `unix`, `websocket` must be set
- `spec.timeout` must be a valid Go duration string
