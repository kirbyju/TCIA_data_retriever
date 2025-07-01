#!/bin/bash
# Test MD5 validation and no-decompress features for NBIA downloader

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
echo "NBIA MD5 & No-Decompress Features Test"
echo "=========================================="
echo

# Test 1: Default behavior (decompress files)
print_info "Test 1: Testing default behavior (decompress)..."
test1_dir="$TEST_OUTPUT/test1_decompress"
mkdir -p "$test1_dir"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test1_dir" \
    -p 1 \
    --debug 2>&1 | tee "$test1_dir/output.log"; then
    
    # Check for extracted directories
    if find "$test1_dir" -type d -name "1.3.6.1.4.1.14519.*" | grep -q .; then
        print_success "Files extracted to directories (default behavior)"
        
        # Check for DICOM files
        if find "$test1_dir" -name "*.dcm" -o -name "[0-9]*" | grep -q .; then
            print_success "DICOM files found in directories"
        else
            print_error "No DICOM files found"
        fi
    else
        print_error "No extracted directories found"
    fi
else
    print_error "Download failed"
fi

echo

# Test 2: No-decompress mode (keep ZIP files)
print_info "Test 2: Testing no-decompress mode..."
test2_dir="$TEST_OUTPUT/test2_nodecompress"
mkdir -p "$test2_dir"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test2_dir" \
    -p 1 \
    --no-decompress \
    --debug 2>&1 | tee "$test2_dir/output.log"; then
    
    # Check for ZIP files
    if find "$test2_dir" -name "*.zip" | grep -q .; then
        print_success "ZIP files preserved (no-decompress mode)"
        
        # Count ZIP files
        zip_count=$(find "$test2_dir" -name "*.zip" | wc -l)
        print_result "Found $zip_count ZIP files"
        
        # Check that no directories were created
        if find "$test2_dir" -type d -name "1.3.6.1.4.1.14519.*" | grep -q .; then
            print_error "Unexpected directories found (should be ZIP only)"
        else
            print_success "No extraction occurred (correct behavior)"
        fi
    else
        print_error "No ZIP files found"
    fi
else
    print_error "Download failed"
fi

echo

# Test 3: MD5 validation mode
print_info "Test 3: Testing MD5 validation mode..."
test3_dir="$TEST_OUTPUT/test3_md5"
mkdir -p "$test3_dir"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test3_dir" \
    -p 1 \
    --md5 \
    --debug 2>&1 | tee "$test3_dir/output.log"; then
    
    # Check for MD5 validation in logs
    if grep -q "MD5\|hash\|validation\|verified" "$test3_dir/output.log"; then
        print_success "MD5 validation messages found in logs"
        
        # Check if using v1 API endpoint
        if grep -q "v1/getImageWithMD5Hash" "$test3_dir/output.log"; then
            print_success "Using correct v1 MD5 API endpoint"
        else
            print_error "Not using MD5 API endpoint"
        fi
    else
        print_info "MD5 validation not explicitly shown in logs"
    fi
    
    # Files should be extracted when using MD5
    if find "$test3_dir" -type d -name "1.3.6.1.4.1.14519.*" | grep -q .; then
        print_success "Files extracted (required for MD5 validation)"
    else
        print_error "Files not extracted (MD5 requires extraction)"
    fi
else
    print_error "Download failed"
fi

echo

# Test 4: Incompatible options (MD5 + no-decompress)
print_info "Test 4: Testing incompatible options (MD5 + no-decompress)..."
test4_dir="$TEST_OUTPUT/test4_incompatible"
mkdir -p "$test4_dir"

if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test4_dir" \
    -p 1 \
    --md5 \
    --no-decompress \
    --debug 2>&1 | grep -q "incompatible"; then
    print_success "Correctly rejected incompatible options"
else
    print_error "Should have rejected --md5 with --no-decompress"
fi

echo

# Test 5: Skip existing with different modes
print_info "Test 5: Testing skip-existing behavior..."
test5_dir="$TEST_OUTPUT/test5_skip"
mkdir -p "$test5_dir"

# First download with decompress
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test5_dir" \
    -p 1 \
    --debug > "$test5_dir/first_run.log" 2>&1 || true

# Try again with skip-existing
if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test5_dir" \
    -p 1 \
    --skip-existing \
    --debug 2>&1 | tee "$test5_dir/skip_run.log" | grep -q "Skip\|exists"; then
    print_success "Skip-existing works for decompressed files"
fi

# Now try with no-decompress in a new directory
test5b_dir="$TEST_OUTPUT/test5b_skip_zip"
mkdir -p "$test5b_dir"

# First download with no-decompress
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test5b_dir" \
    -p 1 \
    --no-decompress \
    --debug > "$test5b_dir/first_run.log" 2>&1 || true

# Try again with skip-existing
if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test5b_dir" \
    -p 1 \
    --no-decompress \
    --skip-existing \
    --debug 2>&1 | tee "$test5b_dir/skip_run.log" | grep -q "Skip\|exists"; then
    print_success "Skip-existing works for ZIP files"
fi

echo

# Test 6: Force re-download with different modes
print_info "Test 6: Testing force re-download..."
test6_dir="$TEST_OUTPUT/test6_force"
mkdir -p "$test6_dir"

# First download
"$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test6_dir" \
    -p 1 \
    --debug > "$test6_dir/first_run.log" 2>&1 || true

# Force re-download
if "$NBIA_TOOL" -u "$USERNAME" --passwd "$PASSWORD" \
    -i "$MANIFEST" \
    -s "$test6_dir" \
    -p 1 \
    --force \
    --debug 2>&1 | tee "$test6_dir/force_run.log" | grep -q "Force\|re-download"; then
    print_success "Force re-download works"
fi

echo

# Final Report
echo "=========================================="
echo "MD5 & No-Decompress Test Summary"
echo "=========================================="
echo
echo "Tests completed:"
echo "1. Default decompress behavior - Check"
echo "2. No-decompress mode (ZIP preservation) - Check"
echo "3. MD5 validation mode - Check"
echo "4. Incompatible options detection - Check"
echo "5. Skip-existing with both modes - Check"
echo "6. Force re-download - Check"
echo
print_success "All MD5 and no-decompress tests completed!"