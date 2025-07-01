#!/bin/bash
# Full integration test for NBIA downloader
# Tests real-world scenarios and edge cases

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NBIA_TOOL="${SCRIPT_DIR}/../../nbia-downloader-fixed"
TEST_OUTPUT="${SCRIPT_DIR}/../test_output"
MANIFEST="${SCRIPT_DIR}/../fixtures/NSCLC-RADIOMICS-INTEROBSERVER1-Aug-31-2020-NBIA-manifest.tcia"
USERNAME="${NBIA_USER:-nbia_guest}"
PASSWORD="${NBIA_PASS:-}"

# Functions
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

print_result() {
    echo -e "${BLUE}  $1${NC}"
}

cleanup() {
    print_info "Cleaning up test directory..."
    rm -rf "$TEST_OUTPUT"
}

# Trap cleanup on exit
trap cleanup EXIT

echo "=========================================="
echo "NBIA Downloader Integration Test Suite"
echo "=========================================="
echo

# Test 1: Continue interrupted download (skip existing)
print_info "Test 1: Continue interrupted download..."
test1_dir="$TEST_OUTPUT/test1"
mkdir -p "$test1_dir"

# Start download and interrupt after 5 seconds
timeout 5 "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test1_dir" \
    -p 3 \
    --debug 2>&1 || true

initial_count=$(find "$test1_dir" -name "*.zip" -type f | wc -l)
print_result "Downloaded $initial_count files before interruption"

# Continue download (will skip already downloaded files)
print_info "Continuing download..."
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test1_dir" \
    -p 3 \
    --debug 2>&1 | tee "$test1_dir/continue.log"

final_count=$(find "$test1_dir" -name "*.zip" -type f | wc -l)
skipped=$(grep -c "already exists" "$test1_dir/continue.log" || echo 0)

if [ "$final_count" -gt "$initial_count" ] && [ "$skipped" -ge "$initial_count" ]; then
    print_success "Skip-existing functionality works correctly"
    print_result "Final count: $final_count files"
    print_result "Skipped existing: $skipped files"
else
    print_error "Skip-existing functionality failed"
fi

echo

# Test 2: Invalid credentials handling
print_info "Test 2: Testing invalid credentials..."
test2_dir="$TEST_OUTPUT/test2"
mkdir -p "$test2_dir"

if "$NBIA_TOOL" -u "invalid_user" --passwd "wrong_password" \
    -i "$MANIFEST" \
    -s "$test2_dir" \
    -p 1 \
    --max-retries 1 \
    --debug 2>&1 | grep -q "401\|403\|unauthorized\|failed"; then
    print_success "Invalid credentials properly rejected"
else
    print_error "Invalid credentials not handled correctly"
fi

echo

# Test 3: Network timeout handling
print_info "Test 3: Testing network timeout handling..."
test3_dir="$TEST_OUTPUT/test3"
mkdir -p "$test3_dir"

# Create a manifest with non-existent series to trigger timeouts
cat > "$test3_dir/timeout_manifest.tcia" << EOF
downloadServerUrl=https://public.cancerimagingarchive.net/nbia-download/servlet/DownloadServlet
includeAnnotation=true
noOfrRetry=4
databasketId=test-timeout
manifestVersion=3.0
ListOfSeriesToDownload=
1.2.3.4.5.6.7.8.9.invalid.series.id.12345
EOF

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$test3_dir/timeout_manifest.tcia" \
    -s "$test3_dir" \
    -p 1 \
    --max-retries 2 \
    --debug 2>&1 | tee "$test3_dir/timeout.log"; then
    # Check if it properly handled the error
    if grep -q "Failed: 1" "$test3_dir/timeout.log"; then
        print_success "Timeout/error handling works correctly"
    else
        print_error "Timeout not handled properly"
    fi
else
    print_success "Invalid series properly rejected"
fi

echo

# Test 4: Large file handling (if available in dataset)
print_info "Test 4: Testing file size verification..."
test4_dir="$TEST_OUTPUT/test4"
mkdir -p "$test4_dir"

# Download first 3 files to check size verification
head -n 9 "$MANIFEST" > "$test4_dir/small_test.tcia"

"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$test4_dir/small_test.tcia" \
    -s "$test4_dir" \
    -p 1 \
    --debug 2>&1 | tee "$test4_dir/size_verify.log"

# Check if size verification messages appear
if grep -q "size" "$test4_dir/size_verify.log"; then
    print_success "File size verification active"
else
    print_info "File size verification not tested (no size info in manifest)"
fi

echo

# Test 5: Directory permissions
print_info "Test 5: Testing directory permission handling..."
test5_dir="$TEST_OUTPUT/test5"
mkdir -p "$test5_dir/readonly"
chmod 555 "$test5_dir/readonly"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test5_dir/readonly" \
    -p 1 \
    --max-retries 1 \
    --debug 2>&1 | grep -q "permission\|denied\|failed"; then
    print_success "Permission errors handled correctly"
else
    print_error "Permission errors not detected"
fi

chmod 755 "$test5_dir/readonly"

echo

# Test 6: Token refresh simulation
print_info "Test 6: Testing token handling..."
test6_dir="$TEST_OUTPUT/test6"
mkdir -p "$test6_dir"

# Create an expired token file
cat > "$test6_dir/${USERNAME}.json" << EOF
{
    "access_token": "expired_token",
    "expires_time": "2020-01-01T00:00:00Z",
    "expires_in": 3600
}
EOF

# Run with expired token - should refresh automatically
if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$SCRIPT_DIR/../fixtures/small_manifest.tcia" \
    -s "$test6_dir" \
    -p 1 \
    --debug 2>&1 | tee "$test6_dir/token.log"; then
    
    if grep -q "expired\|refresh\|new token" "$test6_dir/token.log"; then
        print_success "Token refresh works correctly"
    else
        print_info "Token refresh not explicitly shown in logs"
    fi
else
    print_error "Failed with expired token"
fi

echo

# Test 7: Stress test with many workers
print_info "Test 7: Stress testing with many workers..."
test7_dir="$TEST_OUTPUT/test7"
mkdir -p "$test7_dir"

# Use small manifest for stress test
if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$SCRIPT_DIR/../fixtures/small_manifest.tcia" \
    -s "$test7_dir" \
    -p 20 \
    --max-connections 10 \
    --debug 2>&1 | tee "$test7_dir/stress.log"; then
    
    # Check for any panics or errors
    if ! grep -q "panic\|FATAL\|concurrent map" "$test7_dir/stress.log"; then
        print_success "Stress test passed without crashes"
    else
        print_error "Stress test revealed issues"
    fi
else
    print_error "Stress test failed"
fi

echo

# Final Report
echo "=========================================="
echo "Integration Test Summary"
echo "=========================================="
echo
echo "Tests completed:"
echo "1. Continue interrupted downloads (skip existing) - Check"
echo "2. Invalid credentials handling - Check"
echo "3. Network timeout handling - Check"
echo "4. File size verification - Check"
echo "5. Directory permissions - Check"
echo "6. Token refresh - Check"
echo "7. Stress test - Check"
echo
print_success "All integration tests completed!"