#!/usr/bin/env bash
set -euo pipefail

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0

# Helper functions
test_start() {
    echo -n "Testing: $1 ... "
    TESTS_RUN=$((TESTS_RUN + 1))
}

test_pass() {
    echo -e "${GREEN}PASS${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

test_fail() {
    echo -e "${RED}FAIL${NC}"
    echo "  Error: $1"
}

cleanup() {
    # Kill any background processes
    jobs -p | xargs -r kill 2>/dev/null || true
    
    # Stop MySQL container
    docker compose down -v 2>/dev/null || true
}

trap cleanup EXIT

# Start MySQL container
echo "Starting MySQL container..."
docker compose up -d

# Wait for MySQL to be ready
echo "Waiting for MySQL to be ready..."
for i in {1..30}; do
    if docker compose exec -T mysql mysqladmin ping -h localhost -u root -prootpass >/dev/null 2>&1; then
        echo "MySQL is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "MySQL failed to start"
        exit 1
    fi
    sleep 1
done

# Build the binary
echo "Building mylock binary..."
go build -o ./mylock ./cmd/mylock || { echo "Build failed"; exit 1; }
echo "Binary built successfully"
ls -la ./mylock

# Set environment variables
export MYLOCK_HOST=127.0.0.1
export MYLOCK_PORT=13306
export MYLOCK_USER=testuser
export MYLOCK_PASSWORD=testpass
export MYLOCK_DATABASE=testdb

# Test 1: Help flag
test_start "Help flag"
if [ ! -f ./mylock ]; then
    test_fail "Binary not found at ./mylock"
else
    HELP_OUTPUT=$(./mylock --help 2>&1) || true
    if echo "$HELP_OUTPUT" | grep -q "Acquire a MySQL advisory lock"; then
        test_pass
    else
        test_fail "Help output not found. Got: $HELP_OUTPUT"
    fi
fi

# Test 2: Simple command execution
test_start "Simple command execution"
OUTPUT=$(./mylock --lock-name test-simple --timeout 5 -- echo "hello world")
if [ "$OUTPUT" = "hello world" ]; then
    test_pass
else
    test_fail "Expected 'hello world', got '$OUTPUT'"
fi

# Test 3: Command exit code propagation
test_start "Exit code propagation"
./mylock --lock-name test-exit --timeout 5 -- sh -c "exit 42" || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 42 ]; then
    test_pass
else
    test_fail "Expected exit code 42, got ${EXIT_CODE:-0}"
fi

# Test 4: Lock timeout
test_start "Lock timeout"
# First process holds the lock
./mylock --lock-name test-timeout --timeout 10 -- sleep 5 &
PID1=$!
sleep 1

# Second process should timeout
./mylock --lock-name test-timeout --timeout 1 -- echo "should not print" 2>/dev/null || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 200 ]; then
    test_pass
else
    test_fail "Expected exit code 200 (timeout), got ${EXIT_CODE:-0}"
fi

wait $PID1

# Test 5: Concurrent execution prevention
test_start "Concurrent execution prevention"
LOCK_NAME="test-concurrent"
TEMP_FILE=$(mktemp)

# Start two processes that try to write to the same file
(./mylock --lock-name "$LOCK_NAME" --timeout 5 -- sh -c "echo 'process1' >> $TEMP_FILE; sleep 2") &
PID1=$!
sleep 0.5
(./mylock --lock-name "$LOCK_NAME" --timeout 5 -- sh -c "echo 'process2' >> $TEMP_FILE; sleep 2") &
PID2=$!

# Wait for both to complete
wait $PID1
wait $PID2

# Check that writes were serialized (should have exactly 2 lines)
LINE_COUNT=$(wc -l < "$TEMP_FILE")
if [ "$LINE_COUNT" -eq 2 ]; then
    test_pass
else
    test_fail "Expected 2 lines in output, got $LINE_COUNT"
fi
rm -f "$TEMP_FILE"

# Test 6: Signal forwarding
test_start "Signal forwarding (SIGTERM)"
./mylock --lock-name test-signal --timeout 10 -- sh -c "trap 'exit 143' TERM; sleep 10" &
PID=$!
sleep 1
kill -TERM $PID
wait $PID || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 143 ]; then
    test_pass
else
    test_fail "Expected exit code 143 (SIGTERM), got ${EXIT_CODE:-0}"
fi

# Test 7: stdin/stdout/stderr passthrough
test_start "I/O passthrough"
echo "test input" | ./mylock --lock-name test-io --timeout 5 -- cat > /tmp/mylock-test-output
if [ "$(cat /tmp/mylock-test-output)" = "test input" ]; then
    test_pass
else
    test_fail "stdin/stdout passthrough failed"
fi
rm -f /tmp/mylock-test-output

# Test 8: Different locks don't block each other
test_start "Different locks don't block"
./mylock --lock-name lock-a --timeout 5 -- sleep 2 &
PID1=$!
sleep 0.5
START_TIME=$(date +%s)
./mylock --lock-name lock-b --timeout 5 -- echo "done"
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ $DURATION -lt 2 ]; then
    test_pass
else
    test_fail "Different locks blocked each other (took ${DURATION}s)"
fi
wait $PID1

# Test 9: Missing environment variables
test_start "Missing environment variables"
unset MYLOCK_HOST
./mylock --lock-name test-env --timeout 5 -- echo "should fail" 2>/dev/null || EXIT_CODE=$?
export MYLOCK_HOST=127.0.0.1
if [ "${EXIT_CODE:-0}" -eq 201 ]; then
    test_pass
else
    test_fail "Expected exit code 201 (internal error), got ${EXIT_CODE:-0}"
fi

# Test 10: Invalid arguments
test_start "Invalid arguments"
./mylock --invalid-flag 2>/dev/null || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 201 ]; then
    test_pass
else
    test_fail "Expected exit code 201 (internal error), got ${EXIT_CODE:-0}"
fi

# Test 11: Lock name from command
test_start "Lock name from command"
OUTPUT=$(./mylock --lock-name-from-command --timeout 5 -- echo "test output")
if [ "$OUTPUT" = "test output" ]; then
    test_pass
else
    test_fail "Expected 'test output', got '$OUTPUT'"
fi

# Test 12: Same command produces same lock name
test_start "Same command with --lock-name-from-command blocks"
# First process holds the lock
./mylock --lock-name-from-command --timeout 10 -- sh -c "sleep 3; echo 'first'" &
PID1=$!
sleep 1

# Second process with same command should timeout
./mylock --lock-name-from-command --timeout 1 -- sh -c "sleep 3; echo 'first'" 2>/dev/null || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 200 ]; then
    test_pass
else
    test_fail "Expected exit code 200 (timeout), got ${EXIT_CODE:-0}"
fi
wait $PID1

# Test 13: Different commands don't block each other with --lock-name-from-command
test_start "Different commands with --lock-name-from-command don't block"
./mylock --lock-name-from-command --timeout 5 -- sleep 2 &
PID1=$!
sleep 0.5
START_TIME=$(date +%s)
./mylock --lock-name-from-command --timeout 5 -- echo "different command"
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ $DURATION -lt 2 ]; then
    test_pass
else
    test_fail "Different commands blocked each other (took ${DURATION}s)"
fi
wait $PID1

# Test 14: Cannot use both --lock-name and --lock-name-from-command
test_start "Cannot use both lock name options"
./mylock --lock-name test --lock-name-from-command --timeout 5 -- echo "test" 2>/dev/null || EXIT_CODE=$?
if [ "${EXIT_CODE:-0}" -eq 201 ]; then
    test_pass
else
    test_fail "Expected exit code 201 (internal error), got ${EXIT_CODE:-0}"
fi

# Test 15: Empty password is allowed
test_start "Empty password allowed"
# Temporarily set empty password
OLD_PASSWORD="$MYLOCK_PASSWORD"
export MYLOCK_PASSWORD=""
# The command should either succeed (if MySQL accepts empty password) or fail with connection error
# But it should NOT fail with "MYLOCK_PASSWORD environment variable is required"
OUTPUT=$(./mylock --lock-name test-empty-pass --timeout 1 -- echo "empty pass test" 2>&1)
if echo "$OUTPUT" | grep -q "MYLOCK_PASSWORD environment variable is required"; then
    test_fail "Empty password was rejected by config validation"
else
    test_pass
fi
export MYLOCK_PASSWORD="$OLD_PASSWORD"

# Summary
echo
echo "================================="
echo "Tests run: $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $((TESTS_RUN - TESTS_PASSED))"
echo "================================="

if [ $TESTS_PASSED -eq $TESTS_RUN ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi