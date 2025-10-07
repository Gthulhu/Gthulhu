#!/bin/bash
set -e

echo "Deploying GTP5G Operator locally..."

# Install CRD
echo "Installing CRD..."
kubectl apply -f config/crd/gtp5gmodule.yaml

# Deploy operator
echo "Deploying operator..."
kubectl apply -k config/deploy/

echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=60s \
  deployment/gtp5g-operator -n gtp5g-operator-system

echo "✅ GTP5G Operator deployed successfully!"
echo ""
echo "Check status:"
echo "  kubectl get pods -n gtp5g-operator-system"
echo "  kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager -f"
