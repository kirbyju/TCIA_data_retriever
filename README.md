# NBIA Data Retriever CLI

> A robust command-line replacement for NBIA Data Retriever with enhanced features and reliability

---

## Features
- **Thread-safe parallel downloads** - Proper concurrency with mutex protection
- **Server-friendly operation** - Configurable delays and connection limits to avoid server issues
- **Automatic retry with exponential backoff** - Handles transient failures gracefully
- **File verification** - Size and MD5 checksum validation
- **MD5 validation mode** - Enhanced integrity checking with server-provided MD5 hashes
- **Flexible storage options** - Choose between extracted DICOM files or compressed ZIP archives
- **Comprehensive test suite** - Smoke, parallel, integration, and performance tests
- **Connection pooling** - HTTP/1.1 with efficient connection reuse
- **Progress tracking** - Real-time download progress with size and speed indicators

## Installation

### 1. Download Pre-built Binary
Download the latest release from the releases page.

### 2. Build from Source

```bash
git clone https://github.com/GrigoryEvko/NBIA_data_retriever_CLI.git
cd NBIA_data_retriever_CLI
go mod tidy   # prepare the dependencies

# Build for current platform
go build -o nbia-data-retriever-cli .

# Or use the Python build script
python build.py --platform linux --arch amd64
```

### 3. Using Docker

```bash
# Build docker image
git clone https://github.com/GrigoryEvko/NBIA_data_retriever_CLI.git
cd NBIA_data_retriever_CLI
docker build -t nbia .

# Run with docker
docker run --rm -v $PWD:$PWD -w $PWD nbia --help
```

## Command Line Usage

```bash
SYNOPSIS:
    nbia-data-retriever-cli [--debug] [--help|-h] [--input|-i <string>]
                    [--output|-s <string>] [--processes|-p <int>]
                    [--user|-u <string>] [--passwd <string>]
                    [--max-connections <int>] [--max-retries <int>]
                    [--server-friendly] [--force|-f] [--skip-existing]
                    [--proxy|-x <string>] [--meta|-m] [--save-log]
                    [--md5] [--no-decompress] [--version|-v]

OPTIONS:
    --debug                 Show debug information (default: false)
    --help|-h               Show help information
    --input|-i <string>     Path to TCIA manifest file (required)
    --output|-s <string>    Output directory (default: "./")
    --processes|-p <int>    Number of parallel downloads (default: 2)
    --user|-u <string>      Username for authentication (default: "nbia_guest")
    --passwd <string>       Password for authentication
    --prompt|-w             Prompt for password
    
    --max-connections <int> Maximum connections per host (default: 8)
    --max-retries <int>     Maximum download retries (default: 3)
    --server-friendly       Use conservative settings to avoid server issues
    --force|-f              Force re-download existing files
    --skip-existing         Skip if image file already exists
    
    --proxy|-x <string>     Proxy URL [http, socks5://user:pass@host:port]
    --meta|-m               Only download metadata
    --save-log              Save debug log to progress.log
    --md5                   Enable MD5 validation for downloaded files
    --no-decompress         Keep downloaded files as ZIP archives (skip extraction)
    --version|-v            Show version information
```

## Basic Usage

```bash
# Download with default settings
./nbia-data-retriever-cli -i manifest.tcia

# Download with authentication
./nbia-data-retriever-cli -i manifest.tcia -u username --passwd password

# Use server-friendly mode for problematic servers
./nbia-data-retriever-cli -i manifest.tcia --server-friendly

# Parallel downloads with 5 workers
./nbia-data-retriever-cli -i manifest.tcia -p 5

# Skip existing files
./nbia-data-retriever-cli -i manifest.tcia --skip-existing

# Force re-download all files
./nbia-data-retriever-cli -i manifest.tcia --force

# Download with MD5 validation
./nbia-data-retriever-cli -i manifest.tcia --md5

# Keep files as ZIP archives (don't extract)
./nbia-data-retriever-cli -i manifest.tcia --no-decompress
```

## Advanced Usage Examples

```bash
# Download to specific directory with debug logging
./nbia-data-retriever-cli -i manifest.tcia -s /data/medical/images --debug --save-log

# Use proxy with authentication
./nbia-data-retriever-cli -i manifest.tcia --proxy socks5://user:pass@proxy.example.com:1080

# Download with custom connection settings
./nbia-data-retriever-cli -i manifest.tcia -p 10 --max-connections 20 --max-retries 5

# Download only metadata (no images)
./nbia-data-retriever-cli -i manifest.tcia --meta -s metadata.json

# Prompt for password (secure input)
./nbia-data-retriever-cli -i manifest.tcia -u myusername --prompt

# Combine multiple options for large datasets
./nbia-data-retriever-cli -i large_dataset.tcia \
  -u myusername \
  --passwd mypassword \
  -s /large/storage/path \
  -p 8 \
  --max-connections 16 \
  --skip-existing \
  --debug \
  --save-log

# Download with custom API endpoints (for private NBIA instances)
./nbia-data-retriever-cli -i manifest.tcia \
  --token-url https://private.nbia.com/oauth/token \
  --image-url https://private.nbia.com/api/v2/getImage \
  --meta-url https://private.nbia.com/api/v2/getSeriesMetaData
```

## Performance Tuning

```bash
# For fast networks and servers (maximize throughput)
./nbia-data-retriever-cli -i manifest.tcia -p 20 --max-connections 40

# For slow/unstable networks (maximize reliability)
./nbia-data-retriever-cli -i manifest.tcia -p 2 --max-connections 4 --max-retries 5

# For rate-limited servers (avoid 429 errors)
./nbia-data-retriever-cli -i manifest.tcia -p 1 --server-friendly

# Balance performance and stability
./nbia-data-retriever-cli -i manifest.tcia -p 5 --max-connections 10 --max-retries 3
```

## Troubleshooting Examples

```bash
# Debug connection issues
./nbia-data-retriever-cli -i manifest.tcia --debug --save-log -p 1

# Test with single file download
head -20 manifest.tcia > test_single.tcia
./nbia-data-retriever-cli -i test_single.tcia --debug

# Verify checksums after download
./nbia-data-retriever-cli -i manifest.tcia --skip-existing --debug
# (will verify existing files and skip if valid)

# Use with system proxy settings
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
./nbia-data-retriever-cli -i manifest.tcia
```

## Server-Friendly Mode

If you experience truncated downloads or server errors, use the `--server-friendly` flag:

```bash
./nbia-data-retriever-cli -i manifest.tcia --server-friendly
```

This enables:
- Single worker (no parallel downloads)
- Reduced connections (2 max)
- Longer retry delays (30s)
- Request delays (2s between requests)

## MD5 Validation and Storage Options

### MD5 Validation Mode

Enable MD5 checksum validation for enhanced data integrity:

```bash
./nbia-data-retriever-cli -i manifest.tcia --md5
```

This mode:
- Uses the NBIA MD5 API endpoint
- Validates each file against server-provided MD5 hashes
- Ensures complete and uncorrupted downloads
- Requires file extraction (incompatible with --no-decompress)

### No-Decompress Mode

Keep downloaded files as ZIP archives without extraction:

```bash
./nbia-data-retriever-cli -i manifest.tcia --no-decompress
```

This mode:
- Preserves original ZIP files from NBIA
- Saves disk space (compressed storage)
- Faster downloads (no extraction time)
- Useful for archival or transfer purposes

### Default Behavior

By default, the tool automatically extracts ZIP files to directories containing DICOM images:
- ZIP files are downloaded to temporary locations
- Files are extracted and verified
- Original ZIP files are deleted after successful extraction
- DICOM files are organized in series-specific directories

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
cd tests
./run_all_tests.sh

# Run specific test suite
./test_smoke.sh              # Basic functionality
./test_parallel.sh           # Parallel download tests
./test_integration.sh        # Edge cases and error handling
./test_md5_nodecompress.sh   # MD5 validation and storage options
./test_performance.sh        # Performance benchmarks
```

## Known Issues

- TCIA servers do not support HTTP Range requests (resumable downloads)
- Some servers may truncate large files - use `--server-friendly` mode
- Server rate limiting may cause failures - adjust `--processes` and `--max-connections`

## Requirements

- Go 1.24.4 or later
- Network connection to TCIA servers
- Valid TCIA manifest file (.tcia)

## License

Same as original project

## Contributing

1. Fork the repository
2. Create your feature branch
3. Run tests to ensure everything works
4. Commit your changes
5. Push to the branch
6. Create a Pull Request