# GTP5G Operator - Build and Test Guide

## Quick Test with Docker

Since you have Docker locally, here's the fastest way to test:

### 1. Start Local Registry (if not running)

```bash
docker run -d -p 5000:5000 --restart=always --name registry registry:2
```

### 2. Build Images

```bash
cd operators/gtp5g-operator

# Build operator
docker build -t localhost:5000/gtp5g-operator:latest .
docker push localhost:5000/gtp5g-operator:latest

# Build installer
cd installer
docker build -t localhost:5000/gtp5g-installer:latest .
docker push localhost:5000/gtp5g-installer:latest
cd ..
```

### 3. Quick Test with kind (Kubernetes in Docker)

```bash
# Create kind cluster if not exists
kind create cluster --name gtp5g-test

# Connect registry to kind network
docker network connect kind registry || true

# Install CRD
kubectl apply -f config/crd/gtp5gmodule.yaml

# Deploy operator
kubectl apply -k config/deploy/

# Wait for operator
kubectl wait --for=condition=available --timeout=60s \
  deployment/gtp5g-operator -n gtp5g-operator-system

# Label a node
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl label node $NODE gtp5g.gthulhu.io/enabled=true

# Create GTP5GModule
kubectl apply -f config/samples/gtp5gmodule_sample.yaml

# Check status
kubectl get gtp5gmodule
kubectl get daemonset
kubectl get pods -l app=gtp5g-installer
```

### 4. Verify

```bash
# Check operator logs
kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager

# Check GTP5GModule status
kubectl describe gtp5gmodule sample-gtp5g

# Check installer pod logs
kubectl logs -l app=gtp5g-installer
```

### 5. Cleanup

```bash
kubectl delete -f config/samples/gtp5gmodule_sample.yaml
kubectl delete -k config/deploy/
kubectl delete -f config/crd/gtp5gmodule.yaml
kind delete cluster --name gtp5g-test
```

## Test via Helm Chart

```bash
cd ../../..  # Back to repo root

# Install with operator enabled
helm install gthulhu ./chart/gthulhu \
  --set gtp5gOperator.enabled=true \
  --set gtp5gOperator.operator.image.repository=localhost:5000/gtp5g-operator \
  --set gtp5gOperator.operator.image.tag=latest \
  --set gtp5gOperator.installer.image.repository=localhost:5000/gtp5g-installer \
  --set gtp5gOperator.installer.image.tag=latest \
  --namespace gthulhu-system \
  --create-namespace

# Label node
kubectl label node <node-name> gtp5g.gthulhu.io/enabled=true

# Create GTP5GModule
cat <<EOF | kubectl apply -f -
apiVersion: operator.gthulhu.io/v1alpha1
kind: GTP5GModule
metadata:
  name: test-gtp5g
spec:
  version: v0.8.3
EOF

# Check
kubectl get gtp5gmodule
helm status gthulhu -n gthulhu-system
```

## Automated Build Script

Save this as `build.sh`:

```bash
#!/bin/bash
set -e

echo "🚀 Building GTP5G Operator..."

# Check Docker
if ! docker ps > /dev/null 2>&1; then
    echo "❌ Docker is not running"
    exit 1
fi

# Build operator
echo "📦 Building operator image..."
cd operators/gtp5g-operator
docker build -t localhost:5000/gtp5g-operator:latest . || {
    echo "❌ Operator build failed"
    exit 1
}
docker push localhost:5000/gtp5g-operator:latest

# Build installer
echo "📦 Building installer image..."
cd installer
docker build -t localhost:5000/gtp5g-installer:latest . || {
    echo "❌ Installer build failed"
    exit 1
}
docker push localhost:5000/gtp5g-installer:latest
cd ../..

echo "✅ Build complete!"
echo ""
echo "Images built:"
echo "  - localhost:5000/gtp5g-operator:latest"
echo "  - localhost:5000/gtp5g-installer:latest"
echo ""
echo "Next steps:"
echo "  1. kubectl apply -f operators/gtp5g-operator/config/crd/gtp5gmodule.yaml"
echo "  2. kubectl apply -k operators/gtp5g-operator/config/deploy/"
echo "  3. kubectl label node <node> gtp5g.gthulhu.io/enabled=true"
echo "  4. kubectl apply -f operators/gtp5g-operator/config/samples/gtp5gmodule_sample.yaml"
```

## Status Check

```bash
# Operator status
kubectl get pods -n gtp5g-operator-system
kubectl get deployment -n gtp5g-operator-system

# GTP5GModule status
kubectl get gtp5gmodule -o wide
kubectl describe gtp5gmodule <name>

# Installer pods
kubectl get ds
kubectl get pods -l app=gtp5g-installer

# Logs
kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager --tail=50
kubectl logs -l app=gtp5g-installer --tail=50
```

## Common Issues

### 1. Image pull fails
```bash
# Check if registry is accessible
docker pull localhost:5000/gtp5g-operator:latest
```

### 2. CRD not found
```bash
kubectl get crd gtp5gmodules.operator.gthulhu.io
kubectl describe crd gtp5gmodules.operator.gthulhu.io
```

### 3. RBAC issues
```bash
kubectl get clusterrole gtp5g-operator-role
kubectl get clusterrolebinding gtp5g-operator-rolebinding
```

## Development Workflow

1. **Make changes** to code
2. **Build images**:
   ```bash
   cd operators/gtp5g-operator
   docker build -t localhost:5000/gtp5g-operator:latest .
   docker push localhost:5000/gtp5g-operator:latest
   ```
3. **Restart operator**:
   ```bash
   kubectl rollout restart deployment/gtp5g-operator -n gtp5g-operator-system
   ```
4. **Test changes**
5. **Commit** when working

## Ready to Push?

After all tests pass:

```bash
git add .
git commit -m "feat(operator): complete gtp5g operator implementation

- Add all missing components (main.go, Makefile, tests, etc.)
- Add Helm templates for operator deployment
- Add deployment configurations and RBAC
- Add build and test scripts
- Complete implementation ready for PR

Closes #11"

git push origin feature/gtp5g-operator
```

Then create PR on GitHub!
