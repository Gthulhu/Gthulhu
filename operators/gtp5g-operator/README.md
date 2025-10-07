# GTP5G Operator

Kubernetes operator for managing gtp5g kernel modules for 5G UPF workloads.

## Quick Start

### Install CRD
```bash
kubectl apply -f config/crd/gtp5gmodule.yaml
```

### Deploy Operator
```bash
kubectl apply -f config/deploy/operator.yaml
```

### Create GTP5GModule
```bash
cat <<EOF | kubectl apply -f -
apiVersion: operator.gthulhu.io/v1alpha1
kind: GTP5GModule
metadata:
  name: upf-gtp5g
spec:
  version: v0.8.3
  nodeSelector:
    gtp5g.gthulhu.io/enabled: "true"
EOF
```

### Label Nodes
```bash
kubectl label node <node-name> gtp5g.gthulhu.io/enabled=true
```

### Check Status
```bash
kubectl get gtp5gmodule
kubectl describe gtp5gmodule upf-gtp5g
```

## Architecture

The operator consists of:
- **CRD**: GTP5GModule - Declarative API for module management
- **Controller**: Reconciles desired state via DaemonSet
- **Installer**: Container that builds and loads gtp5g module

## For free5GC Users

This operator simplifies deploying free5GC UPF on Kubernetes by automating gtp5g installation.

See [Integration Guide](docs/free5gc-integration.md) for details.
