#!/bin/bash
# Schtest integration script
# This script runs schtest test cases against the Gthulhu scheduler
# Note: schtest will start the scheduler itself, but we can pre-initialize it

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
if [ ! -f "${SCHEDULER_DIR}/api/jwt_public_key.pem" ] && [ ! -f "${SCHEDULER_DIR}/api/config/jwt_public_key.pem" ]; then
    echo "⚠ JWT public key file not found, creating dummy file to avoid startup errors..."
    mkdir -p "${SCHEDULER_DIR}/api/config" 2>/dev/null || true
    echo "# Dummy JWT public key for testing" > "${SCHEDULER_DIR}/api/config/jwt_public_key.pem" 2>/dev/null || true
fi

# Note: schtest will start the scheduler itself
# We don't pre-initialize because it causes scheduler state issues
# (e.g., "disabling" status when schtest tries to query it)

# Run schtest - check if schtest has a test runner
# Note: schtest will start the scheduler itself, we don't need to start it in background
echo "Running schtest tests..."

# Check if there's a built binary (preferred, works in vng environment)
if [ -f "${SCHTEST_DIR}/target/release/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/target/release/schtest"
    echo "Running schtest binary: ${SCHTEST_BIN}"
    # Check schtest help to understand usage
    echo "Checking schtest usage..."
    "${SCHTEST_BIN}" --help 2>&1 | head -20 || true
    # Run schtest with scheduler binary path as argument (if supported)
    # Also export as environment variable for cases that use it
    if "${SCHTEST_BIN}" "${SCHEDULER_BINARY}" 2>&1; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed"
        exit 1
    fi
elif [ -f "${SCHTEST_DIR}/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/schtest"
    echo "Running schtest binary: ${SCHTEST_BIN}"
    # Run schtest with scheduler binary path
    if "${SCHTEST_BIN}" "${SCHEDULER_BINARY}" 2>&1; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed"
        exit 1
    fi
# Check if schtest has a Makefile with test target
elif [ -f "${SCHTEST_DIR}/Makefile" ]; then
    echo "Running schtest via Makefile..."
    cd "${SCHTEST_DIR}"
    # Pass SCHEDULER_BINARY as environment variable, not as Makefile variable
    if SCHEDULER_BINARY="${SCHEDULER_BINARY}" make test 2>&1; then
        echo "✓ Schtest tests passed"
        cd - > /dev/null
    else
        echo "✗ Schtest tests failed"
        cd - > /dev/null
        exit 1
    fi
# Check if schtest has Cargo.toml (Rust project) - only if cargo is available
elif [ -f "${SCHTEST_DIR}/Cargo.toml" ] && command -v cargo >/dev/null 2>&1; then
    echo "Running schtest via Cargo..."
    cd "${SCHTEST_DIR}"
    # Pass SCHEDULER_BINARY as environment variable for Rust tests
    # Rust tests can access it via std::env::var("SCHEDULER_BINARY")
    if SCHEDULER_BINARY="${SCHEDULER_BINARY}" cargo test --all --all-features -- --nocapture 2>&1; then
        echo "✓ Schtest tests passed"
        cd - > /dev/null
    else
        echo "✗ Schtest tests failed"
        cd - > /dev/null
        exit 1
    fi
else
    echo "⚠ Could not find schtest test runner"
    echo "Expected one of:"
    echo "  - ${SCHTEST_DIR}/target/release/schtest (built binary)"
    echo "  - ${SCHTEST_DIR}/schtest (binary)"
    echo "  - ${SCHTEST_DIR}/Makefile (with test target)"
    if [ -f "${SCHTEST_DIR}/Cargo.toml" ]; then
        echo "  - ${SCHTEST_DIR}/Cargo.toml (Rust project, but cargo not available in vng environment)"
        echo "    Note: schtest should be built before running in vng environment"
    fi
    ls -la "${SCHTEST_DIR}" 2>/dev/null | head -20 || true
    ls -la "${SCHTEST_DIR}/target/release" 2>/dev/null | head -10 || true
    exit 1
fi

echo "✓ Schtest integration test completed successfully"
exit 0

