#!/bin/bash
set -e

echo "========================================="
echo "GTP5G Kernel Module Installer"
echo "========================================="

# Get environment variables
GTP5G_VERSION=${GTP5G_VERSION:-v0.8.3}
KERNEL_VERSION=${KERNEL_VERSION:-$(uname -r)}

echo "GTP5G Version: $GTP5G_VERSION"
echo "Kernel Version: $KERNEL_VERSION"

# Check if module is already loaded
if lsmod | grep -q gtp5g; then
    echo "gtp5g module is already loaded"
    # Keep container running for monitoring
    while true; do
        sleep 30
        if ! lsmod | grep -q gtp5g; then
            echo "gtp5g module unloaded, reloading..."
            modprobe gtp5g || echo "Failed to reload gtp5g"
        fi
    done
fi

# Clone gtp5g repository
echo "Cloning gtp5g repository..."
cd /workspace
rm -rf gtp5g
git clone -b $GTP5G_VERSION --depth 1 https://github.com/free5gc/gtp5g.git
cd gtp5g

# Build module
echo "Building gtp5g module..."
make KVER=$KERNEL_VERSION

# Install module
echo "Installing gtp5g module..."
make install

# Load module
echo "Loading gtp5g module..."
modprobe gtp5g

echo "gtp5g module installed and loaded successfully!"

# Monitor module status
echo "Monitoring gtp5g module status..."
while true; do
    sleep 30
    if ! lsmod | grep -q gtp5g; then
        echo "gtp5g module unloaded, reloading..."
        modprobe gtp5g || echo "Failed to reload gtp5g"
    fi
done
