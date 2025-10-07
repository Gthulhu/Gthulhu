# GTP5G Operator - Quick Start

## Prerequisites

- Docker running
- kubectl configured
- Kubernetes cluster (kind/minikube/real cluster)
- Local registry at localhost:5000 (optional)

## Build and Deploy (Local Testing)

### Step 1: Build Images

```bash
cd operators/gtp5g-operator

# Build operator image
docker build -t localhost:5000/gtp5g-operator:latest .
docker push localhost:5000/gtp5g-operator:latest

# Build installer image
cd installer
docker build -t localhost:5000/gtp5g-installer:latest .
docker push localhost:5000/gtp5g-installer:latest
cd ..
```

Or use the helper script:
```bash
bash hack/build-images.sh
```

### Step 2: Install CRD

```bash
kubectl apply -f config/crd/gtp5gmodule.yaml
```

### Step 3: Deploy Operator

```bash
kubectl apply -k config/deploy/
```

Or use the helper script:
```bash
bash hack/deploy-local.sh
```

### Step 4: Verify Deployment

```bash
kubectl get pods -n gtp5g-operator-system
kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager -f
```

### Step 5: Label Node and Create GTP5GModule

```bash
# Label a node
kubectl label node <node-name> gtp5g.gthulhu.io/enabled=true

# Create GTP5GModule
kubectl apply -f config/samples/gtp5gmodule_sample.yaml

# Check status
kubectl get gtp5gmodule
kubectl describe gtp5gmodule sample-gtp5g

# Check installer pods
kubectl get pods -l app=gtp5g-installer
```

## Deploy via Helm (Production)

```bash
cd ../../..  # Back to repo root

helm install gthulhu ./chart/gthulhu \
  --set gtp5gOperator.enabled=true \
  --set gtp5gOperator.operator.image.repository=localhost:5000/gtp5g-operator \
  --set gtp5gOperator.installer.image.repository=localhost:5000/gtp5g-installer \
  --namespace gthulhu-system \
  --create-namespace
```

## Testing

### Run Unit Tests

```bash
cd operators/gtp5g-operator
make test
```

### Run E2E Test

```bash
bash hack/test-e2e.sh
```

## Cleanup

```bash
# Delete GTP5GModule
kubectl delete -f config/samples/gtp5gmodule_sample.yaml

# Delete operator
kubectl delete -k config/deploy/

# Delete CRD
kubectl delete -f config/crd/gtp5gmodule.yaml

# Or via Helm
helm uninstall gthulhu -n gthulhu-system
```

## Development

### Build Binary

```bash
make build
./bin/manager --help
```

### Run Locally (outside cluster)

```bash
# Install CRD first
kubectl apply -f config/crd/gtp5gmodule.yaml

# Run
make run
```

### Update Go Dependencies

```bash
go mod tidy
go mod vendor
```

## Troubleshooting

### Operator not starting

```bash
kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager
```

### Installer failing

```bash
kubectl get pods -l app=gtp5g-installer
kubectl logs <installer-pod>
```

### Module not loading

Check node has kernel headers:
```bash
kubectl exec -it <installer-pod> -- bash
apt list --installed | grep linux-headers
```

## Next Steps

- Read [Development Plan](../../GTP5G_OPERATOR_DEVELOPMENT_PLAN.md)
- Read [Architecture Documentation](../../docs/gtp5g-operator-quickstart.md)
- Check [free5GC Integration Guide](../../docs/gtp5g-operator-quickstart.md#integration-with-free5gc)
