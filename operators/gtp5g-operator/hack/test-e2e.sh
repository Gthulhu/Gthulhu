#!/bin/bash
set -e

echo "Running E2E Test for GTP5G Operator..."

# Label a node
echo "Labeling node for testing..."
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl label node $NODE gtp5g.gthulhu.io/enabled=true --overwrite

# Create GTP5GModule
echo "Creating GTP5GModule..."
kubectl apply -f config/samples/gtp5gmodule_sample.yaml

# Wait for installation
echo "Waiting for installation to complete..."
sleep 10

# Check DaemonSet
echo "Checking DaemonSet..."
kubectl get daemonset

# Check pods
echo "Checking installer pods..."
kubectl get pods -l app=gtp5g-installer

# Check GTP5GModule status
echo "Checking GTP5GModule status..."
kubectl get gtp5gmodule
kubectl describe gtp5gmodule sample-gtp5g

echo "✅ E2E Test completed!"
echo ""
echo "Cleanup:"
echo "  kubectl delete -f config/samples/gtp5gmodule_sample.yaml"
echo "  kubectl label node $NODE gtp5g.gthulhu.io/enabled-"
