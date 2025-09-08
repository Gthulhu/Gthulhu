#!/bin/bash

# gthulhu snap launcher script
# This script ensures proper environment setup for the Gthulhu scheduler

set -e

# Check if running with sufficient privileges
if [ "$EUID" -ne 0 ]; then
    echo "Error: Gthulhu scheduler requires root privileges to load BPF programs"
    echo "Please run with: sudo snap run gthulhu"
    exit 1
fi

# Check kernel version and sched_ext support
KERNEL_VERSION=$(uname -r)
MAJOR_VERSION=$(echo $KERNEL_VERSION | cut -d. -f1)
MINOR_VERSION=$(echo $KERNEL_VERSION | cut -d. -f2)

if [ "$MAJOR_VERSION" -lt 6 ] || ([ "$MAJOR_VERSION" -eq 6 ] && [ "$MINOR_VERSION" -lt 12 ]); then
    echo "Warning: Kernel version $KERNEL_VERSION detected."
    echo "Gthulhu requires Linux kernel 6.12+ with sched_ext support."
    echo "Please upgrade your kernel or ensure sched_ext is enabled."
fi

# Check if sched_ext is available
if [ ! -f /sys/kernel/sched_ext/enable ]; then
    echo "Error: sched_ext interface not found at /sys/kernel/sched_ext/enable"
    echo "Please ensure your kernel has CONFIG_SCHED_CLASS_EXT=y enabled."
    exit 1
fi

# Set up configuration path
CONFIG_PATH="$SNAP_USER_DATA/config.yaml"
if [ ! -f "$CONFIG_PATH" ]; then
    echo "Creating default configuration at $CONFIG_PATH"
    mkdir -p "$SNAP_USER_DATA"
    cp "$SNAP/etc/gthulhu/config.yaml" "$CONFIG_PATH"
fi

# Launch the scheduler
exec "$SNAP/bin/gthulhu" -config "$CONFIG_PATH" "$@"