#!/bin/bash
# Schtest integration script
# This script runs schtest test cases against the Gthulhu scheduler
# It starts the Gthulhu scheduler first, then runs schtest against it

set -euo pipefail

SCHTEST_DIR="schtest"
SCHEDULER_BINARY_DEFAULT="./main"
LOGFILE="/tmp/schtest_scheduler.log"
WARMUP_TIME=15
MAX_RETRIES=3

# Get absolute path of scheduler binary
if [ -f "${SCHEDULER_BINARY_DEFAULT}" ]; then
    SCHEDULER_BINARY=$(cd "$(dirname "${SCHEDULER_BINARY_DEFAULT}")" && pwd)/$(basename "${SCHEDULER_BINARY_DEFAULT}")
else
    SCHEDULER_BINARY="${SCHEDULER_BINARY_DEFAULT}"
fi

echo "Starting schtest integration test..."
echo "Using SCHEDULER_BINARY=${SCHEDULER_BINARY}"

# Check if schtest directory exists
if [ ! -d "${SCHTEST_DIR}" ]; then
    echo "✗ Schtest directory not found. Please run 'make schtest-dep' first."
    exit 1
fi

# Check if scheduler binary exists
if [ ! -f "${SCHEDULER_BINARY}" ]; then
    echo "✗ Scheduler binary not found: ${SCHEDULER_BINARY}"
    echo "  Please run 'make build' first."
    exit 1
fi

# Try to build debug version of schtest if it doesn't exist
if [ ! -f "${SCHTEST_DIR}/target/debug/schtest" ]; then
    echo "Debug version of schtest not found, attempting to build..."
    if [ -f "${SCHTEST_DIR}/Cargo.toml" ] && command -v cargo >/dev/null 2>&1; then
        echo "Building schtest in debug mode..."
        cd "${SCHTEST_DIR}"
        if cargo build 2>&1; then
            echo "✓ Successfully built schtest debug version"
        else
            echo "⚠ Failed to build schtest debug version, will try release version"
        fi
        cd - > /dev/null
    else
        echo "⚠ Cannot build debug version (no Cargo.toml or cargo not available)"
    fi
fi

# Check for schtest binary - prefer debug, then release, then other locations
SCHTEST_BIN=""
if [ -f "${SCHTEST_DIR}/target/debug/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/target/debug/schtest"
    echo "Found schtest debug binary: ${SCHTEST_BIN}"
elif [ -f "${SCHTEST_DIR}/target/release/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/target/release/schtest"
    echo "Found schtest release binary: ${SCHTEST_BIN}"
elif [ -f "${SCHTEST_DIR}/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/schtest"
    echo "Found schtest binary: ${SCHTEST_BIN}"
else
    echo ""
    echo "✗✗✗ ERROR: Could not find schtest binary ✗✗✗"
    echo "✗ Expected one of:"
    echo "✗   - ${SCHTEST_DIR}/target/debug/schtest (preferred)"
    echo "✗   - ${SCHTEST_DIR}/target/release/schtest"
    echo "✗   - ${SCHTEST_DIR}/schtest"
    echo ""
    echo "✗ Directory contents:"
    ls -la "${SCHTEST_DIR}" 2>/dev/null | head -20 || true
    echo ""
    echo "✗ Debug directory contents:"
    ls -la "${SCHTEST_DIR}/target/debug" 2>/dev/null | head -10 || true
    echo ""
    echo "✗ Release directory contents:"
    ls -la "${SCHTEST_DIR}/target/release" 2>/dev/null | head -10 || true
    exit 1
fi

# Export SCHEDULER_BINARY as environment variable for schtest cases
export SCHEDULER_BINARY

# Function to cleanup scheduler processes
cleanup_scheduler() {
    echo ""
    echo "Cleaning up any running scheduler processes..."
    
    # Kill specific PID if set
    if [ -n "${SCHEDULER_PID:-}" ]; then
        if kill -0 "$SCHEDULER_PID" 2>/dev/null; then
            echo "  Stopping scheduler (PID: $SCHEDULER_PID)..."
            kill "$SCHEDULER_PID" 2>/dev/null || true
            sleep 2
            kill -9 "$SCHEDULER_PID" 2>/dev/null || true
        fi
    fi
    
    # Kill any remaining scheduler processes by binary path
    if [ -n "${SCHEDULER_BINARY:-}" ]; then
        if pgrep -f "${SCHEDULER_BINARY}" > /dev/null 2>&1; then
            echo "  Stopping remaining scheduler processes..."
            pkill -f "${SCHEDULER_BINARY}" 2>/dev/null || true
            sleep 2
            pkill -9 -f "${SCHEDULER_BINARY}" 2>/dev/null || true
        fi
    fi
    
    # Wait for state to become disabled (if sysfs available)
    if [ -f "/sys/kernel/sched_ext/state" ]; then
        echo "  Waiting for scheduler state to clear..."
        for i in {1..10}; do
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            if [ "${CURRENT_STATE}" = "disabled" ] || [ "${CURRENT_STATE}" = "unknown" ]; then
                echo "  ✓ Scheduler state cleared: ${CURRENT_STATE}"
                break
            fi
            sleep 1
        done
    fi
}

# Function to start scheduler and wait for it to be ready
start_scheduler() {
    cleanup_scheduler  # Ensure clean start
    
    echo "Starting Gthulhu scheduler: ${SCHEDULER_BINARY}"
    
    # Prepare scheduler environment
    SCHEDULER_DIR=$(dirname "${SCHEDULER_BINARY}")
    
    # Create API directory if it doesn't exist (for JWT public key)
    if [ ! -d "${SCHEDULER_DIR}/api" ]; then
        mkdir -p "${SCHEDULER_DIR}/api/config" 2>/dev/null || true
    fi
    
    # Create a dummy JWT public key file if it doesn't exist (to avoid startup errors)
    # The scheduler will log a warning but should still start
    # Note: The scheduler looks for ./api/jwt_public_key.pem (relative to working directory)
    if [ ! -f "${SCHEDULER_DIR}/api/jwt_public_key.pem" ]; then
        echo "⚠ JWT public key file not found, creating dummy file to avoid startup errors..."
        mkdir -p "${SCHEDULER_DIR}/api" 2>/dev/null || true
        # Create a minimal valid PEM file structure (even if it's not a real key)
        cat > "${SCHEDULER_DIR}/api/jwt_public_key.pem" <<EOF
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z3VS5JJcds3xfn/ygWp
4X2HVIjLc5e3uYuKtAhQ8SIGH9cPeHxTOAYWl0VHZRhqvQnF6XjLhOmNZHPDHuBP
wBKjqrXXRkWXZfVZcnLQrmFUvNRoMdBfr7E5T8W0W7XQ7x/oVXn6l6NRQw2Ycb5c
b0mQhXLZfJXsrFqYXQWXC7nJx6D5/CJvGNB3xXmXQ0VqYHPvOXxRQQvNQqQVbPZH
8WQXJ0jVWQFxYXmQz9QW+MZhXPwXvHqQjQXQ7vNRQQwZJXxvHQmQZvXQRQXQHvQZ
vHQZQXQvZHQRQXHQvQRQZQXvQHQZQRvXQHQXvQZHQXRQZvHQXQRHQZvXQRQXHQvZ
QIDAQAB
-----END PUBLIC KEY-----
EOF
        chmod 644 "${SCHEDULER_DIR}/api/jwt_public_key.pem" 2>/dev/null || true
    fi
    
    # Start scheduler in background from its directory (for relative paths)
    cd "${SCHEDULER_DIR}"
    "${SCHEDULER_BINARY}" >> "${LOGFILE}" 2>&1 &
    SCHEDULER_PID=$!
    cd - > /dev/null
    
    # Register cleanup function
    trap cleanup_scheduler EXIT INT TERM
    
    # Wait for warmup
    echo "Waiting for scheduler to initialize (${WARMUP_TIME}s)..."
    sleep "${WARMUP_TIME}"
    
    # Check if scheduler is still running
    if ! kill -0 "$SCHEDULER_PID" 2>/dev/null; then
        echo "✗✗✗ ERROR: Scheduler process died immediately after starting ✗✗✗"
        echo "✗ Check scheduler logs:"
        tail -50 "${LOGFILE}" 2>/dev/null || true
        return 1
    fi
    
    # Check and wait for state=enabled (if sysfs available)
    if [ -f "/sys/kernel/sched_ext/state" ]; then
        echo "Waiting for scheduler to reach 'enabled' state..."
        for i in {1..10}; do
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            echo "  Attempt $i: scheduler state = ${CURRENT_STATE}"
            if [ "${CURRENT_STATE}" = "enabled" ] || [ "${CURRENT_STATE}" = "running" ]; then
                echo "✓ Scheduler is enabled (state: ${CURRENT_STATE})"
                return 0
            elif [ "${CURRENT_STATE}" = "disabling" ]; then
                echo "  ⚠ Scheduler is in 'disabling' state, waiting..."
                sleep 2
            elif [ "${CURRENT_STATE}" = "disabled" ] && [ $i -lt 10 ]; then
                echo "  ⚠ Scheduler state is 'disabled', waiting for it to enable..."
                sleep 2
            fi
        done
        echo "⚠ Warning: Scheduler state is '${CURRENT_STATE}' after 10 attempts"
        echo "  Continuing anyway (scheduler may still be functional)..."
    else
        echo "⚠ sysfs state not available; assuming scheduler is ready after warmup."
    fi
    
    echo "✓ Scheduler is running (PID: $SCHEDULER_PID)"
    return 0
}

# Main test loop with retries
RETRY_COUNT=0
SCHTEST_SUCCESS=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    echo ""
    echo "=========================================="
    echo "Attempt $((RETRY_COUNT + 1)) of $MAX_RETRIES"
    echo "=========================================="
    
    if ! start_scheduler; then
        echo "✗ Failed to start scheduler, aborting..."
        exit 1
    fi
    
    echo ""
    echo "Running schtest: sudo ${SCHTEST_BIN}"
    echo ""
    
    # Run schtest with sudo (no scheduler binary argument needed since it's already running)
    if sudo "${SCHTEST_BIN}" 2>&1; then
        SCHTEST_SUCCESS=true
        echo ""
        echo "✓ Schtest tests passed"
        break
    else
        SCHTEST_EXIT_CODE=$?
        echo ""
        echo "✗ Schtest attempt $((RETRY_COUNT + 1)) failed (exit code: $SCHTEST_EXIT_CODE)"
        echo "  Check ${LOGFILE} for scheduler logs"
    fi
    
    # Cleanup for next retry
    cleanup_scheduler
    
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        echo ""
        echo "Retrying in 5 seconds..."
        sleep 5
    fi
done

# Final cleanup
cleanup_scheduler

if [ "$SCHTEST_SUCCESS" = true ]; then
    echo ""
    echo "✓✓✓ Schtest integration test completed successfully ✓✓✓"
    exit 0
else
    echo ""
    echo "✗✗✗ Schtest tests FAILED after $MAX_RETRIES attempts ✗✗✗"
    echo "✗ Please check the logs above and ${LOGFILE} for details"
    exit 1
fi
