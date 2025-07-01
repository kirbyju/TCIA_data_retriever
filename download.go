package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Directory creation mutex
	dirMutex sync.Mutex
)

// decodeTCIA is used to decode the tcia file with parallel metadata fetching
func decodeTCIA(path string, httpClient *http.Client, authToken *Token) []*FileInfo {
	logger.Debugf("decoding tcia file: %s", path)
	
	f, err := os.Open(path)
	if err != nil {
		logger.Fatal(err)
	}
	defer f.Close()

	// First, collect all series IDs
	seriesIDs := make([]string, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.ContainsAny(line, "=") {
			seriesIDs = append(seriesIDs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Errorf("error reading tcia file: %v", err)
	}
	
	logger.Infof("Found %d series to fetch metadata for", len(seriesIDs))
	
	// Use parallel workers to fetch metadata
	const metadataWorkers = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]*FileInfo, 0)
	
	// Create a channel for series IDs
	idChan := make(chan string, len(seriesIDs))
	for _, id := range seriesIDs {
		idChan <- id
	}
	close(idChan)
	
	// Start workers
	wg.Add(metadataWorkers)
	for i := 0; i < metadataWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			
			for seriesID := range idChan {
				logger.Debugf("[Meta Worker %d] Fetching metadata for: %s", workerID, seriesID)
				
				url_, err := makeURL(MetaUrl, map[string]interface{}{"SeriesInstanceUID": seriesID})
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to make URL: %v", workerID, err)
					continue
				}
				
				req, err := http.NewRequest("GET", url_, nil)
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to create request: %v", workerID, err)
					continue
				}
				
				// Get current access token
				accessToken, err := authToken.GetAccessToken()
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to get access token: %v", workerID, err)
					continue
				}
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
				
				// Set timeout for metadata request
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				req = req.WithContext(ctx)
				
				resp, err := httpClient.Do(req)
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to do request: %v", workerID, err)
					continue
				}
				
				content, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to read response data: %v", workerID, err)
					continue
				}
				
				files := make([]*FileInfo, 0)
				err = json.Unmarshal(content, &files)
				if err != nil {
					logger.Errorf("[Meta Worker %d] Failed to parse response data: %v", workerID, err)
					logger.Debugf("%s", content)
					continue
				}
				
				// Thread-safe append to results
				mu.Lock()
				results = append(results, files...)
				mu.Unlock()
			}
		}(i + 1)
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	logger.Infof("Successfully fetched metadata for %d files", len(results))
	return results
}

type FileInfo struct {
	NumberOfImages     string `json:"Number of Images"`
	SOPClassUID        string `json:"SOP Class UID"`
	Manufacturer       string `json:"Manufacturer"`
	DataDescriptionURI string `json:"Data Description URI"`
	LicenseURL         string `json:"License URL"`
	AnnotationSize     string `json:"Annotation Size"`
	Collection         string `json:"Collection"`
	StudyDescription   string `json:"Study Description"`
	SeriesUID          string `json:"Series UID"`
	StudyUID           string `json:"Study UID"`
	LicenseName        string `json:"License Name"`
	StudyDate          string `json:"Study Date"`
	SeriesDescription  string `json:"Series Description"`
	Modality           string `json:"Modality"`
	RdPartyAnalysis    string `json:"3rd Party Analysis"`
	FileSize           string `json:"File Size"`
	SubjectID          string `json:"Subject ID"`
	SeriesNumber       string `json:"Series Number"`
	MD5Hash            string `json:"MD5 Hash,omitempty"`
}

// GetOutput construct the output directory (thread-safe)
func (info *FileInfo) getOutput(output string) string {
	outputDir := filepath.Join(output, info.SubjectID, info.StudyDate)

	// Check if directory exists without lock first
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		return outputDir
	}
	
	// Directory doesn't exist, acquire lock to create it
	dirMutex.Lock()
	defer dirMutex.Unlock()
	
	// Double-check after acquiring lock
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err = os.MkdirAll(outputDir, 0755); err != nil {
			logger.Fatal(err)
		}
	}

	return outputDir
}

func (info *FileInfo) MetaFile(output string) string {
	return filepath.Join(info.getOutput(output), fmt.Sprintf("%s.json", info.SeriesUID))
}

func (info *FileInfo) DcimFiles(output string) string {
	return filepath.Join(info.getOutput(output), fmt.Sprintf("%s.zip", info.SeriesUID))
}

// NeedsDownload checks if file needs to be downloaded
func (info *FileInfo) NeedsDownload(output string, force bool) bool {
	if force {
		logger.Debugf("Force flag set, will re-download %s", info.SeriesUID)
		return true
	}
	
	filePath := info.DcimFiles(output)
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debugf("File %s does not exist, need to download", filePath)
			return true
		}
		logger.Warnf("Error checking file %s: %v", filePath, err)
		return true
	}
	
	// Check file size
	expectedSize, err := strconv.ParseInt(info.FileSize, 10, 64)
	if err == nil && stat.Size() != expectedSize {
		logger.Debugf("File %s size mismatch: expected %d, got %d", filePath, expectedSize, stat.Size())
		return true
	}
	
	// If we have MD5, verify it
	if info.MD5Hash != "" {
		actualMD5, err := calculateFileMD5(filePath)
		if err != nil {
			logger.Warnf("Error calculating MD5 for %s: %v", filePath, err)
			return true
		}
		if actualMD5 != info.MD5Hash {
			logger.Debugf("File %s MD5 mismatch: expected %s, got %s", filePath, info.MD5Hash, actualMD5)
			return true
		}
	}
	
	logger.Debugf("File %s exists with correct size/checksum, skipping", filePath)
	return false
}

// calculateFileMD5 calculates MD5 hash of a file
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (info *FileInfo) GetMeta(output string) error {
	logger.Debugf("getting meta information and save to %s", output)
	f, err := os.OpenFile(info.MetaFile(output), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open meta file %s: %v", info.MetaFile(output), err)
	}
	content, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshall meta: %v", err)
	}
	_, err = f.Write(content)
	if err != nil {
		return err
	}
	return f.Close()
}

// Download is real function to download file with retry logic
func (info *FileInfo) Download(output string, httpClient *http.Client, authToken *Token, maxRetries int, retryDelay time.Duration, requestDelay time.Duration) error {
	// Add rate limiting delay between requests
	if requestDelay > 0 {
		time.Sleep(requestDelay)
	}
	return info.DownloadWithRetry(output, httpClient, authToken, maxRetries, retryDelay)
}

// DownloadWithRetry downloads file with retry logic and exponential backoff
func (info *FileInfo) DownloadWithRetry(output string, httpClient *http.Client, authToken *Token, maxRetries int, initialDelay time.Duration) error {
	var lastErr error
	delay := initialDelay
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.Infof("Retrying download %s (attempt %d/%d) after %v delay", info.SeriesUID, attempt, maxRetries, delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
		
		err := info.doDownload(output, httpClient, authToken)
		if err == nil {
			return nil
		}
		
		lastErr = err
		logger.Warnf("Download %s failed (attempt %d/%d): %v", info.SeriesUID, attempt+1, maxRetries+1, err)
		
		// Check if error is retryable
		if !isRetryableError(err) {
			logger.Errorf("Non-retryable error for %s: %v", info.SeriesUID, err)
			return err
		}
	}
	
	return fmt.Errorf("download failed after %d attempts: %v", maxRetries+1, lastErr)
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// Check for network errors, timeouts, and certain HTTP status codes
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "size mismatch") || // Truncated downloads
		strings.Contains(errStr, "429") || // Rate limiting
		strings.Contains(errStr, "500") || // Server error
		strings.Contains(errStr, "502") || // Bad gateway
		strings.Contains(errStr, "503") || // Service unavailable
		strings.Contains(errStr, "504")    // Gateway timeout
}

// doDownload performs the actual download
func (info *FileInfo) doDownload(output string, httpClient *http.Client, authToken *Token) error {
	logger.Debugf("getting image file to %s", output)
	url_, err := makeURL(ImageUrl, map[string]interface{}{"SeriesInstanceUID": info.SeriesUID})
	if err != nil {
		return fmt.Errorf("failed to make URL: %v", err)
	}
	
	outputPath := info.DcimFiles(output)
	tempPath := outputPath + ".tmp"
	
	// Always start fresh - TCIA server doesn't support resumable downloads
	if _, err := os.Stat(tempPath); err == nil {
		logger.Debugf("Removing incomplete download: %s", tempPath)
		os.Remove(tempPath)
	}
	
	req, err := http.NewRequest("GET", url_, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	// Get current access token
	accessToken, err := authToken.GetAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Set timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Create new temp file
	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer func() {
		f.Close()
		// Clean up temp file on error
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Create progress bar
	var totalSize int64 = resp.ContentLength
	
	// Use FileSize from manifest if available and larger
	if fSize, err := strconv.ParseInt(info.FileSize, 10, 64); err == nil && fSize > 0 {
		totalSize = fSize
	}
	
	bar := bytesBar(totalSize, info.SeriesUID)
	
	// Download with progress
	written, err := io.Copy(io.MultiWriter(f, bar), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}
	
	// Verify size if available
	if info.FileSize != "" {
		expectedSize, _ := strconv.ParseInt(info.FileSize, 10, 64)
		if expectedSize > 0 && written != expectedSize {
			return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, written)
		}
	}
	
	// Close file before rename
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %v", err)
	}
	
	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("failed to rename file: %v", err)
	}
	
	return nil
}
