# NBIA Data Retriever CLI

> A robust command-line replacement for NBIA Data Retriever with enhanced features and reliability

---

## Features
- **Thread-safe parallel downloads** - Proper concurrency with mutex protection
- **Server-friendly operation** - Configurable delays and connection limits to avoid server issues
- **Automatic retry with exponential backoff** - Handles transient failures gracefully
- **File verification** - Size and MD5 checksum validation
- **Comprehensive test suite** - Smoke, parallel, integration, and performance tests
- **Connection pooling** - HTTP/2 support with efficient connection reuse
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
go build -o nbia-downloader .

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
    nbia-downloader [--debug] [--help|-h] [--input|-i <string>]
                    [--output|-s <string>] [--processes|-p <int>]
                    [--user|-u <string>] [--passwd <string>]
                    [--max-connections <int>] [--max-retries <int>]
                    [--server-friendly] [--force|-f] [--skip-existing]
                    [--proxy|-x <string>] [--meta|-m] [--save-log]
                    [--version|-v]

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
    --version|-v            Show version information
```

## Basic Usage

```bash
# Download with default settings
./nbia-downloader -i manifest.tcia

# Download with authentication
./nbia-downloader -i manifest.tcia -u username --passwd password

# Use server-friendly mode for problematic servers
./nbia-downloader -i manifest.tcia --server-friendly

# Parallel downloads with 5 workers
./nbia-downloader -i manifest.tcia -p 5

# Skip existing files
./nbia-downloader -i manifest.tcia --skip-existing

# Force re-download all files
./nbia-downloader -i manifest.tcia --force
```

## Advanced Usage Examples

```bash
# Download to specific directory with debug logging
./nbia-downloader -i manifest.tcia -s /data/medical/images --debug --save-log

# Use proxy with authentication
./nbia-downloader -i manifest.tcia --proxy socks5://user:pass@proxy.example.com:1080

# Download with custom connection settings
./nbia-downloader -i manifest.tcia -p 10 --max-connections 20 --max-retries 5

# Download only metadata (no images)
./nbia-downloader -i manifest.tcia --meta -s metadata.json

# Prompt for password (secure input)
./nbia-downloader -i manifest.tcia -u myusername --prompt

# Combine multiple options for large datasets
./nbia-downloader -i large_dataset.tcia \
  -u myusername \
  --passwd mypassword \
  -s /large/storage/path \
  -p 8 \
  --max-connections 16 \
  --skip-existing \
  --debug \
  --save-log

# Download with custom API endpoints (for private NBIA instances)
./nbia-downloader -i manifest.tcia \
  --token-url https://private.nbia.com/oauth/token \
  --image-url https://private.nbia.com/api/v2/getImage \
  --meta-url https://private.nbia.com/api/v2/getSeriesMetaData
```

## Performance Tuning

```bash
# For fast networks and servers (maximize throughput)
./nbia-downloader -i manifest.tcia -p 20 --max-connections 40

# For slow/unstable networks (maximize reliability)
./nbia-downloader -i manifest.tcia -p 2 --max-connections 4 --max-retries 5

# For rate-limited servers (avoid 429 errors)
./nbia-downloader -i manifest.tcia -p 1 --server-friendly

# Balance performance and stability
./nbia-downloader -i manifest.tcia -p 5 --max-connections 10 --max-retries 3
```

## Troubleshooting Examples

```bash
# Debug connection issues
./nbia-downloader -i manifest.tcia --debug --save-log -p 1

# Test with single file download
head -20 manifest.tcia > test_single.tcia
./nbia-downloader -i test_single.tcia --debug

# Verify checksums after download
./nbia-downloader -i manifest.tcia --skip-existing --debug
# (will verify existing files and skip if valid)

# Use with system proxy settings
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
./nbia-downloader -i manifest.tcia
```

## Server-Friendly Mode

If you experience truncated downloads or server errors, use the `--server-friendly` flag:

```bash
./nbia-downloader -i manifest.tcia --server-friendly
```

This enables:
- Single worker (no parallel downloads)
- Reduced connections (2 max)
- Longer retry delays (30s)
- Request delays (2s between requests)

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
cd tests
./run_all_tests.sh

# Run specific test suite
./test_smoke.sh          # Basic functionality
./test_parallel.sh       # Parallel download tests
./test_integration.sh    # Edge cases and error handling
./test_performance.sh    # Performance benchmarks
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