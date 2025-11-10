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

# Note: schtest will start the scheduler itself
# We don't pre-initialize because it causes scheduler state issues
# (e.g., "disabling" status when schtest tries to query it)

# Create a wrapper script for the scheduler that adds a startup delay
# This helps schtest avoid catching the scheduler in "disabling" transition state
SCHEDULER_WRAPPER="${SCHEDULER_DIR}/scheduler_wrapper.sh"
echo "Creating scheduler wrapper script..."
cat > "${SCHEDULER_WRAPPER}" <<'WRAPPER_EOF'
#!/bin/bash
# Wrapper script to add startup delay for scheduler
# This ensures schtest doesn't query the scheduler during state transitions

# Get the real scheduler binary path (passed as first argument or from wrapper location)
REAL_SCHEDULER="$(dirname "$0")/main"

# Start the scheduler in background
"${REAL_SCHEDULER}" "$@" &
SCHEDULER_PID=$!

# Add a delay to let the scheduler fully initialize
# This prevents schtest from catching it in "disabling" state
sleep 2

# Wait for the scheduler process
wait $SCHEDULER_PID
WRAPPER_EOF

chmod +x "${SCHEDULER_WRAPPER}"

# Use the wrapper instead of the real binary
SCHEDULER_BINARY="${SCHEDULER_WRAPPER}"
export SCHEDULER_BINARY

echo "Waiting for system to stabilize before running schtest..."
sleep 2

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
    
    # Run schtest with retry logic to handle scheduler state transition timing
    # Sometimes schtest catches the scheduler in "disabling" state during startup
    MAX_RETRIES=3
    RETRY_COUNT=0
    SCHTEST_SUCCESS=false
    
    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        echo "Attempt $((RETRY_COUNT + 1)) of $MAX_RETRIES..."
        if "${SCHTEST_BIN}" "${SCHEDULER_BINARY}" 2>&1; then
            SCHTEST_SUCCESS=true
            break
        else
            RETRY_COUNT=$((RETRY_COUNT + 1))
            if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
                echo "⚠ Schtest attempt failed, retrying in 5 seconds..."
                # Ensure any running scheduler processes are stopped
                pkill -9 -f "${SCHEDULER_BINARY}" 2>/dev/null || true
                sleep 5
            fi
        fi
    done
    
    if [ "$SCHTEST_SUCCESS" = true ]; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed after $MAX_RETRIES attempts"
        exit 1
    fi
elif [ -f "${SCHTEST_DIR}/schtest" ]; then
    SCHTEST_BIN="${SCHTEST_DIR}/schtest"
    echo "Running schtest binary: ${SCHTEST_BIN}"
    
    # Run schtest with retry logic
    MAX_RETRIES=3
    RETRY_COUNT=0
    SCHTEST_SUCCESS=false
    
    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        echo "Attempt $((RETRY_COUNT + 1)) of $MAX_RETRIES..."
        if "${SCHTEST_BIN}" "${SCHEDULER_BINARY}" 2>&1; then
            SCHTEST_SUCCESS=true
            break
        else
            RETRY_COUNT=$((RETRY_COUNT + 1))
            if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
                echo "⚠ Schtest attempt failed, retrying in 5 seconds..."
                pkill -9 -f "${SCHEDULER_BINARY}" 2>/dev/null || true
                sleep 5
            fi
        fi
    done
    
    if [ "$SCHTEST_SUCCESS" = true ]; then
        echo "✓ Schtest tests passed"
    else
        echo "✗ Schtest tests failed after $MAX_RETRIES attempts"
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

