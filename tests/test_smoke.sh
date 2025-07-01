#!/bin/bash
# Smoke test for NBIA downloader
# Tests basic functionality with a small dataset

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NBIA_TOOL="${SCRIPT_DIR}/../nbia-downloader-fixed"
TEST_OUTPUT="${SCRIPT_DIR}/test_output"
MANIFEST="${SCRIPT_DIR}/fixtures/small_manifest.tcia"
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

cleanup() {
    print_info "Cleaning up test directory..."
    rm -rf "$TEST_OUTPUT"
}

# Trap cleanup on exit
trap cleanup EXIT

# Test 1: Check if binary exists
print_info "Test 1: Checking if NBIA downloader exists..."
if [ ! -f "$NBIA_TOOL" ]; then
    print_error "NBIA downloader not found at $NBIA_TOOL"
    exit 1
fi
print_success "NBIA downloader found"

# Test 2: Check help output
print_info "Test 2: Testing help output..."
if "$NBIA_TOOL" --help > /dev/null 2>&1; then
    print_success "Help command works"
else
    # Help might exit with non-zero but that's OK if it shows help
    if "$NBIA_TOOL" --help 2>&1 | grep -q "SYNOPSIS"; then
        print_success "Help command works (shows usage)"
    else
        print_error "Help command failed"
        exit 1
    fi
fi

# Test 3: Basic download with 1 worker
print_info "Test 3: Testing basic download (1 worker)..."
mkdir -p "$TEST_OUTPUT/test3"
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test3" \
    -p 1 \
    --debug; then
    print_error "Basic download failed"
    exit 1
fi

# Check if files were downloaded
file_count=$(find "$TEST_OUTPUT/test3" -name "*.zip" -type f | wc -l)
if [ "$file_count" -eq 0 ]; then
    print_error "No files downloaded"
    exit 1
fi
print_success "Downloaded $file_count files with 1 worker"

# Test 4: Verify skip-existing functionality
print_info "Test 4: Testing skip-existing functionality..."
original_count=$file_count
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test3" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | grep -q "Skip"; then
    print_error "Skip-existing not working"
    exit 1
fi
print_success "Skip-existing works correctly"

# Test 5: Force re-download
print_info "Test 5: Testing force re-download..."
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test5" \
    -p 1 \
    --force \
    --debug; then
    print_error "Force download failed"
    exit 1
fi
print_success "Force download works"

# Test 6: Parallel download with 3 workers
print_info "Test 6: Testing parallel download (3 workers)..."
mkdir -p "$TEST_OUTPUT/test6"
start_time=$(date +%s)
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test6" \
    -p 3 \
    --debug; then
    print_error "Parallel download failed"
    exit 1
fi
end_time=$(date +%s)
duration=$((end_time - start_time))

parallel_count=$(find "$TEST_OUTPUT/test6" -name "*.zip" -type f | wc -l)
print_success "Downloaded $parallel_count files with 3 workers in ${duration}s"

# Test 7: Check summary output
print_info "Test 7: Checking summary output..."
output=$("$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test7" \
    -p 2 \
    --skip-existing 2>&1)

if ! echo "$output" | grep -q "Download Summary"; then
    print_error "Summary output not found"
    exit 1
fi
print_success "Summary output displayed correctly"

# Test 8: Metadata-only download
print_info "Test 8: Testing metadata-only download..."
mkdir -p "$TEST_OUTPUT/test8"
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$TEST_OUTPUT/test8" \
    -p 1 \
    --meta; then
    print_error "Metadata download failed"
    exit 1
fi

json_count=$(find "$TEST_OUTPUT/test8" -name "*.json" -type f | wc -l)
zip_count=$(find "$TEST_OUTPUT/test8" -name "*.zip" -type f | wc -l)

if [ "$json_count" -eq 0 ] || [ "$zip_count" -ne 0 ]; then
    print_error "Metadata-only mode not working correctly"
    exit 1
fi
print_success "Metadata-only download works (found $json_count JSON files)"

# Final summary
echo
echo "======================================"
echo -e "${GREEN}All smoke tests passed!${NC}"
echo "======================================"
echo
echo "Summary:"
echo "- Binary exists and help works"
echo "- Basic download functionality verified"
echo "- Skip-existing feature works"
echo "- Force re-download works"
echo "- Parallel downloads work"
echo "- Summary output displayed"
echo "- Metadata-only mode works"