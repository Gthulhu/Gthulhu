#!/bin/bash
# Scheduler test script
# This script runs the scheduler and verifies it starts successfully

set -e

LOGFILE="/tmp/scheduler_test.log"
TIMEOUT_DURATION=60
WARMUP_TIME=15

echo "Starting scheduler test..."

# Run scheduler in background
timeout ${TIMEOUT_DURATION} ./main > "${LOGFILE}" 2>&1 &
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

echo "✓ Scheduler is running"

# Check if scheduler started successfully
if grep -q "scheduler started" "${LOGFILE}"; then
    echo "✓ Scheduler started successfully"
else
    echo "✗ Scheduler did not start properly"
    echo "Log output:"
    cat "${LOGFILE}"
    kill ${SCHED_PID} 2>/dev/null || true
    exit 1
fi

# Let it run for a few more seconds
sleep 20

# Check final stats
if grep -q "bss data" "${LOGFILE}"; then
    echo "✓ Scheduler produced stats"
fi

# Clean shutdown
echo "Stopping scheduler..."
kill ${SCHED_PID} 2>/dev/null || true
wait ${SCHED_PID} 2>/dev/null || true

echo "✓ Test completed successfully"
exit 0
