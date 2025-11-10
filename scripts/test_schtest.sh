#!/bin/bash
# Schtest integration script (adjusted based on official advice: start Gthulhu first, then sudo schtest)

set -euo pipefail

SCHTEST_DIR="schtest"
SCHEDULER_BINARY_DEFAULT="./main"  # Assuming Gthulhu is ./main; adjust if needed
LOGFILE="/tmp/schtest_scheduler.log"
WARMUP_TIME=15  # Time to wait for scheduler to stabilize (adjust as needed)
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

# Check if schtest binary exists (using official path: target/debug/schtest; change to release if needed)
SCHTEST_BIN="${SCHTEST_DIR}/target/debug/schtest"
if [ ! -f "${SCHTEST_BIN}" ]; then
    echo "✗ Schtest binary not found: ${SCHTEST_BIN}"
    echo "  Please build schtest in debug mode or adjust path."
    exit 1
fi

# Function to clean up running scheduler
cleanup_scheduler() {
    echo "Cleaning up any running scheduler processes..."
    if pgrep -f "^${SCHEDULER_BINARY}(\s|$)" >/dev/null 2>&1; then
        echo "⚠ Found running scheduler, stopping..."
        pkill -f "^${SCHEDULER_BINARY}(\s|$)" || true
        sleep 2
        pkill -9 -f "^${SCHEDULER_BINARY}(\s|$)" || true
        sleep 1
    fi

    # Wait for state to become disabled (if sysfs available)
    if [ -f "/sys/kernel/sched_ext/state" ]; then
        echo "Waiting for scheduler state to clear (target: disabled)..."
        for i in {1..10}; do
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            if [ "${CURRENT_STATE}" = "disabled" ] || [ "${CURRENT_STATE}" = "unknown" ]; then
                echo "✓ Scheduler state cleared: ${CURRENT_STATE}"
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
    # Start in background; redirect output to log (assume no sudo needed; add if required)
    "${SCHEDULER_BINARY}" >> "${LOGFILE}" 2>&1 &
    SCHEDULER_PID=$!

    # Wait for warmup
    sleep "${WARMUP_TIME}"

    # Check and wait for state=enabled (if sysfs available)
    if [ -f "/sys/kernel/sched_ext/state" ]; then
        echo "Waiting for scheduler to reach 'enabled' state..."
        for i in {1..10}; do
            CURRENT_STATE=$(cat /sys/kernel/sched_ext/state 2>/dev/null || echo "unknown")
            echo "Current state: ${CURRENT_STATE}"
            if [ "${CURRENT_STATE}" = "enabled" ]; then
                echo "✓ Scheduler is enabled."
                return 0
            fi
            sleep 1
        done
        echo "✗ Scheduler failed to reach 'enabled' state. Check logs."
        kill -9 "${SCHEDULER_PID}" 2>/dev/null || true
        exit 1
    else
        echo "⚠ sysfs state not available; assuming scheduler is ready after warmup."
    fi
}

# Main test loop with retries
RETRY_COUNT=0
SCHTEST_SUCCESS=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    echo "Attempt $((RETRY_COUNT + 1)) of $MAX_RETRIES..."

    start_scheduler

    echo "Running schtest: sudo ${SCHTEST_BIN}"
    # Run schtest with sudo; redirect output to log
    if sudo "${SCHTEST_BIN}" >> "${LOGFILE}" 2>&1; then
        SCHTEST_SUCCESS=true
        break
    else
        echo "⚠ Schtest attempt failed. Check ${LOGFILE} for details."
    fi

    # Cleanup for next retry
    cleanup_scheduler

    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        echo "Retrying in 5 seconds..."
        sleep 5
    fi
done

# Kill scheduler after tests (success or failure)
cleanup_scheduler

if [ "$SCHTEST_SUCCESS" = true ]; then
    echo "✓ Schtest tests passed"
else
    echo "✗ Schtest tests failed after $MAX_RETRIES attempts"
    exit 1
fi

echo "✓ Schtest integration test completed successfully"
exit 0