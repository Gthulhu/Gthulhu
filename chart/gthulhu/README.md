# Gthulhu Helm Chart

This Helm chart deploys Gthulhu BPF Scheduler and BSS Metrics API Server to Kubernetes.

## Overview

Gthulhu optimizes cloud-native workloads using the Linux Scheduler Extension (sched-ext) for different application scenarios. This chart deploys:

1. **Gthulhu Scheduler**: A BPF-based scheduler that runs as a DaemonSet on all nodes
2. **BSS Metrics API Server**: A REST API service for collecting and processing scheduler metrics

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Linux kernel 6.12+ with sched-ext support on worker nodes
- Container runtime with BPF capabilities

## Installation

### Add the Helm repository (if applicable)

```bash
helm repo add gthulhu https://your-helm-repo.com
helm repo update
```

### Install the chart

```bash
# Install with default values
helm install gthulhu gthulhu/gthulhu

# Install with custom values
helm install gthulhu gthulhu/gthulhu -f custom-values.yaml

# Install from local chart
helm install gthulhu ./gthulhu
```

## Configuration

The following table lists the configurable parameters and their default values:

### Global Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imagePullSecrets` | Image pull secrets | `[]` |
| `global.nameOverride` | Override the name | `""` |
| `global.fullnameOverride` | Override the full name | `""` |

### Scheduler Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `scheduler.enabled` | Enable the BPF scheduler | `true` |
| `scheduler.image.repository` | Scheduler image repository | `gthulhu` |
| `scheduler.image.tag` | Scheduler image tag | `latest` |
| `scheduler.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `scheduler.hostPID` | Use host PID namespace | `true` |
| `scheduler.resources.limits.cpu` | CPU limit | `500m` |
| `scheduler.resources.limits.memory` | Memory limit | `512Mi` |
| `scheduler.resources.requests.cpu` | CPU request | `100m` |
| `scheduler.resources.requests.memory` | Memory request | `128Mi` |

### API Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `api.enabled` | Enable the metrics API server | `true` |
| `api.replicaCount` | Number of API server replicas | `1` |
| `api.image.repository` | API server image repository | `gthulhu-api` |
| `api.image.tag` | API server image tag | `latest` |
| `api.port` | API server port | `8080` |
| `api.service.type` | Service type | `ClusterIP` |
| `api.service.port` | Service port | `80` |
| `api.ingress.enabled` | Enable ingress | `false` |
| `api.healthCheck.enabled` | Enable health checks | `true` |
| `api.autoscaling.enabled` | Enable HPA | `false` |

### Monitoring Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `monitoring.enabled` | Enable monitoring | `false` |
| `monitoring.serviceMonitor.enabled` | Enable ServiceMonitor for Prometheus | `false` |

## Examples

### Basic Installation

```bash
helm install gthulhu ./gthulhu
```

### Production Installation with Custom Values

```yaml
# production-values.yaml
scheduler:
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 256Mi

api:
  replicaCount: 3
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: gthulhu-api.example.com
        paths:
          - path: /
            pathType: Prefix
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70

monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
```

```bash
helm install gthulhu ./gthulhu -f production-values.yaml
```

### Development Installation (API only)

```yaml
# dev-values.yaml
scheduler:
  enabled: false

api:
  enabled: true
  service:
    type: NodePort
```

```bash
helm install gthulhu-dev ./gthulhu -f dev-values.yaml
```

## Accessing the API

### Using port-forward (ClusterIP)

```bash
kubectl port-forward svc/gthulhu-api 8080:80
curl http://localhost:8080/health
```

### Using NodePort

```bash
export NODE_PORT=$(kubectl get svc gthulhu-api -o jsonpath='{.spec.ports[0].nodePort}')
export NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')
curl http://$NODE_IP:$NODE_PORT/health
```

### Using Ingress

```bash
curl http://gthulhu-api.example.com/health
```

## API Endpoints

- `POST /api/v1/metrics` - Submit BSS metrics data
- `GET /health` - Health check endpoint
- `GET /` - API information

## Monitoring

When monitoring is enabled, the chart creates a ServiceMonitor that can be picked up by Prometheus Operator to scrape metrics from the API server.

## Troubleshooting

### Scheduler Issues

Check if the scheduler is running:

```bash
kubectl get daemonset gthulhu-scheduler
kubectl logs -l app.kubernetes.io/component=scheduler
```

### API Server Issues

Check API server logs:

```bash
kubectl logs -l app.kubernetes.io/component=api
```

### Common Issues

1. **Kernel Compatibility**: Ensure your nodes have Linux kernel 6.12+ with sched-ext support
2. **BPF Capabilities**: Verify that your container runtime supports BPF operations
3. **Privileges**: The scheduler requires privileged access and host PID namespace

## Uninstallation

```bash
helm uninstall gthulhu
```

## mTLS (Mutual TLS)

Gthulhu supports **mutual TLS** to authenticate and encrypt traffic on two communication paths:

| Path | Client | Server | Notes |
|------|--------|--------|-------|
| Manager → DM sidecar | Manager (Deployment) | DM sidecar (DaemonSet) | Cross-node; protects scheduling intents |
| Scheduler → DM sidecar | Scheduler (same Pod) | DM sidecar (same Pod) | Loopback; protects the local API call |

Both paths share a single **private CA** — every certificate is signed by this CA so each peer can verify the other.

### Quick Start

```bash
# 1. Generate a private CA + leaf certificates
./gen-mtls-certs.sh certs

# 2. Install the chart with mTLS enabled
helm install gthulhu ./gthulhu \
  --set mtls.enabled=true \
  --set-file mtls.ca.cert=certs/ca.crt \
  --set-file mtls.dm.cert=certs/dm.crt \
  --set-file mtls.dm.key=certs/dm.key \
  --set-file mtls.manager.cert=certs/manager.crt \
  --set-file mtls.manager.key=certs/manager.key
```

### Using Your Own Certificates

If you already have a PKI or want to bring your own certificates, follow the steps below.

#### 1. Create a Private CA

```bash
# EC P-256 key (recommended); RSA-4096 also works
openssl ecparam -name prime256v1 -genkey -noout -out ca.key

# Self-signed CA certificate (10-year validity)
openssl req -new -x509 -days 3650 \
  -key ca.key -out ca.crt \
  -subj "/CN=Gthulhu-Private-CA"
```

#### 2. Generate the DM Sidecar Server Certificate

The DM sidecar is the TLS **server**. Its certificate needs a `subjectAltName` that covers
`localhost` and `127.0.0.1` (so the in-pod scheduler can connect) plus any DNS names
the Manager uses to reach it (e.g. `*.svc.cluster.local`).

```bash
openssl ecparam -name prime256v1 -genkey -noout -out dm.key

openssl req -new -key dm.key -out dm.csr \
  -subj "/CN=gthulhu-decisionmaker"

openssl x509 -req -days 730 \
  -in dm.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:*.svc.cluster.local\nextendedKeyUsage=serverAuth,clientAuth") \
  -out dm.crt
```

#### 3. Generate the Manager / Scheduler Client Certificate

The Manager and the Scheduler both act as mTLS **clients** when talking to the DM sidecar.
They share the same client certificate.

```bash
openssl ecparam -name prime256v1 -genkey -noout -out manager.key

openssl req -new -key manager.key -out manager.csr \
  -subj "/CN=gthulhu-manager"

openssl x509 -req -days 730 \
  -in manager.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -extfile <(printf "extendedKeyUsage=clientAuth") \
  -out manager.crt
```

#### 4. Supply the Certificates to the Chart

**Option A — inline via `--set-file`:**

```bash
helm install gthulhu ./gthulhu \
  --set mtls.enabled=true \
  --set-file mtls.ca.cert=ca.crt \
  --set-file mtls.dm.cert=dm.crt \
  --set-file mtls.dm.key=dm.key \
  --set-file mtls.manager.cert=manager.crt \
  --set-file mtls.manager.key=manager.key
```

**Option B — pre-created Kubernetes Secret:**

```bash
kubectl create secret generic my-gthulhu-mtls \
  --from-file=ca.crt \
  --from-file=dm.crt \
  --from-file=dm.key \
  --from-file=manager.crt \
  --from-file=manager.key

helm install gthulhu ./gthulhu \
  --set mtls.enabled=true \
  --set mtls.existingSecret=my-gthulhu-mtls
```

### Certificate Rotation

Because the ConfigMap and the scheduler mTLS config Secret are created with
`immutable: true`, you must use `helm upgrade --force` (which deletes and
recreates immutable resources) when rotating certificates:

```bash
helm upgrade gthulhu ./gthulhu --force \
  --set mtls.enabled=true \
  --set-file mtls.ca.cert=new-ca.crt \
  --set-file mtls.dm.cert=new-dm.crt \
  --set-file mtls.dm.key=new-dm.key \
  --set-file mtls.manager.cert=new-manager.crt \
  --set-file mtls.manager.key=new-manager.key
```

### mTLS Configuration Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `mtls.enabled` | Enable mutual TLS | `false` |
| `mtls.existingSecret` | Name of a pre-created Secret containing all PEM files | `""` |
| `mtls.ca.cert` | PEM-encoded CA certificate | `""` |
| `mtls.dm.cert` | PEM-encoded DM sidecar server certificate | `""` |
| `mtls.dm.key` | PEM-encoded DM sidecar server private key | `""` |
| `mtls.manager.cert` | PEM-encoded Manager/Scheduler client certificate | `""` |
| `mtls.manager.key` | PEM-encoded Manager/Scheduler client private key | `""` |

### Architecture Notes

- The Manager's **external HTTP API** (web GUI / Ingress) remains **plain HTTP**.
  Use a Kubernetes Ingress with TLS termination for external HTTPS.
- When mTLS is enabled, health-check probes on the DM sidecar switch from
  `httpGet` to `tcpSocket` because the kubelet cannot present a client certificate.
- The scheduler config is stored in a Kubernetes **Secret** (not a ConfigMap)
  when mTLS is enabled, because it contains private-key material inline.

## Development

### Testing the Chart

```bash
# Lint the chart
helm lint ./gthulhu

# Dry run installation
helm install gthulhu ./gthulhu --dry-run --debug

# Template rendering
helm template gthulhu ./gthulhu
```

## Contributing

Please read the main project [README](https://github.com/Gthulhu/Gthulhu) for contribution guidelines.

## License

This software is distributed under the terms of the Apache License 2.0.
