#!/bin/bash
set -e

echo "Building GTP5G Operator Images..."

# Build operator image
echo "Building operator image..."
docker build -t localhost:5000/gtp5g-operator:latest -f Dockerfile .
echo "Pushing operator image..."
docker push localhost:5000/gtp5g-operator:latest

# Build installer image
echo "Building installer image..."
cd installer
docker build -t localhost:5000/gtp5g-installer:latest .
echo "Pushing installer image..."
docker push localhost:5000/gtp5g-installer:latest
cd ..

echo "??All images built and pushed successfully!"
echo ""
echo "Images:"
echo "  - localhost:5000/gtp5g-operator:latest"
echo "  - localhost:5000/gtp5g-installer:latest"
