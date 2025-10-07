# GTP5G Operator - Quick Start Guide

## Overview

The GTP5G Operator automates the installation and management of the gtp5g kernel module for 5G User Plane Function (UPF) workloads running on Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Linux kernel 5.0+ on worker nodes
- Node must have kernel headers installed
- Docker for building installer image

## Installation

### Option 1: Using Helm (Recommended)

Enable the operator in Gthulhu Helm chart:

```bash
helm install gthulhu ./chart/gthulhu \
  --set gtp5gOperator.enabled=true \
  --namespace gthulhu-system \
  --create-namespace
```

### Option 2: Manual Installation

1. **Install CRD**
```bash
kubectl apply -f operators/gtp5g-operator/config/crd/gtp5gmodule.yaml
```

2. **Build and Push Installer Image**
```bash
cd operators/gtp5g-operator/installer
docker build -t localhost:5000/gtp5g-installer:latest .
docker push localhost:5000/gtp5g-installer:latest
```

3. **Deploy Operator**
```bash
# Deploy will be handled by Helm in production
kubectl apply -f config/deploy/
```

## Usage

### 1. Label Target Nodes

Label nodes where you want to install gtp5g:

```bash
kubectl label node worker1 gtp5g.gthulhu.io/enabled=true
kubectl label node worker2 gtp5g.gthulhu.io/enabled=true
```

### 2. Create GTP5GModule Resource

```yaml
apiVersion: operator.gthulhu.io/v1alpha1
kind: GTP5GModule
metadata:
  name: upf-gtp5g
spec:
  version: v0.8.3
  # Optional: specify kernel version
  # kernelVersion: "5.15.0-56-generic"
  # Optional: override installer image
  # image: "custom-registry/gtp5g-installer:v1.0"
```

Apply it:
```bash
kubectl apply -f gtp5gmodule.yaml
```

### 3. Verify Installation

Check GTP5GModule status:
```bash
kubectl get gtp5gmodule
```

Expected output:
```
NAME        VERSION   PHASE       INSTALLED   AGE
upf-gtp5g   v0.8.3    Installed   2           5m
```

Check detailed status:
```bash
kubectl describe gtp5gmodule upf-gtp5g
```

Check installer pods:
```bash
kubectl get pods -l app=gtp5g-installer
```

### 4. Verify Module is Loaded

Exec into an installer pod:
```bash
kubectl exec -it <installer-pod> -- lsmod | grep gtp5g
```

## Troubleshooting

### Module Compilation Fails

**Symptom**: Installer pod crashes with compilation errors

**Solution**: Ensure node has correct kernel headers:
```bash
# On the node
apt-get install linux-headers-$(uname -r)
```

### Module Not Loading

**Symptom**: Module compiles but doesn't load

**Solution**: Check installer logs:
```bash
kubectl logs <installer-pod>
```

Common issues:
- Kernel version mismatch
- Missing dependencies
- SELinux/AppArmor restrictions

### DaemonSet Not Created

**Symptom**: No installer pods created

**Solution**: Check operator logs and GTP5GModule status

### Node Not Selected

**Symptom**: Some nodes don't have installer pods

**Solution**: Verify node labels:
```bash
kubectl get nodes --show-labels | grep gtp5g
```

## Integration with free5GC

After installing gtp5g, you can deploy free5GC UPF:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: free5gc-upf
spec:
  hostNetwork: true
  containers:
  - name: upf
    image: free5gc/upf:v3.3.0
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]
    # UPF configuration...
```

## Cleanup

Delete GTP5GModule:
```bash
kubectl delete gtp5gmodule upf-gtp5g
```

This will automatically:
1. Delete installer DaemonSet
2. Unload gtp5g module from nodes (on pod termination)

## Advanced Configuration

### Custom Installer Image

```yaml
spec:
  version: v0.8.3
  image: "my-registry/custom-gtp5g-installer:latest"
```

### Specific Kernel Version

```yaml
spec:
  version: v0.8.3
  kernelVersion: "5.15.0-56-generic"
```

### Node Affinity

Use nodeSelector for fine-grained control:
```yaml
spec:
  version: v0.8.3
  nodeSelector:
    kubernetes.io/hostname: "specific-node"
    node-role.kubernetes.io/worker: ""
```

## Next Steps

- [Architecture Documentation](gtp5g-operator-architecture.md)
- [API Reference](gtp5g-operator-api.md)
- [free5GC Integration Guide](free5gc-integration.md)
- [Troubleshooting Guide](gtp5g-operator-troubleshooting.md)

## Support

For issues and questions:
- GitHub Issues: https://github.com/Gthulhu/Gthulhu/issues
- Discussions: https://github.com/Gthulhu/Gthulhu/discussions
