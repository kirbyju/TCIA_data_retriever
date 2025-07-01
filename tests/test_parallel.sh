#!/bin/bash
# Parallel download test for NBIA downloader
# Tests performance and correctness with different worker counts

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NBIA_TOOL="${SCRIPT_DIR}/../nbia-downloader-fixed"
TEST_OUTPUT="${SCRIPT_DIR}/test_output"
MANIFEST="${SCRIPT_DIR}/fixtures/NSCLC-RADIOMICS-INTEROBSERVER1-Aug-31-2020-NBIA-manifest.tcia"
USERNAME="${NBIA_USER:-nbia_guest}"
PASSWORD="${NBIA_PASS:-}"

# Create a medium-size manifest (20 series)
MEDIUM_MANIFEST="${SCRIPT_DIR}/fixtures/medium_manifest.tcia"

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

create_medium_manifest() {
    head -n 6 "$MANIFEST" > "$MEDIUM_MANIFEST"
    tail -n +7 "$MANIFEST" | head -n 20 >> "$MEDIUM_MANIFEST"
}

test_worker_count() {
    local workers=$1
    local test_dir="$TEST_OUTPUT/workers_${workers}"
    
    print_info "Testing with $workers worker(s)..."
    mkdir -p "$test_dir"
    
    # Time the download
    local start_time=$(date +%s.%N)
    
    if ! "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
        -i "$MEDIUM_MANIFEST" \
        -s "$test_dir" \
        -p "$workers" \
        --max-connections "$((workers * 5))" \
        --debug 2>&1 | tee "$test_dir/output.log"; then
        print_error "Download with $workers workers failed"
        return 1
    fi
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    
    # Count downloaded files
    local file_count=$(find "$test_dir" -name "*.zip" -type f | wc -l)
    local total_size=$(du -sh "$test_dir" | cut -f1)
    
    # Extract summary from log
    local downloaded=$(grep "Downloaded:" "$test_dir/output.log" | tail -1 | awk '{print $2}')
    local skipped=$(grep "Skipped:" "$test_dir/output.log" | tail -1 | awk '{print $2}')
    local failed=$(grep "Failed:" "$test_dir/output.log" | tail -1 | awk '{print $2}')
    
    # Calculate rate
    local rate=$(echo "scale=2; $file_count / $duration" | bc)
    
    print_result "Duration: ${duration}s"
    print_result "Files: $file_count"
    print_result "Size: $total_size"
    print_result "Rate: $rate files/sec"
    print_result "Summary - Downloaded: $downloaded, Skipped: $skipped, Failed: $failed"
    
    echo "$workers,$duration,$file_count,$rate,$downloaded,$skipped,$failed" >> "$TEST_OUTPUT/results.csv"
    
    return 0
}

verify_consistency() {
    print_info "Verifying download consistency across different worker counts..."
    
    # Get file lists from each test
    local base_files=$(find "$TEST_OUTPUT/workers_1" -name "*.zip" -type f | sed 's|.*/||' | sort)
    local consistent=true
    
    for workers in 2 5 10; do
        local test_files=$(find "$TEST_OUTPUT/workers_$workers" -name "*.zip" -type f 2>/dev/null | sed 's|.*/||' | sort)
        
        if [ "$base_files" != "$test_files" ]; then
            print_error "File list mismatch between 1 worker and $workers workers"
            consistent=false
        fi
    done
    
    if $consistent; then
        print_success "All worker configurations downloaded the same files"
    fi
    
    return 0
}

# Trap cleanup on exit
trap cleanup EXIT

# Setup
print_info "Setting up test environment..."
mkdir -p "$TEST_OUTPUT"
create_medium_manifest

# Create results file
echo "Workers,Duration,Files,Rate,Downloaded,Skipped,Failed" > "$TEST_OUTPUT/results.csv"

# Test different worker counts
echo
echo "======================================"
echo "Testing Parallel Download Performance"
echo "======================================"
echo

# Test with 1, 2, 5, and 10 workers
for workers in 1 2 5 10; do
    test_worker_count $workers
    echo
done

# Verify consistency
echo
verify_consistency

# Generate performance report
echo
echo "======================================"
echo "Performance Summary"
echo "======================================"
echo
echo "Worker Performance Comparison:"
echo "------------------------------"
column -t -s',' "$TEST_OUTPUT/results.csv" | sed 's/^/  /'

# Find optimal worker count
optimal=$(tail -n +2 "$TEST_OUTPUT/results.csv" | sort -t',' -k4 -nr | head -1 | cut -d',' -f1)
echo
print_success "Optimal worker count: $optimal workers"

# Check for race conditions
echo
print_info "Checking for race conditions..."
error_count=$(grep -c "panic\|race\|concurrent map" "$TEST_OUTPUT"/*/output.log 2>/dev/null || echo 0)
if [ "$error_count" -eq 0 ]; then
    print_success "No race conditions detected"
else
    print_error "Found $error_count potential race conditions"
fi

# Test connection limits
echo
print_info "Testing connection pool limits..."
test_dir="$TEST_OUTPUT/conn_test"
mkdir -p "$test_dir"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MEDIUM_MANIFEST" \
    -s "$test_dir" \
    -p 20 \
    --max-connections 5 \
    --debug 2>&1 | tee "$test_dir/output.log"; then
    print_success "Connection limiting works correctly"
else
    print_error "Connection limiting test failed"
fi

echo
echo "======================================"
echo -e "${GREEN}Parallel download tests completed!${NC}"
echo "======================================" 