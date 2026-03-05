# Get the project root directory (3 levels up from deployment/kind/setup.sh)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"


CLUSTER="gthulhu-api-local"
NS="gthulhu-api-local"

echo "Tearing down local kind cluster..."
if kind get clusters | grep -qx "$CLUSTER"; then
  echo "Deleting cluster '$CLUSTER'..."
  kind delete cluster --name "$CLUSTER"
else
  echo "Cluster '$CLUSTER' does not exist. Nothing to delete."
fi