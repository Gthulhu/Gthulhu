#!/bin/bash
# Schtest integration script
# This script runs schtest test cases against the Gthulhu scheduler

set -e

LOGFILE="/tmp/schtest_test.log"
SCHTEST_DIR="schtest"
SCHTEST_CASES_DIR="${SCHTEST_DIR}/src/cases"
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

# Check if test cases directory exists
if [ ! -d "${SCHTEST_CASES_DIR}" ]; then
    echo "✗ Test cases directory not found at ${SCHTEST_CASES_DIR}"
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

# Run schtest cases from src/cases directory
echo "Running schtest cases from ${SCHTEST_CASES_DIR}..."
TEST_COUNT=0
PASSED_COUNT=0
FAILED_COUNT=0

for test_case in "${SCHTEST_CASES_DIR}"/*; do
    if [ -f "${test_case}" ] && [ -x "${test_case}" ]; then
        TEST_COUNT=$((TEST_COUNT + 1))
        echo "Running test case: $(basename ${test_case})"
        if "${test_case}" --scheduler-pid ${SCHED_PID} 2>&1; then
            PASSED_COUNT=$((PASSED_COUNT + 1))
            echo "✓ Test case passed: $(basename ${test_case})"
        else
            FAILED_COUNT=$((FAILED_COUNT + 1))
            echo "✗ Test case failed: $(basename ${test_case})"
        fi
    fi
done

if [ ${TEST_COUNT} -eq 0 ]; then
    echo "⚠ No executable test cases found in ${SCHTEST_CASES_DIR}"
    kill ${SCHED_PID} 2>/dev/null || true
    exit 1
fi

echo ""
echo "Test Summary:"
echo "  Total tests: ${TEST_COUNT}"
echo "  Passed: ${PASSED_COUNT}"
echo "  Failed: ${FAILED_COUNT}"

if [ ${FAILED_COUNT} -gt 0 ]; then
    echo "✗ Some tests failed"
    kill ${SCHED_PID} 2>/dev/null || true
    exit 1
fi

# Clean shutdown
echo "Stopping scheduler..."
kill ${SCHED_PID} 2>/dev/null || true
wait ${SCHED_PID} 2>/dev/null || true

echo "✓ Schtest integration test completed successfully"
exit 0

