package main

import (
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	// version and build info
	buildStamp string
	gitHash    string
	goVersion  string
	version    string
	client     *http.Client
	token      *Token
	logger     *zap.SugaredLogger
)

// DownloadStats tracks download statistics
type DownloadStats struct {
	Total      int32
	Downloaded int32
	Skipped    int32
	Failed     int32
}

// WorkerContext contains all dependencies for workers
type WorkerContext struct {
	HTTPClient *http.Client
	AuthToken  *Token
	Options    *Options
	Stats      *DownloadStats
	WorkerID   int
}

// SetupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. We then handle this by calling
// our clean-up procedure and exiting the program.
func setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		os.Exit(0)
	}()
}

func main() {
	setupCloseHandler()

	var options = InitOptions()

	if options.Version {
		logger.Infof("Current version: %s", version)
		logger.Infof("Git Commit Hash: %s", gitHash)
		logger.Infof("UTC Build Time : %s", buildStamp)
		logger.Infof("Golang Version : %s", goVersion)
		os.Exit(0)
	} else {
		client = newClient(options.Proxy, options.MaxConnsPerHost)

		err := os.MkdirAll(options.Output, os.ModePerm)
		if err != nil {
			logger.Fatalf("failed to create output directory: %v", err)
		}
		token, err = NewToken(
			options.Username, options.Password,
			filepath.Join(options.Output, fmt.Sprintf("%s.json", options.Username)))

		if err != nil {
			logger.Fatal(err)
		}

		var wg sync.WaitGroup
		files := decodeTCIA(options.Input, client, token)
		stats := &DownloadStats{Total: int32(len(files))}

		wg.Add(options.Concurrent)
		inputChan := make(chan *FileInfo, len(files)) // Larger buffer to prevent blocking
		
		// Create worker contexts
		for i := 0; i < options.Concurrent; i++ {
			ctx := &WorkerContext{
				HTTPClient: client,
				AuthToken:  token,
				Options:    options,
				Stats:      stats,
				WorkerID:   i + 1,
			}

			go func(ctx *WorkerContext, input chan *FileInfo) {
				defer wg.Done()
				for fileInfo := range input {
					logger.Debugf("[Worker %d] Processing %s", ctx.WorkerID, fileInfo.SeriesUID)
					
					if ctx.Options.Meta {
						// Only download metadata
						if err := fileInfo.GetMeta(ctx.Options.Output); err != nil {
							logger.Warnf("[Worker %d] Save meta info %s failed - %s", ctx.WorkerID, fileInfo.SeriesUID, err)
							atomic.AddInt32(&ctx.Stats.Failed, 1)
						} else {
							atomic.AddInt32(&ctx.Stats.Downloaded, 1)
						}
					} else {
						// Download images (and save metadata)
						if ctx.Options.SkipExisting && !fileInfo.NeedsDownload(ctx.Options.Output, false) {
							logger.Infof("[Worker %d] Skip existing %s", ctx.WorkerID, fileInfo.SeriesUID)
							atomic.AddInt32(&ctx.Stats.Skipped, 1)
							continue
						}
						
						if fileInfo.NeedsDownload(ctx.Options.Output, ctx.Options.Force) {
							if err := fileInfo.Download(ctx.Options.Output, ctx.HTTPClient, ctx.AuthToken, ctx.Options.MaxRetries, ctx.Options.RetryDelay, ctx.Options.RequestDelay); err != nil {
								logger.Warnf("[Worker %d] Download %s failed - %s", ctx.WorkerID, fileInfo.SeriesUID, err)
								atomic.AddInt32(&ctx.Stats.Failed, 1)
							} else {
								// Save metadata after successful download
								if err := fileInfo.GetMeta(ctx.Options.Output); err != nil {
									logger.Warnf("[Worker %d] Save meta info %s failed - %s", ctx.WorkerID, fileInfo.SeriesUID, err)
								}
								atomic.AddInt32(&ctx.Stats.Downloaded, 1)
							}
						} else {
							logger.Infof("[Worker %d] Skip %s (already exists with correct size/checksum)", ctx.WorkerID, fileInfo.SeriesUID)
							atomic.AddInt32(&ctx.Stats.Skipped, 1)
						}
					}
				}
			}(ctx, inputChan)
		}

		for _, f := range files {
			inputChan <- f
		}
		close(inputChan)
		wg.Wait()
		
		// Print summary
		logger.Infof("\n=== Download Summary ===")
		logger.Infof("Total files: %d", stats.Total)
		logger.Infof("Downloaded: %d", stats.Downloaded)
		logger.Infof("Skipped: %d", stats.Skipped)
		logger.Infof("Failed: %d", stats.Failed)
		
		if stats.Failed > 0 {
			logger.Warnf("Some downloads failed. Check the logs above for details.")
		}
	}
}
