#!/bin/bash
# Helper functions for test validation

# Validate that a directory contains DICOM files
validate_dicom_directory() {
    local dir="$1"
    local min_files="${2:-1}"  # Minimum expected files, default 1
    
    if [ ! -d "$dir" ]; then
        echo "ERROR: Directory $dir does not exist"
        return 1
    fi
    
    # Count .dcm files
    local dcm_count=$(find "$dir" -name "*.dcm" -type f | wc -l)
    
    # If no .dcm files, check for files without extension (common for DICOM)
    if [ "$dcm_count" -eq 0 ]; then
        # Count files that look like DICOM (numeric names or no extension)
        dcm_count=$(find "$dir" -type f ! -name "*.json" ! -name "*.txt" ! -name "*.zip" | wc -l)
    fi
    
    if [ "$dcm_count" -lt "$min_files" ]; then
        echo "ERROR: Expected at least $min_files DICOM files in $dir, found $dcm_count"
        return 1
    fi
    
    # Sample check: verify first file is valid DICOM by checking for DICM marker
    local first_file=$(find "$dir" -type f ! -name "*.json" ! -name "*.txt" ! -name "*.zip" | head -1)
    if [ -n "$first_file" ]; then
        # DICOM files should have "DICM" at offset 128
        if ! dd if="$first_file" bs=1 skip=128 count=4 2>/dev/null | grep -q "DICM"; then
            echo "WARNING: File $first_file may not be a valid DICOM file"
        fi
    fi
    
    echo "OK: Found $dcm_count DICOM files in $dir"
    return 0
}

# Validate JSON metadata file
validate_json_metadata() {
    local json_file="$1"
    
    if [ ! -f "$json_file" ]; then
        echo "ERROR: JSON file $json_file does not exist"
        return 1
    fi
    
    # Check if valid JSON
    if ! python3 -m json.tool "$json_file" > /dev/null 2>&1; then
        echo "ERROR: $json_file is not valid JSON"
        return 1
    fi
    
    # Check for required fields
    local required_fields=("Series UID" "Study UID" "Modality" "File Size")
    for field in "${required_fields[@]}"; do
        if ! grep -q "\"$field\"" "$json_file"; then
            echo "ERROR: Required field '$field' missing from $json_file"
            return 1
        fi
    done
    
    echo "OK: JSON metadata valid in $json_file"
    return 0
}

# Compare file counts before and after to ensure skip-existing works
validate_skip_existing() {
    local dir="$1"
    local before_count="$2"
    local after_count="$3"
    local log_file="$4"
    
    if [ "$after_count" -ne "$before_count" ]; then
        echo "ERROR: File count changed from $before_count to $after_count (should remain same)"
        return 1
    fi
    
    # Check for skip messages in log
    if [ -f "$log_file" ]; then
        local skip_count=$(grep -c "Skip\|already exists\|skipping" "$log_file" || echo 0)
        if [ "$skip_count" -eq 0 ]; then
            echo "ERROR: No skip messages found in log"
            return 1
        fi
        echo "OK: Found $skip_count skip messages in log"
    fi
    
    echo "OK: Skip-existing validation passed"
    return 0
}

# Validate that extraction produced expected structure
validate_extraction_structure() {
    local base_dir="$1"
    
    # Check for expected directory structure: base/SubjectID/StudyDate/SeriesUID/
    local series_dirs=$(find "$base_dir" -mindepth 3 -maxdepth 3 -type d)
    
    if [ -z "$series_dirs" ]; then
        echo "ERROR: No series directories found at expected depth"
        return 1
    fi
    
    local valid_count=0
    local total_count=0
    
    for series_dir in $series_dirs; do
        total_count=$((total_count + 1))
        # Each series directory should contain DICOM files
        if validate_dicom_directory "$series_dir" 1 > /dev/null 2>&1; then
            valid_count=$((valid_count + 1))
        fi
        
        # Check for metadata JSON in parent directory
        local parent_dir=$(dirname "$series_dir")
        local series_name=$(basename "$series_dir")
        local json_file="$parent_dir/${series_name}.json"
        
        if [ ! -f "$json_file" ]; then
            echo "WARNING: Missing metadata file $json_file"
        fi
    done
    
    echo "OK: Validated $valid_count/$total_count series directories"
    if [ "$valid_count" -ne "$total_count" ]; then
        return 1
    fi
    return 0
}

# Check MD5 validation actually happened
validate_md5_verification() {
    local log_file="$1"
    
    if [ ! -f "$log_file" ]; then
        echo "ERROR: Log file $log_file not found"
        return 1
    fi
    
    # Look for actual MD5 verification messages
    if grep -q "MD5 verified\|checksum.*match\|validation.*passed" "$log_file"; then
        echo "OK: MD5 verification messages found"
        return 0
    else
        echo "WARNING: No explicit MD5 verification messages found"
        # Check if using MD5 endpoint
        if grep -q "getImageWithMD5Hash" "$log_file"; then
            echo "OK: Using MD5 validation endpoint"
            return 0
        fi
        return 1
    fi
}