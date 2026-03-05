# Get the project root directory (3 levels up from deployment/kind/setup.sh)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"


CLUSTER="gthulhu-api-local"
NS="gthulhu-api-local"

echo "Project root directory: $PROJECT_ROOT"

echo "Setting up local kind cluster..."
# Verify if 'kind' is available
if command -v kind >/dev/null 2>&1; then
    echo "kind found: $(kind version)"
else
    echo "kind not found; will install via 'go install' next"
    go install sigs.k8s.io/kind@v0.30.0
fi

if ! kind get clusters | grep -qx "$CLUSTER"; then
  echo "Cluster '$CLUSTER' does not exist. Creating..."
  kind create cluster --name "$CLUSTER"
else
  echo "Cluster '$CLUSTER' already exists."
fi

docker build -f  $PROJECT_ROOT/Dockerfile -t gthulhu-api:local .

docker pull mongo:8.2.2
kind load docker-image mongo:8.2.2 --name "$CLUSTER"
kind load docker-image gthulhu-api:local --name "$CLUSTER"

kubectl get ns "$NS" >/dev/null 2>&1 || kubectl create ns "$NS"

kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/mongo/secret.yaml" 
kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/mongo/service.yaml" 
kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/mongo/statefulset.yaml"

# Wait for MongoDB StatefulSet to be ready
echo "Waiting for MongoDB StatefulSet to be ready..."
STS_NAME=""
for i in {1..30}; do
  STS_NAME=$(kubectl -n "$NS" get statefulset -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  [ -n "$STS_NAME" ] && break
  sleep 2
done
if [ -z "$STS_NAME" ]; then
  echo "Failed to discover StatefulSet in namespace '$NS'"
  exit 1
fi
kubectl -n "$NS" rollout status "statefulset/$STS_NAME" --timeout=5m

echo "deploy mongo"

kubectl apply -f "$PROJECT_ROOT/deployment/kind/decisonmaker/service.yaml" 
kubectl apply -f "$PROJECT_ROOT/deployment/kind/decisonmaker/daemonset.yaml"

echo "deploy decisionmaker"

kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/pod/busybox.yaml"

echo "deploy busybox pods"

kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/manager/service.yaml"
kubectl apply -n "$NS" -f "$PROJECT_ROOT/deployment/kind/manager/deployment.yaml"

echo "Waiting for manager Deployment to be ready..."
DEPLOYMENT_NAME=""
for i in {1..30}; do
  DEPLOYMENT_NAME=$(kubectl -n "$NS" get deployment -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  [ -n "$DEPLOYMENT_NAME" ] && break
  sleep 2
done
if [ -z "$DEPLOYMENT_NAME" ]; then
  echo "Failed to discover Deployment in namespace '$NS'"
  exit 1
fi
kubectl -n "$NS" rollout status "deployment/$DEPLOYMENT_NAME" --timeout=5m

kubectl port-forward -n "$NS" svc/manager 8080:8080 &

echo "Go to http://localhost:8080/swagger/index.html to access the Swagger UI for the manager API."