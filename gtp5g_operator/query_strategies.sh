#!/bin/bash
# Query Gthulhu scheduling strategies with JWT authentication
# Usage: ./query_strategies.sh

API_ENDPOINT="http://localhost:8080"
PUBLIC_KEY_PATH="/tmp/jwt_public_key.pem"

echo "Generating JWT token..."
TOKEN_RESPONSE=$(jq -n --arg pk "$(cat $PUBLIC_KEY_PATH)" '{public_key: $pk}' | \
  curl -s -X POST "$API_ENDPOINT/api/v1/auth/token" \
    -H "Content-Type: application/json" \
    -d @-)

TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
    echo "Failed to get token:"
    echo "$TOKEN_RESPONSE" | jq '.'
    exit 1
fi

echo "Token obtained successfully"
echo ""
echo "Querying scheduling strategies..."
echo ""

curl -s -X GET "$API_ENDPOINT/api/v1/scheduling/strategies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq '.'
