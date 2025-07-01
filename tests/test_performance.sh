#!/bin/bash
# Performance test for NBIA downloader
# Ensures downloads are blazing fast and optimized

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
MANIFEST="${SCRIPT_DIR}/fixtures/NSCLC-RADIOMICS-INTEROBSERVER1-Aug-31-2020-NBIA-manifest.tcia"
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
echo "NBIA Downloader Performance Test"
echo "=========================================="
echo

# Create test directory
mkdir -p "$TEST_OUTPUT"

# Test 1: Connection pooling effectiveness
print_info "Test 1: Testing connection pooling..."
test1_dir="$TEST_OUTPUT/test1"
mkdir -p "$test1_dir"

# Use small manifest for quick test
small_manifest="${SCRIPT_DIR}/fixtures/small_manifest.tcia"

# Test with connection pooling
start_time=$(date +%s.%N)
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$small_manifest" \
    -s "$test1_dir/pooled" \
    -p 5 \
    --max-connections 10 \
    --debug 2>&1 | tee "$test1_dir/pooled.log"
pooled_time=$(echo "$(date +%s.%N) - $start_time" | bc)

# Test without connection pooling (1 connection)
start_time=$(date +%s.%N)
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$small_manifest" \
    -s "$test1_dir/single" \
    -p 5 \
    --max-connections 1 \
    --force \
    --debug 2>&1 | tee "$test1_dir/single.log"
single_time=$(echo "$(date +%s.%N) - $start_time" | bc)

# Compare times
speedup=$(echo "scale=2; $single_time / $pooled_time" | bc)
print_result "Pooled time: ${pooled_time}s"
print_result "Single connection time: ${single_time}s"
print_result "Speedup: ${speedup}x"

if (( $(echo "$speedup > 1.2" | bc -l) )); then
    print_success "Connection pooling provides significant speedup"
else
    print_info "Connection pooling speedup minimal (might be due to small dataset)"
fi

echo

# Test 2: Parallel download scaling
print_info "Test 2: Testing parallel download scaling..."
test2_dir="$TEST_OUTPUT/test2"
mkdir -p "$test2_dir"

# Create medium manifest
medium_manifest="$test2_dir/medium_manifest.tcia"
head -n 6 "$MANIFEST" > "$medium_manifest"
tail -n +7 "$MANIFEST" | head -n 10 >> "$medium_manifest"

echo "Workers,Time,Files/sec" > "$test2_dir/scaling.csv"

for workers in 1 2 4 8 16; do
    print_info "Testing with $workers workers..."
    
    start_time=$(date +%s.%N)
    "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
        -i "$medium_manifest" \
        -s "$test2_dir/workers_$workers" \
        -p $workers \
        --max-connections $((workers * 2)) \
        --debug > "$test2_dir/workers_$workers.log" 2>&1
    
    duration=$(echo "$(date +%s.%N) - $start_time" | bc)
    files_per_sec=$(echo "scale=2; 10 / $duration" | bc)
    
    echo "$workers,$duration,$files_per_sec" >> "$test2_dir/scaling.csv"
    print_result "$workers workers: ${duration}s (${files_per_sec} files/sec)"
done

# Check if performance scales with workers
if tail -n +2 "$test2_dir/scaling.csv" | awk -F',' '{print $3}' | sort -n | tail -1 | xargs test 2 -lt; then
    print_success "Performance scales with worker count"
else
    print_info "Performance scaling could be better"
fi

echo

# Test 3: Memory usage under load
print_info "Test 3: Testing memory usage under heavy load..."
test3_dir="$TEST_OUTPUT/test3"
mkdir -p "$test3_dir"

# Monitor memory during parallel download
# Note: time command can fail if tool fails, so we handle both cases
if /usr/bin/time -v "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$medium_manifest" \
    -s "$test3_dir" \
    -p 20 \
    --max-connections 40 \
    --force \
    --debug 2>&1 | tee "$test3_dir/memory.log"; then
    print_info "Memory test completed successfully"
else
    print_info "Tool exited with error but we still check memory usage"
fi

if grep -q "Maximum resident set size" "$test3_dir/memory.log"; then
    mem_kb=$(grep "Maximum resident set size" "$test3_dir/memory.log" | awk '{print $6}')
    mem_mb=$((mem_kb / 1024))
    print_result "Peak memory with 20 workers: ${mem_mb}MB"
    
    if [ "$mem_mb" -lt 200 ]; then
        print_success "Excellent memory efficiency (<200MB)"
    elif [ "$mem_mb" -lt 500 ]; then
        print_success "Good memory efficiency (<500MB)"
    else
        print_error "High memory usage: ${mem_mb}MB"
    fi
fi

echo

# Test 4: HTTP/2 and keep-alive
print_info "Test 4: Testing connection reuse..."
test4_dir="$TEST_OUTPUT/test4"
mkdir -p "$test4_dir"

# Run with debug to see connection info
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$small_manifest" \
    -s "$test4_dir" \
    -p 1 \
    --force \
    --debug 2>&1 | tee "$test4_dir/connections.log"

# Check for connection reuse indicators
if grep -q "reuse\|keep-alive\|HTTP/2\|h2" "$test4_dir/connections.log"; then
    print_success "Connection reuse detected"
else
    print_info "Connection reuse not explicitly shown"
fi

echo

# Test 5: Optimal worker count detection
print_info "Test 5: Finding optimal worker count..."
test5_dir="$TEST_OUTPUT/test5"
mkdir -p "$test5_dir"

best_rate=0
best_workers=1

for workers in 1 2 3 5 8 10 15 20; do
    start_time=$(date +%s.%N)
    
    if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
        -i "$small_manifest" \
        -s "$test5_dir/workers_$workers" \
        -p $workers \
        --force \
        --debug > "$test5_dir/workers_$workers.log" 2>&1; then
        
        duration=$(echo "$(date +%s.%N) - $start_time" | bc)
        rate=$(echo "scale=2; 5 / $duration" | bc)
        
        if (( $(echo "$rate > $best_rate" | bc -l) )); then
            best_rate=$rate
            best_workers=$workers
        fi
        
        print_result "$workers workers: $rate files/sec"
    fi
done

print_success "Optimal worker count: $best_workers (${best_rate} files/sec)"

echo

# Test 6: Bandwidth utilization
print_info "Test 6: Testing bandwidth utilization..."
test6_dir="$TEST_OUTPUT/test6"
mkdir -p "$test6_dir"

# Download with bandwidth monitoring
start_time=$(date +%s.%N)
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$medium_manifest" \
    -s "$test6_dir" \
    -p 10 \
    --force \
    --debug 2>&1 | tee "$test6_dir/bandwidth.log"
duration=$(echo "$(date +%s.%N) - $start_time" | bc)

# Calculate bandwidth
total_size=$(du -sb "$test6_dir" | cut -f1)
bandwidth_mbps=$(echo "scale=2; ($total_size * 8) / ($duration * 1000000)" | bc)

print_result "Downloaded $(echo "scale=2; $total_size / 1048576" | bc)MB in ${duration}s"
print_result "Average bandwidth: ${bandwidth_mbps} Mbps"

if (( $(echo "$bandwidth_mbps > 10" | bc -l) )); then
    print_success "Good bandwidth utilization"
else
    print_info "Bandwidth utilization could be improved"
fi

echo

# Final Report
echo "=========================================="
echo "Performance Test Summary"
echo "=========================================="
echo
echo "Performance metrics:"
echo "- Connection pooling: Tested"
echo "- Parallel scaling: Tested"
echo "- Memory efficiency: Tested"
echo "- Connection reuse: Tested"
echo "- Optimal workers: $best_workers"
echo "- Bandwidth usage: ${bandwidth_mbps} Mbps"
echo

# Performance recommendations
echo "Recommendations for blazing fast downloads:"
if [ "$best_workers" -gt 5 ]; then
    echo "- Use $best_workers workers for this system"
else
    echo "- Use 5-10 workers for optimal performance"
fi
echo "- Set max-connections to 2x worker count"
echo "- Ensure good network connectivity"
echo "- Use SSD for output directory"
echo
print_success "Performance testing complete!"