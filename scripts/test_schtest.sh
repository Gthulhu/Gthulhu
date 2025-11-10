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

# Pre-initialize scheduler to ensure it's ready
# This helps avoid watchdog timeouts by ensuring the scheduler is fully initialized
echo "Pre-initializing scheduler..."
timeout 30 "${SCHEDULER_BINARY}" > "${LOGFILE}" 2>&1 &
SCHED_PID=$!

echo "Scheduler PID: ${SCHED_PID}"

# Wait for scheduler to initialize
sleep ${WARMUP_TIME}

# Check if scheduler is still running
if ! ps -p ${SCHED_PID} > /dev/null 2>&1; then
    echo "✗ Scheduler crashed during initialization"
    echo "Log output:"
    cat "${LOGFILE}"
    exit 1
fi

# Check if scheduler started successfully
if grep -q "scheduler started" "${LOGFILE}"; then
    echo "✓ Scheduler initialized successfully"
else
    echo "⚠ Scheduler may not have started properly"
    echo "Log output:"
    cat "${LOGFILE}"
fi

# Stop the pre-initialized scheduler (schtest will start its own instance)
echo "Stopping pre-initialized scheduler..."
kill ${SCHED_PID} 2>/dev/null || true
wait ${SCHED_PID} 2>/dev/null || true

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

