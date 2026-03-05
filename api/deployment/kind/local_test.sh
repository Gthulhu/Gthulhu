#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:8080"
USERNAME="admin@example.com"
PASSWORD="your-password-here"

echo "[1] Login..."

LOGIN_RESP=$(curl -s -X POST \
  "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -H "accept: application/json" \
  -d "{
    \"username\": \"$USERNAME\",
    \"password\": \"$PASSWORD\"
  }")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.token')

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "❌ Failed to get token"
  echo "$LOGIN_RESP"
  exit 1
fi

echo "✅ Token acquired"

echo "[2] Create strategy..."

curl -s -X POST \
  "$BASE_URL/api/v1/strategies" \
  -H "Content-Type: application/json" \
  -H "accept: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "labelSelectors": [
      {
        "key": "app",
        "value": "busybox"
      }
    ],
    "priority": 1
  }' | jq
echo "✅ Strategy created"