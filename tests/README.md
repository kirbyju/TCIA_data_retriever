# NBIA Downloader Test Suite

This test suite ensures the NBIA downloader works correctly in various scenarios.

## Test Structure

```
tests/
├── test_smoke.sh           # Basic functionality tests
├── test_parallel.sh        # Parallel download performance tests
├── integration/
│   └── test_integration.sh # Full integration tests
└── fixtures/
    ├── small_manifest.tcia # 5 series for quick tests
    └── medium_manifest.tcia # 20 series (created by test_parallel.sh)
```

## Running Tests

### Prerequisites
- Set environment variables for credentials:
  ```bash
  export NBIA_USER=your_username
  export NBIA_PASS=your_password
  ```
  (defaults to `nbia_guest` with empty password)

### Quick Smoke Test
```bash
./tests/test_smoke.sh
```
Tests basic functionality with a small dataset (5 series).

### Parallel Performance Test
```bash
./tests/test_parallel.sh
```
Tests download performance with 1, 2, 5, and 10 workers.

### Full Integration Test
```bash
./tests/integration/test_integration.sh
```
Tests edge cases, error handling, and real-world scenarios.

### Run All Tests
```bash
cd tests
./test_smoke.sh && ./test_parallel.sh && ./integration/test_integration.sh
```

## Test Coverage

### Smoke Tests
- Binary existence and help
- Basic download (1 worker)
- Skip-existing functionality
- Force re-download
- Parallel downloads (3 workers)
- Summary output
- Metadata-only mode

### Parallel Tests
- Performance with different worker counts (1, 2, 5, 10)
- Download consistency verification
- Race condition detection
- Connection pool limiting
- Performance metrics and optimal worker count

### Integration Tests
- Skip already downloaded files
- Invalid credentials handling
- Network timeout handling
- File size verification
- Directory permission errors
- Token refresh mechanism
- Stress test with many workers

## Interpreting Results

### Success Indicators
- ✓ Green checkmarks indicate passed tests
- Summary statistics show downloaded/skipped/failed counts
- Performance metrics help optimize worker count

### Common Issues
- **Authentication failures**: Check credentials
- **Network timeouts**: May indicate server issues
- **Permission errors**: Ensure write access to output directory
- **Race conditions**: Check logs for panic messages

## Adding New Tests

1. Create a new test script in appropriate directory
2. Follow the color coding convention:
   - Green (✓) for success
   - Red (✗) for errors
   - Yellow (→) for info
   - Blue for results
3. Include cleanup functions
4. Add test to this README

## Test Data

The test suite uses the NSCLC-Radiomics-Interobserver1 dataset:
- Small manifest: 5 series
- Medium manifest: 20 series  
- Full manifest: 64 series

This dataset is publicly available and good for testing.