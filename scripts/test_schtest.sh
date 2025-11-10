#!/bin/bash
# Schtest integration script
# This script runs schtest test cases against the Gthulhu scheduler

set -e

LOGFILE="/tmp/schtest_test.log"
SCHTEST_DIR="schtest"
SCHEDULER_BINARY="./main"
TIMEOUT_DURATION=300

echo "Starting schtest integration test..."

# Check if schtest directory exists
if [ ! -d "${SCHTEST_DIR}" ]; then
    echo "✗ Schtest directory not found. Please run 'make schtest-dep' first."
    exit 1
fi

# Check if scheduler binary exists
if [ ! -f "${SCHEDULER_BINARY}" ]; then
    echo "✗ Scheduler binary not found. Please run 'make build' first."
    exit 1
fi


# Start scheduler in background
echo "Starting scheduler..."
timeout ${TIMEOUT_DURATION} ${SCHEDULER_BINARY} > "${LOGFILE}" 2>&1 &
SCHED_PID=$!

echo "Scheduler PID: ${SCHED_PID}"

# Wait for scheduler to initialize
sleep 5

# Check if scheduler is still running
if ! ps -p ${SCHED_PID} > /dev/null 2>&1; then
    echo "✗ Scheduler crashed during initialization"
    echo "Log output:"
    cat "${LOGFILE}"
    exit 1
fi

echo "✓ Scheduler is running"

# Run schtest - check if schtest has a test runner
echo "Running schtest tests..."

# Check if there's a built binary (preferred, works in vng environment)
if [ -f "${SCHTEST_DIR}/target/release/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/target/release/schtest"
    echo "Running schtest binary: ${SCHTEST_BIN}"
    if "${SCHTEST_BIN}" --scheduler-pid ${SCHED_PID} 2>&1; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed"
        kill ${SCHED_PID} 2>/dev/null || true
        exit 1
    fi
elif [ -f "${SCHTEST_DIR}/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/schtest"
    echo "Running schtest binary: ${SCHTEST_BIN}"
    if "${SCHTEST_BIN}" --scheduler-pid ${SCHED_PID} 2>&1; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed"
        kill ${SCHED_PID} 2>/dev/null || true
        exit 1
    fi
# Check if schtest has a Makefile with test target
elif [ -f "${SCHTEST_DIR}/Makefile" ]; then
    echo "Running schtest via Makefile..."
    cd "${SCHTEST_DIR}"
    if make test SCHEDULER_PID=${SCHED_PID} 2>&1; then
        echo "✓ Schtest tests passed"
        cd - > /dev/null
    else
        echo "✗ Schtest tests failed"
        cd - > /dev/null
        kill ${SCHED_PID} 2>/dev/null || true
        exit 1
    fi
# Check if schtest has Cargo.toml (Rust project) - only if cargo is available
elif [ -f "${SCHTEST_DIR}/Cargo.toml" ] && command -v cargo >/dev/null 2>&1; then
    echo "Running schtest via Cargo..."
    cd "${SCHTEST_DIR}"
    if cargo test -- --scheduler-pid ${SCHED_PID} 2>&1; then
        echo "✓ Schtest tests passed"
        cd - > /dev/null
    else
        echo "✗ Schtest tests failed"
        cd - > /dev/null
        kill ${SCHED_PID} 2>/dev/null || true
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
    kill ${SCHED_PID} 2>/dev/null || true
    exit 1
fi

# Clean shutdown
echo "Stopping scheduler..."
kill ${SCHED_PID} 2>/dev/null || true
wait ${SCHED_PID} 2>/dev/null || true

echo "✓ Schtest integration test completed successfully"
exit 0

