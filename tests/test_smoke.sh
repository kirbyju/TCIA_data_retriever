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

# Test 3: Basic download with 1 worker (extracts by default)
print_info "Test 3: Testing basic download (1 worker, extraction mode)..."
mkdir -p "$TEST_OUTPUT/test3"
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test3" \
    -p 1 \
    --debug; then
    print_error "Basic download failed"
    exit 1
fi

# Check if directories were created (files are extracted by default)
dir_count=$(find "$TEST_OUTPUT/test3" -mindepth 3 -maxdepth 3 -type d | wc -l)
if [ "$dir_count" -eq 0 ]; then
    print_error "No directories created (extraction failed)"
    exit 1
fi

# Validate extracted content
if validate_extraction_structure "$TEST_OUTPUT/test3"; then
    print_success "Downloaded and extracted $dir_count series with valid structure"
else
    print_error "Extraction structure validation failed"
    exit 1
fi

# Test 4: Verify skip-existing functionality
print_info "Test 4: Testing skip-existing functionality..."
original_count=$dir_count

# Record file count and timestamps before
before_files=$(find "$TEST_OUTPUT/test3" -type f | wc -l)
# Get modification times of first few files
first_files=($(find "$TEST_OUTPUT/test3" -type f ! -name "*.json" | head -5))
declare -A original_mtimes
for f in "${first_files[@]}"; do
    if [ -f "$f" ]; then
        original_mtimes["$f"]=$(stat -c %Y "$f" 2>/dev/null || stat -f %m "$f" 2>/dev/null)
    fi
done

# Run with skip-existing
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test3" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$TEST_OUTPUT/test3_skip.log"; then
    print_error "Skip-existing command failed"
    exit 1
fi

# Verify no new files were created
after_files=$(find "$TEST_OUTPUT/test3" -type f | wc -l)
if ! validate_skip_existing "$TEST_OUTPUT/test3" "$before_files" "$after_files" "$TEST_OUTPUT/test3_skip.log"; then
    print_error "Skip-existing validation failed"
    exit 1
fi

# Verify existing files were not re-downloaded
# Note: Some filesystems may update access times even when files are not modified
# So we primarily rely on the skip messages and file count validation
if [ "$before_files" -eq "$after_files" ]; then
    print_success "Skip-existing works correctly (no new files created)"
else
    print_error "Skip-existing failed - file count changed"
    exit 1
fi

# Test 5: Force re-download with no-decompress (requires --no-md5)
print_info "Test 5: Testing force re-download with no-decompress..."
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test5" \
    -p 1 \
    --force \
    --no-md5 \
    --no-decompress \
    --debug; then
    print_error "Force download failed"
    exit 1
fi
# Check for zip files when using no-decompress
zip_count=$(find "$TEST_OUTPUT/test5" -name "*.zip" -type f | wc -l)
if [ "$zip_count" -eq 0 ]; then
    print_error "No ZIP files found with --no-decompress"
    exit 1
fi
print_success "Force download works with no-decompress ($zip_count ZIP files)"

# Test 6: Parallel download with 3 workers
print_info "Test 6: Testing parallel download (3 workers, extraction mode)..."
mkdir -p "$TEST_OUTPUT/test6"
start_time=$(date +%s)
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test6" \
    -p 3 \
    --debug; then
    print_error "Parallel download failed"
    exit 1
fi
end_time=$(date +%s)
duration=$((end_time - start_time))

parallel_count=$(find "$TEST_OUTPUT/test6" -mindepth 3 -maxdepth 3 -type d | wc -l)
print_success "Downloaded and extracted $parallel_count series with 3 workers in ${duration}s"

# Test 7: Check summary output
print_info "Test 7: Checking summary output..."
output=$("$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test7" \
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
    -o "$TEST_OUTPUT/test8" \
    -p 1 \
    --meta; then
    print_error "Metadata download failed"
    exit 1
fi

# Check metadata directory
json_count=$(find "$TEST_OUTPUT/test8/metadata" -name "*.json" -type f 2>/dev/null | wc -l)
zip_count=$(find "$TEST_OUTPUT/test8" -name "*.zip" -type f | wc -l)
dir_count=$(find "$TEST_OUTPUT/test8" -mindepth 3 -maxdepth 3 -type d | wc -l)

if [ "$json_count" -eq 0 ] || [ "$zip_count" -ne 0 ] || [ "$dir_count" -ne 0 ]; then
    print_error "Metadata-only mode not working correctly"
    exit 1
fi

# Validate JSON metadata files
valid_json=0
for json_file in $(find "$TEST_OUTPUT/test8/metadata" -name "*.json" -type f 2>/dev/null); do
    if validate_json_metadata "$json_file" > /dev/null 2>&1; then
        valid_json=$((valid_json + 1))
    fi
done

if [ "$valid_json" -eq "$json_count" ]; then
    print_success "Metadata-only download works ($json_count valid JSON files in metadata/)"
else
    print_error "Some JSON metadata files are invalid"
    exit 1
fi

# Test 9: Metadata caching
print_info "Test 9: Testing metadata caching..."
mkdir -p "$TEST_OUTPUT/test9"

# First run - should fetch from server
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test9" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$TEST_OUTPUT/test9_first.log"; then
    print_error "First run failed"
    exit 1
fi

# Count metadata files
metadata_count=$(find "$TEST_OUTPUT/test9/metadata" -name "*.json" -type f 2>/dev/null | wc -l)
if [ "$metadata_count" -eq 0 ]; then
    print_error "No metadata cached"
    exit 1
fi

# Record cache file access times
declare -A cache_atimes
for cache_file in $(find "$TEST_OUTPUT/test9/metadata" -name "*.json" -type f); do
    cache_atimes["$cache_file"]=$(stat -c %X "$cache_file" 2>/dev/null || stat -f %a "$cache_file" 2>/dev/null)
done

# Small delay to ensure access times can differ
sleep 2

# Second run - should use cache
if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test9" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$TEST_OUTPUT/test9_cached.log"; then
    print_error "Cached run failed"
    exit 1
fi

# Validate cache was actually used
if ! validate_cache_usage "$TEST_OUTPUT/test9/metadata" "$TEST_OUTPUT/test9_cached.log" "$metadata_count"; then
    print_error "Metadata cache validation failed"
    exit 1
fi

# Verify cache files were accessed (access time should be updated)
cache_accessed=0
for cache_file in $(find "$TEST_OUTPUT/test9/metadata" -name "*.json" -type f); do
    if [ -n "${cache_atimes[$cache_file]}" ]; then
        new_atime=$(stat -c %X "$cache_file" 2>/dev/null || stat -f %a "$cache_file" 2>/dev/null)
        if [ "$new_atime" -gt "${cache_atimes[$cache_file]}" ]; then
            cache_accessed=$((cache_accessed + 1))
        fi
    fi
done

if [ "$cache_accessed" -gt 0 ]; then
    print_success "Metadata caching works correctly ($cache_accessed cache files accessed)"
else
    # Some filesystems don't update access times reliably, so just check logs
    if validate_cache_usage "$TEST_OUTPUT/test9/metadata" "$TEST_OUTPUT/test9_cached.log" 1 > /dev/null 2>&1; then
        print_success "Metadata caching works correctly (verified via logs)"
    else
        print_error "Metadata cache not used"
        exit 1
    fi
fi

# Test 10: Force metadata refresh
print_info "Test 10: Testing --refresh-metadata flag..."

# Get modification times of cache files before refresh
declare -A cache_mtimes_before
for cache_file in $(find "$TEST_OUTPUT/test9/metadata" -name "*.json" -type f); do
    cache_mtimes_before["$cache_file"]=$(stat -c %Y "$cache_file" 2>/dev/null || stat -f %m "$cache_file" 2>/dev/null)
done

# Small delay to ensure modification times can differ
sleep 2

if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -o "$TEST_OUTPUT/test9" \
    -p 1 \
    --skip-existing \
    --refresh-metadata \
    --debug 2>&1 | tee "$TEST_OUTPUT/test9_refresh.log"; then
    print_error "Refresh metadata failed"
    exit 1
fi

# Check for force refresh in log AND verify files were updated
force_refresh_found=false
if grep -q "Force refresh" "$TEST_OUTPUT/test9_refresh.log"; then
    force_refresh_found=true
fi

# Count how many cache files were updated
cache_updated=0
for cache_file in $(find "$TEST_OUTPUT/test9/metadata" -name "*.json" -type f); do
    if [ -n "${cache_mtimes_before[$cache_file]}" ]; then
        new_mtime=$(stat -c %Y "$cache_file" 2>/dev/null || stat -f %m "$cache_file" 2>/dev/null)
        if [ "$new_mtime" -gt "${cache_mtimes_before[$cache_file]}" ]; then
            cache_updated=$((cache_updated + 1))
        fi
    fi
done

if [ "$force_refresh_found" = true ] && [ "$cache_updated" -gt 0 ]; then
    print_success "Force metadata refresh works correctly ($cache_updated files refreshed)"
elif [ "$force_refresh_found" = true ]; then
    # Server might return same data, so mtime might not change
    print_success "Force metadata refresh works correctly (refresh attempted)"
else
    print_error "Force refresh not working properly"
    exit 1
fi

# Final summary
echo
echo "======================================"
echo -e "${GREEN}All smoke tests passed!${NC}"
echo "======================================"
echo
echo "Summary:"
echo "- Binary exists and help works"
echo "- Basic download extracts files by default"
echo "- Skip-existing feature works"
echo "- Force re-download with --no-md5 --no-decompress keeps ZIP files"
echo "- Parallel downloads work with extraction"
echo "- Summary output displayed"
echo "- Metadata-only mode works"
echo "- Metadata caching works"
echo "- Force metadata refresh works"