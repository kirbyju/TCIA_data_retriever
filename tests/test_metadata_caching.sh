#!/bin/bash
# Test metadata caching functionality for NBIA downloader

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NBIA_TOOL="${SCRIPT_DIR}/../nbia-data-retriever-cli"
TEST_OUTPUT="${SCRIPT_DIR}/test_output"
MANIFEST="${SCRIPT_DIR}/fixtures/small_manifest.tcia"
USERNAME="${NBIA_USER:-nbia_guest}"
PASSWORD="${NBIA_PASS:-}"

# Source helper functions
source "${SCRIPT_DIR}/test_helpers.sh"

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
echo "NBIA Metadata Caching Test"
echo "=========================================="
echo

# Test 1: Initial metadata fetch (cold cache)
print_info "Test 1: Initial metadata fetch (cold cache)..."
test1_dir="$TEST_OUTPUT/test1_cold"
mkdir -p "$test1_dir"

start_time=$(date +%s)
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$test1_dir" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$test1_dir/output.log"; then
    print_error "Initial fetch failed"
    exit 1
fi
end_time=$(date +%s)
cold_duration=$((end_time - start_time))

# Check metadata directory was created
if [ ! -d "$test1_dir/metadata" ]; then
    print_error "Metadata directory not created"
    exit 1
fi

# Count cached metadata files
cached_count=$(find "$test1_dir/metadata" -name "*.json" -type f | wc -l)
if [ "$cached_count" -eq 0 ]; then
    print_error "No metadata files cached"
    exit 1
fi

print_success "Cold cache: Downloaded $cached_count metadata files in ${cold_duration}s"

echo

# Test 2: Cached metadata fetch (warm cache)
print_info "Test 2: Cached metadata fetch (warm cache)..."

start_time=$(date +%s)
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$test1_dir" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$test1_dir/output_cached.log"; then
    print_error "Cached fetch failed"
    exit 1
fi
end_time=$(date +%s)
warm_duration=$((end_time - start_time))

# Validate cache was actually used
if ! validate_cache_usage "$test1_dir/metadata" "$test1_dir/output_cached.log" "$cached_count"; then
    print_error "Cache validation failed - cache not used as expected"
    exit 1
fi

cache_hits=$(grep -c "Loaded metadata from cache" "$test1_dir/output_cached.log" || echo 0)
print_success "Warm cache: Used $cache_hits cached metadata files in ${warm_duration}s"
if [ "$warm_duration" -lt "$cold_duration" ]; then
    print_result "Speed improvement: $((cold_duration - warm_duration))s faster"
fi

echo

# Test 3: Force metadata refresh
print_info "Test 3: Testing --refresh-metadata flag..."

# Save original modification time of a metadata file
test_file=$(find "$test1_dir/metadata" -name "*.json" -type f | head -1)
if [ -n "$test_file" ]; then
    original_mtime=$(stat -c %Y "$test_file" 2>/dev/null || stat -f %m "$test_file" 2>/dev/null)
    sleep 2  # Ensure time difference
fi

if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$test1_dir" \
    -p 1 \
    --skip-existing \
    --refresh-metadata \
    --debug 2>&1 | tee "$test1_dir/output_refresh.log"; then
    print_error "Refresh metadata failed"
    exit 1
fi

# Check for force refresh messages
if ! grep -q "Force refresh" "$test1_dir/output_refresh.log"; then
    print_error "Force refresh not working - no refresh messages in log"
    exit 1
fi

# Verify file was updated
files_refreshed=0
if [ -n "$test_file" ] && [ -f "$test_file" ]; then
    new_mtime=$(stat -c %Y "$test_file" 2>/dev/null || stat -f %m "$test_file" 2>/dev/null)
    if [ "$new_mtime" -gt "$original_mtime" ]; then
        files_refreshed=1
    fi
fi

# Also check that we didn't use cache (should have fetch messages)
fetch_count=$(grep -c "Fetching metadata\|metadata.*from.*server" "$test1_dir/output_refresh.log" || echo 0)
if [ "$fetch_count" -gt 0 ] || [ "$files_refreshed" -eq 1 ]; then
    print_success "Force refresh working correctly (fetched from server)"
else
    print_error "Force refresh may not be working - no evidence of server fetches"
    exit 1
fi

echo

# Test 4: Partial cache (add new series to manifest)
print_info "Test 4: Testing partial cache (mixed cached/new series)..."
test4_dir="$TEST_OUTPUT/test4_partial"
mkdir -p "$test4_dir"

# Use medium manifest with more series
MEDIUM_MANIFEST="${SCRIPT_DIR}/fixtures/medium_manifest.tcia"
if [ ! -f "$MEDIUM_MANIFEST" ]; then
    # Create if doesn't exist
    head -n 6 "$MANIFEST" > "$MEDIUM_MANIFEST"
    tail -n +7 "${SCRIPT_DIR}/fixtures/NSCLC-RADIOMICS-INTEROBSERVER1-Aug-31-2020-NBIA-manifest.tcia" | head -n 20 >> "$MEDIUM_MANIFEST"
fi

# First download subset
head -n 10 "$MEDIUM_MANIFEST" > "$test4_dir/subset.tcia"
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$test4_dir/subset.tcia" \
    -o "$test4_dir" \
    -p 1 \
    --skip-existing \
    --debug > "$test4_dir/subset.log" 2>&1; then
    print_error "Subset download failed"
    exit 1
fi

initial_cache=$(find "$test4_dir/metadata" -name "*.json" -type f | wc -l)

# Now download full manifest
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MEDIUM_MANIFEST" \
    -o "$test4_dir" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$test4_dir/full.log"; then
    print_error "Full manifest download failed"
    exit 1
fi

final_cache=$(find "$test4_dir/metadata" -name "*.json" -type f | wc -l)
new_fetches=$((final_cache - initial_cache))

# Check for mix of cache hits and fetches
cache_hits=$(grep -c "Loaded metadata from cache" "$test4_dir/full.log" || echo 0)
cache_misses=$(grep -c "Cache miss\|Fetching metadata" "$test4_dir/full.log" || echo 0)

# For partial cache, we MUST have both hits and new fetches
if [ "$cache_hits" -eq 0 ]; then
    print_error "Partial cache test failed - no cache hits detected"
    exit 1
elif [ "$new_fetches" -eq 0 ]; then
    print_error "Partial cache test failed - no new metadata fetched"
    exit 1
fi

print_success "Partial cache working correctly: $cache_hits hits, $new_fetches new fetches"

echo

# Test 5: Concurrent cache access
print_info "Test 5: Testing concurrent cache access..."
test5_dir="$TEST_OUTPUT/test5_concurrent"
mkdir -p "$test5_dir"

# Run multiple instances in parallel
(
    "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
        -i "$MANIFEST" \
        -o "$test5_dir" \
        -p 2 \
        --skip-existing \
        --debug > "$test5_dir/concurrent1.log" 2>&1
) &
pid1=$!

(
    "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
        -i "$MANIFEST" \
        -o "$test5_dir" \
        -p 2 \
        --skip-existing \
        --debug > "$test5_dir/concurrent2.log" 2>&1
) &
pid2=$!

# Wait for both to complete
wait $pid1
result1=$?
wait $pid2
result2=$?

if [ "$result1" -eq 0 ] && [ "$result2" -eq 0 ]; then
    print_success "Concurrent cache access handled correctly"
    
    # Check for any corruption
    for json_file in "$test5_dir/metadata"/*.json; do
        if [ -f "$json_file" ] && ! jq empty "$json_file" 2>/dev/null; then
            print_error "Corrupted JSON file detected: $json_file"
            exit 1
        fi
    done
else
    print_error "Concurrent access failed"
    exit 1
fi

echo

# Test 6: Invalid cache handling
print_info "Test 6: Testing corrupted cache handling..."
test6_dir="$TEST_OUTPUT/test6_corrupt"
mkdir -p "$test6_dir/metadata"

# Create a corrupted cache file
echo "invalid json {" > "$test6_dir/metadata/test.invalid.json"

# Try to use it
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$test6_dir" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$test6_dir/corrupt.log"; then
    print_error "Failed to handle corrupted cache"
    exit 1
fi

# Should fetch fresh metadata when cache is invalid
if ! grep -q "Cache miss\|Failed to load\|Fetching metadata\|Error.*cache" "$test6_dir/corrupt.log"; then
    print_error "Corrupted cache not handled properly - no recovery messages"
    exit 1
fi

# Verify we got valid metadata despite corruption
valid_json_count=$(find "$test6_dir/metadata" -name "*.json" -type f -exec sh -c 'jq empty "{}" 2>/dev/null && echo valid' \; | grep -c valid || echo 0)
if [ "$valid_json_count" -eq 0 ]; then
    print_error "No valid JSON files after corrupted cache recovery"
    exit 1
fi

print_success "Corrupted cache handled gracefully (recovered with $valid_json_count valid files)"

echo
echo "=========================================="
echo "Metadata Caching Test Summary"
echo "=========================================="
echo
echo "Tests completed:"
echo "1. Cold cache fetch - Verified metadata saved"
echo "2. Warm cache fetch - Verified cache actually used"
echo "3. Force refresh - Verified server fetch occurred"
echo "4. Partial cache - Verified mixed cache/fetch behavior"
echo "5. Concurrent access - Verified no corruption"
echo "6. Corrupted cache - Verified graceful recovery"
echo
print_success "All metadata caching tests passed with strict validation!"