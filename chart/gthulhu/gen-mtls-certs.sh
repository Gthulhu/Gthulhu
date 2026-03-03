#!/usr/bin/env bash
# gen-mtls-certs.sh — Generate a private CA and leaf certificates for Gthulhu mTLS.
#
# Usage:
#   ./gen-mtls-certs.sh [OUTPUT_DIR]
#
# Environment variables:
#   DM_EXTRA_SANS  — Extra SANs for the DM server certificate (comma-separated).
#                    Example: DM_EXTRA_SANS="IP:10.0.0.5,DNS:my-dm.example.com"
#   HELM_RELEASE   — Helm release name (default: gthulhu). Used to build the
#                    headless-service wildcard DNS SAN.
#   NAMESPACE      — Kubernetes namespace (default: default).
#
# Outputs (all PEM-encoded):
#   ca.crt / ca.key           — Private CA
#   dm.crt / dm.key           — Decision Maker (DM sidecar) server certificate
#   manager.crt / manager.key — Manager client certificate
#
# The DM server cert includes SANs for:
#   - localhost / 127.0.0.1          (scheduler → sidecar, same pod)
#   - *.<release>-gthulhu-scheduler-sidecar.<ns>.svc.cluster.local
#                                     (manager → sidecar, cross-node via headless svc DNS)
#   - Any extra SANs from DM_EXTRA_SANS

set -euo pipefail

OUT="${1:-certs}"
mkdir -p "$OUT"

CA_DAYS=3650    # 10 years
LEAF_DAYS=730   # 2 years

RELEASE="${HELM_RELEASE:-gthulhu}"
NS="${NAMESPACE:-default}"
HEADLESS_SVC="${RELEASE}-gthulhu-scheduler-sidecar"

# Build DM SAN string
DM_SANS="DNS:localhost,IP:127.0.0.1"
DM_SANS="${DM_SANS},DNS:*.${HEADLESS_SVC}.${NS}.svc.cluster.local"
DM_SANS="${DM_SANS},DNS:*.${HEADLESS_SVC}.${NS}.svc"
DM_SANS="${DM_SANS},DNS:${HEADLESS_SVC}.${NS}.svc.cluster.local"
if [[ -n "${DM_EXTRA_SANS:-}" ]]; then
  DM_SANS="${DM_SANS},${DM_EXTRA_SANS}"
fi

echo "==> DM certificate SANs: ${DM_SANS}"
echo ""

echo "==> Generating private CA …"
openssl ecparam -name prime256v1 -genkey -noout -out "$OUT/ca.key"
openssl req -new -x509 -days "$CA_DAYS" \
  -key "$OUT/ca.key" \
  -out "$OUT/ca.crt" \
  -subj "/CN=Gthulhu-Private-CA"

echo "==> Generating DM sidecar server certificate …"
openssl ecparam -name prime256v1 -genkey -noout -out "$OUT/dm.key"
openssl req -new \
  -key "$OUT/dm.key" \
  -out "$OUT/dm.csr" \
  -subj "/CN=gthulhu-decisionmaker"
openssl x509 -req -days "$LEAF_DAYS" \
  -in "$OUT/dm.csr" \
  -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" -CAcreateserial \
  -extfile <(printf "subjectAltName=${DM_SANS}\nextendedKeyUsage=serverAuth,clientAuth") \
  -out "$OUT/dm.crt"

echo "==> Generating Manager client certificate …"
openssl ecparam -name prime256v1 -genkey -noout -out "$OUT/manager.key"
openssl req -new \
  -key "$OUT/manager.key" \
  -out "$OUT/manager.csr" \
  -subj "/CN=gthulhu-manager"
openssl x509 -req -days "$LEAF_DAYS" \
  -in "$OUT/manager.csr" \
  -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" -CAcreateserial \
  -extfile <(printf "extendedKeyUsage=clientAuth") \
  -out "$OUT/manager.crt"

# Clean up CSRs
rm -f "$OUT"/*.csr "$OUT"/*.srl

echo ""
echo "✅  Certificates generated in $OUT/"
echo ""
echo "Files:"
ls -1 "$OUT"
echo ""
echo "To install with mTLS enabled:"
echo "  helm install gthulhu ./gthulhu \\"
echo "    --set mtls.enabled=true \\"
echo "    --set-file mtls.ca.cert=$OUT/ca.crt \\"
echo "    --set-file mtls.dm.cert=$OUT/dm.crt \\"
echo "    --set-file mtls.dm.key=$OUT/dm.key \\"
echo "    --set-file mtls.manager.cert=$OUT/manager.crt \\"
echo "    --set-file mtls.manager.key=$OUT/manager.key"
