#!/bin/bash
# Schtest integration script
# This script runs schtest test cases against the Gthulhu scheduler
# It starts the Gthulhu scheduler first, then runs schtest against it

set -e

SCHTEST_DIR="schtest"
SCHEDULER_BINARY_DEFAULT="./main"
LOGFILE="/tmp/schtest_scheduler.log"
WARMUP_TIME=15

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

# Export SCHEDULER_BINARY as environment variable for schtest cases
export SCHEDULER_BINARY

# Ensure no scheduler is running before schtest starts
# This helps avoid "disabling" status issues
echo "Checking for any running scheduler processes..."
if pgrep -f "${SCHEDULER_BINARY}" > /dev/null 2>&1; then
    echo "⚠ Found running scheduler processes, stopping them..."
    pkill -f "${SCHEDULER_BINARY}" || true
    sleep 2
    # Force kill if still running
    pkill -9 -f "${SCHEDULER_BINARY}" || true
    sleep 1
fi

# Check scheduler status via sysfs (if available)
# This helps ensure scheduler state is cleared
if [ -f "/sys/kernel/sched_ext/state" ]; then
    echo "Checking scheduler state..."
    CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
    echo "Current scheduler state: ${CURRENT_STATE}"
    
    # Wait for state to clear if it's disabling
    if [ "${CURRENT_STATE}" = "disabling" ]; then
        echo "⚠ Scheduler is in 'disabling' state, waiting for it to clear..."
        for i in {1..10}; do
            sleep 1
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            if [ "${CURRENT_STATE}" != "disabling" ]; then
                echo "✓ Scheduler state cleared: ${CURRENT_STATE}"
                break
            fi
        done
    fi
    
    # Ensure state is disabled before schtest starts
    # This helps avoid "disabling" status when schtest queries
    if [ "${CURRENT_STATE}" != "disabled" ] && [ "${CURRENT_STATE}" != "unknown" ]; then
        echo "⚠ Scheduler state is not 'disabled' (${CURRENT_STATE}), waiting for it to clear..."
        for i in {1..10}; do
            sleep 1
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            if [ "${CURRENT_STATE}" = "disabled" ] || [ "${CURRENT_STATE}" = "unknown" ]; then
                echo "✓ Scheduler state is now: ${CURRENT_STATE}"
                break
            fi
        done
    fi
fi

# Ensure necessary directories and files exist for scheduler startup
# This helps avoid startup failures that could cause "disabling" status
echo "Preparing scheduler environment..."
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

# Start the Gthulhu scheduler in the background
echo "Starting Gthulhu scheduler..."
cd "${SCHEDULER_DIR}"
"${SCHEDULER_BINARY}" > "${LOGFILE}" 2>&1 &
SCHEDULER_PID=$!
cd - > /dev/null

# Function to cleanup scheduler on exit
cleanup_scheduler() {
    echo ""
    if [ -n "${SCHEDULER_PID:-}" ]; then
        echo "Cleaning up scheduler (PID: $SCHEDULER_PID)..."
        if kill -0 "$SCHEDULER_PID" 2>/dev/null; then
            kill "$SCHEDULER_PID" 2>/dev/null || true
            sleep 2
            # Force kill if still running
            kill -9 "$SCHEDULER_PID" 2>/dev/null || true
        fi
    fi
    # Also kill any remaining scheduler processes
    if [ -n "${SCHEDULER_BINARY:-}" ]; then
        pkill -f "${SCHEDULER_BINARY}" 2>/dev/null || true
        pkill -9 -f "${SCHEDULER_BINARY}" 2>/dev/null || true
    fi
}

# Register cleanup function
trap cleanup_scheduler EXIT INT TERM

# Wait for scheduler to fully start
echo "Waiting for scheduler to initialize..."
sleep 3

# Check if scheduler is still running
if ! kill -0 "$SCHEDULER_PID" 2>/dev/null; then
    echo "✗✗✗ ERROR: Scheduler process died immediately after starting ✗✗✗"
    echo "✗ Check scheduler logs:"
    tail -50 "${LOGFILE}" 2>/dev/null || true
    exit 1
fi

# Check scheduler state via sysfs (if available)
if [ -f "/sys/kernel/sched_ext/state" ]; then
    echo "Checking scheduler state..."
    for i in {1..10}; do
        CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
        echo "  Attempt $i: scheduler state = ${CURRENT_STATE}"
        if [ "${CURRENT_STATE}" = "enabled" ] || [ "${CURRENT_STATE}" = "running" ]; then
            echo "✓ Scheduler is running (state: ${CURRENT_STATE})"
            break
        elif [ "${CURRENT_STATE}" = "disabling" ]; then
            echo "⚠ Scheduler is in 'disabling' state, waiting..."
            sleep 2
        elif [ "${CURRENT_STATE}" = "disabled" ] && [ $i -lt 10 ]; then
            echo "⚠ Scheduler state is 'disabled', waiting for it to enable..."
            sleep 2
        fi
        if [ $i -eq 10 ]; then
            echo "⚠ Warning: Scheduler state is still '${CURRENT_STATE}' after 10 attempts"
        fi
    done
fi

echo "✓ Scheduler is running (PID: $SCHEDULER_PID)"
echo ""

# Run schtest - use debug version if available, otherwise try release
echo "Running schtest tests..."

# Check for schtest binary - prefer debug version, then release, then other locations
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

# Run schtest with sudo (no scheduler binary argument needed since it's already running)
echo "Executing: sudo ${SCHTEST_BIN}"
if sudo "${SCHTEST_BIN}" 2>&1; then
    echo "✓ Schtest tests passed"
    SCHTEST_EXIT_CODE=0
else
    SCHTEST_EXIT_CODE=$?
    echo ""
    echo "✗✗✗ Schtest tests FAILED ✗✗✗"
    echo "✗ Exit code: $SCHTEST_EXIT_CODE"
    echo "✗ Please check the logs above for details"
    exit 1
fi

echo ""
echo "✓ Schtest integration test completed successfully"
exit 0

