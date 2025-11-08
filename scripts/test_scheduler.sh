#!/bin/bash
# Scheduler test script
# This script runs the scheduler, verifies it starts successfully,
# and then runs the external schtest suite.

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
LOGFILE="/tmp/scheduler_test.log"
TIMEOUT_DURATION=60
WARMUP_TIME=15
SCHTEST_DIR="/opt/schtest" # Path where schtest is cloned

# --- Globals ---
# To be populated with the scheduler's process ID.
SCHED_PID=""

# --- Functions ---

# Ensures the background scheduler process is cleaned up on script exit.
cleanup() {
  # The SCHED_PID check ensures we only try to kill if it was set.
  if [[ -n "${SCHED_PID}" ]]; then
    # The ps check ensures we only try to kill if the process still exists.
    if ps -p "${SCHED_PID}" > /dev/null 2>&1; then
      echo "Cleaning up scheduler process (PID: ${SCHED_PID})..."
      # Gracefully terminate the process.
      kill "${SCHED_PID}" 2>/dev/null || true
    fi
  fi
}

# Reports an error message to stderr and exits.
report_error_and_exit() {
    local message="$1"
    local exit_code="${2:-1}" # Default to 1
    echo "✗ ${message}" >&2
    if [ -f "${LOGFILE}" ]; then
        echo "Log output:" >&2
        cat "${LOGFILE}" >&2
    fi
    exit "${exit_code}"
}

# --- Main Script ---

# Register the cleanup function to be called on script exit, error, or interrupt.
trap cleanup EXIT

echo "Starting scheduler test..."

# Run scheduler in background
timeout "${TIMEOUT_DURATION}" ./main > "${LOGFILE}" 2>&1 &
SCHED_PID=$!

echo "Scheduler PID: ${SCHED_PID}"

# Wait for scheduler to initialize
sleep "${WARMUP_TIME}"

# Check if scheduler is still running
if ! ps -p "${SCHED_PID}" > /dev/null 2>&1; then
    report_error_and_exit "Scheduler crashed during initialization"
fi

echo "✓ Scheduler is running"

# Check if scheduler started successfully
if ! grep -q "scheduler started" "${LOGFILE}"; then
    # The trap will handle killing the process on exit.
    report_error_and_exit "Scheduler did not start properly"
fi
echo "✓ Scheduler started successfully"

# Let it run for a few more seconds
sleep 20

# Check final stats
if grep -q "bss data" "${LOGFILE}"; then
    echo "✓ Scheduler produced stats"
fi

# Clean shutdown
echo "Stopping scheduler..."
kill "${SCHED_PID}" 2>/dev/null || true
wait "${SCHED_PID}" 2>/dev/null || true
# Unset PID so the exit trap knows the process was handled cleanly.
SCHED_PID=""

echo "✓ Scheduler self-test completed successfully"

# --- schtest integration ---
# Run in a subshell to isolate directory changes. `set -e` will cause the
# subshell to exit on failure, which in turn causes the main script to exit.
echo ""
_schtest() {
    echo "--- Running schtest suite ---"
    if [ ! -d "${SCHTEST_DIR}" ]; then
        echo "✗ schtest directory '${SCHTEST_DIR}' not found. Please ensure it is cloned." >&2
        exit 127
    fi

    cd "${SCHTEST_DIR}"

    echo "Building schtest..."
    make
    echo "✓ schtest built successfully"

    echo "Executing schtest suite..."
    make test
    echo "✓ schtest suite completed successfully"
}

# Execute the schtest function in a subshell
(_schtest)


# Final success message
echo ""
echo "--- All tests completed successfully ---"
exit 0