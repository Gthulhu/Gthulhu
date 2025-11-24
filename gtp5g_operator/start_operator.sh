#!/bin/bash
# Start GTP5G Operator with K8s Gthulhu API connection
# 
# Prerequisites:
# 1. Port-forward must be running:
#    sudo microk8s.kubectl port-forward gthulhu-api-pod 8081:8080
# 2. K8s public key must be available

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_PUBLIC_KEY="/tmp/k8s_jwt_public_key.pem"
API_ENDPOINT="http://localhost:8081"

echo "=========================================="
echo "   GTP5G Operator - K8s Connection"
echo "=========================================="
echo ""

# Step 1: Get K8s public key
echo "[1/3] Fetching K8s Gthulhu public key..."
POD_NAME=$(sudo microk8s.kubectl get pods -l app=gthulhu-api -o jsonpath='{.items[0].metadata.name}')
if [ -z "$POD_NAME" ]; then
    echo "ERROR: Cannot find gthulhu-api pod"
    exit 1
fi

sudo microk8s.kubectl exec "$POD_NAME" -- cat /app/jwt_public_key.pem > "$K8S_PUBLIC_KEY"
echo "✓ Public key saved to $K8S_PUBLIC_KEY"
echo ""

# Step 2: Check port-forward
echo "[2/3] Checking port-forward status..."
if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "⚠ WARNING: Port-forward may not be running"
    echo "   Please run in another terminal:"
    echo "   sudo microk8s.kubectl port-forward $POD_NAME 8081:8080"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✓ Port-forward is active"
fi
echo ""

# Step 3: Start operator
echo "[3/3] Starting GTP5G Operator..."
echo "   API Endpoint: $API_ENDPOINT"
echo "   Public Key: $K8S_PUBLIC_KEY"
echo ""
echo "Press Ctrl+C to stop"
echo "=========================================="
echo ""

cd "$SCRIPT_DIR"
sudo API_ENDPOINT="$API_ENDPOINT" \
     PUBLIC_KEY_PATH="$K8S_PUBLIC_KEY" \
     ./gtp5g_operator
