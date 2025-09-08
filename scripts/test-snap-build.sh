#!/bin/bash

# Snap build test script for Gthulhu
# This script helps test the snap build process locally

set -e

echo "=== Gthulhu Snap Build Test ==="

# Check if snapcraft is installed
if ! command -v snapcraft &> /dev/null; then
    echo "Error: snapcraft is not installed"
    echo "Install with: sudo snap install snapcraft --classic"
    exit 1
fi

# Check if we're in the right directory
if [ ! -f "snapcraft.yaml" ]; then
    echo "Error: snapcraft.yaml not found"
    echo "Please run this script from the Gthulhu repository root"
    exit 1
fi

# Clean any previous builds
echo "Cleaning previous builds..."
snapcraft clean

# Validate snapcraft.yaml
echo "Validating snapcraft.yaml..."
snapcraft list-plugins > /dev/null

# Build the snap
echo "Building snap package..."
echo "Note: This may take several minutes due to BPF compilation and dependency building"
snapcraft --verbose

# List the generated snap
echo "Build completed successfully!"
ls -la *.snap

echo ""
echo "To test the snap locally:"
echo "  sudo snap install --dangerous gthulhu_*.snap"
echo ""
echo "To run the scheduler:"
echo "  sudo snap run gthulhu"
echo ""
echo "To view snap info:"
echo "  snap info --local gthulhu_*.snap"