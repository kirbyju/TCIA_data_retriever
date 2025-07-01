package main

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
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
	const metadataWorkers = 20
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
				
				resp, err := doRequest(httpClient, req)
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
	return filepath.Join(info.getOutput(output), info.SeriesUID)
}

// NeedsDownload checks if files need to be downloaded
func (info *FileInfo) NeedsDownload(output string, force bool, noDecompress bool) bool {
	if force {
		logger.Debugf("Force flag set, will re-download %s", info.SeriesUID)
		return true
	}
	
	var targetPath string
	if noDecompress {
		// Check for ZIP file
		targetPath = info.DcimFiles(output) + ".zip"
	} else {
		// Check for extracted directory
		targetPath = info.DcimFiles(output)
	}
	
	stat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debugf("Target %s does not exist, need to download", targetPath)
			return true
		}
		logger.Warnf("Error checking target %s: %v", targetPath, err)
		return true
	}
	
	if noDecompress {
		// For ZIP files, check if it's a regular file
		if stat.IsDir() {
			logger.Debugf("%s exists but is a directory, need to re-download", targetPath)
			return true
		}
		// For ZIP files, we can't easily verify the size as it's compressed
		// Just check existence for now
		logger.Debugf("ZIP file %s exists, skipping", targetPath)
		return false
	} else {
		// For extracted files, check if it's a directory
		if !stat.IsDir() {
			logger.Debugf("%s exists but is not a directory, need to re-download", targetPath)
			return true
		}
		
		// Check total size of extracted files
		if info.FileSize != "" {
			expectedSize, err := strconv.ParseInt(info.FileSize, 10, 64)
			if err == nil {
				actualSize, err := getDirectorySize(targetPath)
				if err != nil {
					logger.Warnf("Error calculating directory size for %s: %v", targetPath, err)
					return true
				}
				if actualSize != expectedSize {
					logger.Debugf("Directory %s size mismatch: expected %d, got %d", targetPath, expectedSize, actualSize)
					return true
				}
			}
		}
		
		logger.Debugf("Directory %s exists with correct size, skipping", targetPath)
		return false
	}
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

// extractAndVerifyZip extracts a ZIP file and verifies the total uncompressed size and optional MD5 hashes
func extractAndVerifyZip(zipPath string, destDir string, expectedSize int64, md5Map map[string]string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %v", err)
	}
	defer reader.Close()
	
	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	
	var totalSize int64
	var md5Errors []string
	
	// Check if we're in MD5 validation mode
	md5Mode := md5Map != nil && len(md5Map) > 0
	
	// Extract files
	for _, file := range reader.File {
		// Skip md5hashes.csv if present
		if file.Name == "md5hashes.csv" {
			continue
		}
		
		path := filepath.Join(destDir, file.Name)
		
		// Ensure the file path is within destDir (security check)
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in zip: %s", file.Name)
		}
		
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}
		
		// Create the directory for the file
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create file directory: %v", err)
		}
		
		// Extract file
		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %v", err)
		}
		
		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			return fmt.Errorf("failed to create file: %v", err)
		}
		
		// Check if this file is in the MD5 map (i.e., it's an imaging file)
		isImagingFile := false
		expectedMD5 := ""
		if md5Hash, ok := md5Map[file.Name]; ok {
			isImagingFile = true
			expectedMD5 = md5Hash
		}
		
		// If MD5 validation is needed, use a multi-writer
		var writer io.Writer = targetFile
		var hasher hash.Hash
		if isImagingFile && expectedMD5 != "" {
			hasher = md5.New()
			writer = io.MultiWriter(targetFile, hasher)
		}
		
		written, err := io.Copy(writer, fileReader)
		fileReader.Close()
		targetFile.Close()
		
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %v", file.Name, err)
		}
		
		// Verify MD5 if available
		if hasher != nil && expectedMD5 != "" {
			actualMD5 := hex.EncodeToString(hasher.Sum(nil))
			if actualMD5 != expectedMD5 {
				md5Errors = append(md5Errors, fmt.Sprintf("%s: expected %s, got %s", file.Name, expectedMD5, actualMD5))
			} else {
				logger.Debugf("MD5 verified for %s", file.Name)
			}
		}
		
		// Only count size for imaging files in MD5 mode, or all files in non-MD5 mode
		if md5Mode {
			if isImagingFile {
				totalSize += written
			}
		} else {
			totalSize += written
		}
	}
	
	// Report MD5 errors if any
	if len(md5Errors) > 0 {
		return fmt.Errorf("MD5 validation failed for %d files:\n%s", len(md5Errors), strings.Join(md5Errors, "\n"))
	}
	
	// Verify total size if expected size is provided
	if expectedSize > 0 && totalSize != expectedSize {
		if md5Mode {
			// In MD5 mode, we know exactly which files are imaging files, so size should match
			return fmt.Errorf("size mismatch: expected %d bytes, extracted %d bytes", expectedSize, totalSize)
		} else {
			// In non-MD5 mode, we counted all files including non-imaging files, so just warn
			logger.Warnf("Size mismatch (this may be due to non-imaging files in the archive): expected %d bytes, extracted %d bytes", expectedSize, totalSize)
		}
	}
	
	return nil
}

// getDirectorySize calculates the total size of all files in a directory
func getDirectorySize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// parseMD5HashesCSV parses the md5hashes.csv file from the ZIP and returns a map of filename to MD5 hash
func parseMD5HashesCSV(zipPath string) (map[string]string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %v", err)
	}
	defer reader.Close()
	
	// Find md5hashes.csv in the ZIP
	for _, file := range reader.File {
		if file.Name == "md5hashes.csv" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open md5hashes.csv: %v", err)
			}
			defer rc.Close()
			
			// Parse CSV
			csvReader := csv.NewReader(rc)
			records, err := csvReader.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("failed to parse CSV: %v", err)
			}
			
			// Build map (skip header)
			md5Map := make(map[string]string)
			for i, record := range records {
				if i == 0 || len(record) < 2 {
					continue // Skip header or invalid rows
				}
				filename := record[0]
				md5Hash := record[1]
				md5Map[filename] = md5Hash
			}
			
			return md5Map, nil
		}
	}
	
	return nil, fmt.Errorf("md5hashes.csv not found in ZIP")
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
func (info *FileInfo) Download(output string, httpClient *http.Client, authToken *Token, options *Options) error {
	// Add rate limiting delay between requests
	if options.RequestDelay > 0 {
		time.Sleep(options.RequestDelay)
	}
	return info.DownloadWithRetry(output, httpClient, authToken, options)
}

// DownloadWithRetry downloads file with retry logic and exponential backoff
func (info *FileInfo) DownloadWithRetry(output string, httpClient *http.Client, authToken *Token, options *Options) error {
	var lastErr error
	delay := options.RetryDelay
	
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		if attempt > 0 {
			logger.Infof("Retrying download %s (attempt %d/%d) after %v delay", info.SeriesUID, attempt, options.MaxRetries, delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
		
		err := info.doDownload(output, httpClient, authToken, options)
		if err == nil {
			return nil
		}
		
		lastErr = err
		logger.Warnf("Download %s failed (attempt %d/%d): %v", info.SeriesUID, attempt+1, options.MaxRetries+1, err)
		
		// Check if error is retryable
		if !isRetryableError(err) {
			logger.Errorf("Non-retryable error for %s: %v", info.SeriesUID, err)
			return err
		}
	}
	
	return fmt.Errorf("download failed after %d attempts: %v", options.MaxRetries+1, lastErr)
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// Check for network errors, timeouts, and certain HTTP status codes
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "incomplete download") || // Truncated downloads
		strings.Contains(errStr, "closed") || // Connection closed
		strings.Contains(errStr, "broken pipe") || // Broken connection
		strings.Contains(errStr, "429") || // Rate limiting
		strings.Contains(errStr, "500") || // Server error
		strings.Contains(errStr, "502") || // Bad gateway
		strings.Contains(errStr, "503") || // Service unavailable
		strings.Contains(errStr, "504")    // Gateway timeout
}

// doDownload performs the actual download
func (info *FileInfo) doDownload(output string, httpClient *http.Client, authToken *Token, options *Options) error {
	logger.Debugf("getting image file to %s", output)
	url_, err := makeURL(ImageUrl, map[string]interface{}{"SeriesInstanceUID": info.SeriesUID})
	if err != nil {
		return fmt.Errorf("failed to make URL: %v", err)
	}
	
	// Paths based on decompression mode
	var finalPath string
	var tempZipPath string
	
	if options.NoDecompress {
		// Keep as ZIP file
		finalPath = info.DcimFiles(output) + ".zip"
		tempZipPath = finalPath + ".tmp"
	} else {
		// Extract to directory
		finalPath = info.DcimFiles(output)
		tempZipPath = finalPath + ".zip.tmp"
	}
	
	// Clean up any previous temporary files
	if _, err := os.Stat(tempZipPath); err == nil {
		logger.Debugf("Removing incomplete download: %s", tempZipPath)
		os.Remove(tempZipPath)
	}
	
	// For extraction mode, also clean up temporary extraction directory
	if !options.NoDecompress {
		tempExtractDir := finalPath + ".uncompressed.tmp"
		if _, err := os.Stat(tempExtractDir); err == nil {
			logger.Debugf("Removing incomplete extraction: %s", tempExtractDir)
			os.RemoveAll(tempExtractDir)
		}
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

	// Set timeout based on file size (if known)
	var timeout time.Duration
	if info.FileSize != "" {
		fileSize, _ := strconv.ParseInt(info.FileSize, 10, 64)
		// Calculate timeout: base 5 minutes + 1 minute per 100MB
		timeout = 5*time.Minute + time.Duration(fileSize/(100*1024*1024))*time.Minute
		// Cap at 60 minutes for very large files
		if timeout > 60*time.Minute {
			timeout = 60*time.Minute
		}
	} else {
		// Default timeout for unknown size
		timeout = 30*time.Minute
	}
	logger.Debugf("Setting download timeout to %v for %s", timeout, info.SeriesUID)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := doRequest(httpClient, req)
	if err != nil {
		return fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()
	
	// Log response headers for debugging
	logger.Debugf("Response headers for %s: Status=%s, Content-Length=%d, Transfer-Encoding=%s", 
		info.SeriesUID, resp.Status, resp.ContentLength, resp.Header.Get("Transfer-Encoding"))

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Create new temp ZIP file
	f, err := os.OpenFile(tempZipPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer func() {
		f.Close()
		// Clean up temp files on error
		if err != nil {
			os.Remove(tempZipPath)
			if !options.NoDecompress {
				tempExtractDir := finalPath + ".uncompressed.tmp"
				os.RemoveAll(tempExtractDir)
			}
		}
	}()

	// Create progress bar
	var totalSize int64 = resp.ContentLength
	
	// Handle Content-Length edge cases
	if totalSize < 0 {
		logger.Warnf("Server did not provide Content-Length for %s", info.SeriesUID)
		// Use FileSize from manifest if available
		if fSize, err := strconv.ParseInt(info.FileSize, 10, 64); err == nil && fSize > 0 {
			totalSize = fSize
			logger.Debugf("Using manifest size %d for %s", totalSize, info.SeriesUID)
		} else {
			// Unknown size - use a large default for progress bar
			totalSize = 1024 * 1024 * 1024 // 1GB default
			logger.Warnf("Unknown file size for %s, using default progress bar", info.SeriesUID)
		}
	} else if info.FileSize != "" {
		// Verify Content-Length matches manifest
		if fSize, err := strconv.ParseInt(info.FileSize, 10, 64); err == nil && fSize != totalSize {
			logger.Warnf("Size mismatch for %s: manifest=%d, Content-Length=%d", 
				info.SeriesUID, fSize, totalSize)
			// Use the larger value
			if fSize > totalSize {
				totalSize = fSize
			}
		}
	}
	
	bar := bytesBar(totalSize, info.SeriesUID)
	
	// Buffer the response body for better handling of chunked transfers
	bufferedReader := bufio.NewReaderSize(resp.Body, 64*1024) // 64KB buffer
	
	// Download with progress
	written, err := io.Copy(io.MultiWriter(f, bar), bufferedReader)
	if err != nil {
		// Log detailed error information
		logger.Errorf("Download error for %s: %v (written=%d bytes)", info.SeriesUID, err, written)
		// Check if it's an EOF error (connection closed)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			logger.Errorf("Connection closed prematurely by server for %s", info.SeriesUID)
		}
		return fmt.Errorf("failed to write data after %d bytes: %v", written, err)
	}
	
	logger.Debugf("Downloaded %d bytes for %s", written, info.SeriesUID)
	
	// Note: FileSize in manifest is the uncompressed size, but we download ZIP files
	// So we cannot validate the downloaded size against FileSize
	// Log the download completion instead
	if info.FileSize != "" {
		expectedSize, _ := strconv.ParseInt(info.FileSize, 10, 64)
		compressionRatio := float64(written) / float64(expectedSize) * 100
		logger.Infof("Downloaded %s: %d bytes (%.1f%% of uncompressed size %d)", 
			info.SeriesUID, written, compressionRatio, expectedSize)
	}
	
	// Close ZIP file before extraction
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %v", err)
	}
	
	if options.NoDecompress {
		// No decompression mode: just move the ZIP file to final location
		
		// Remove any existing file
		if _, err := os.Stat(finalPath); err == nil {
			logger.Debugf("Removing existing file: %s", finalPath)
			if err := os.Remove(finalPath); err != nil {
				return fmt.Errorf("failed to remove existing file: %v", err)
			}
		}
		
		// Atomic rename from temp to final location
		if err := os.Rename(tempZipPath, finalPath); err != nil {
			return fmt.Errorf("failed to move ZIP file: %v", err)
		}
		
		logger.Infof("Successfully saved %s as %s", info.SeriesUID, finalPath)
		return nil
	} else {
		// Decompression mode: extract and verify
		tempExtractDir := finalPath + ".uncompressed.tmp"
		
		// Extract and verify the ZIP file
		expectedSize := int64(0)
		if info.FileSize != "" {
			expectedSize, _ = strconv.ParseInt(info.FileSize, 10, 64)
		}
		
		// Parse MD5 hashes if MD5 validation is enabled
		var md5Map map[string]string
		if options.MD5 {
			md5Map, err = parseMD5HashesCSV(tempZipPath)
			if err != nil {
				logger.Warnf("Failed to parse MD5 hashes: %v", err)
				// Continue without MD5 validation
				md5Map = nil
			}
		}
		
		logger.Debugf("Extracting %s to %s", tempZipPath, tempExtractDir)
		if err := extractAndVerifyZip(tempZipPath, tempExtractDir, expectedSize, md5Map); err != nil {
			// Clean up temp files on extraction failure
			logger.Errorf("Extraction failed, cleaning up temporary files")
			if removeErr := os.Remove(tempZipPath); removeErr != nil {
				logger.Warnf("Failed to remove temp ZIP after extraction error: %v", removeErr)
			}
			if removeErr := os.RemoveAll(tempExtractDir); removeErr != nil {
				logger.Warnf("Failed to remove temp extract dir after error: %v", removeErr)
			}
			return fmt.Errorf("failed to extract/verify ZIP: %v", err)
		}
		
		// Remove any existing output directory
		if _, err := os.Stat(finalPath); err == nil {
			logger.Debugf("Removing existing directory: %s", finalPath)
			if err := os.RemoveAll(finalPath); err != nil {
				return fmt.Errorf("failed to remove existing directory: %v", err)
			}
		}
		
		// Atomic rename from temp extraction to final location
		if err := os.Rename(tempExtractDir, finalPath); err != nil {
			// Clean up on rename failure
			logger.Errorf("Rename failed, cleaning up temporary files")
			if removeErr := os.RemoveAll(tempExtractDir); removeErr != nil {
				logger.Warnf("Failed to remove temp extract dir after rename error: %v", removeErr)
			}
			if removeErr := os.Remove(tempZipPath); removeErr != nil {
				logger.Warnf("Failed to remove temp ZIP after rename error: %v", removeErr)
			}
			return fmt.Errorf("failed to move extracted files: %v", err)
		}
		
		// Clean up the temporary ZIP file
		if err := os.Remove(tempZipPath); err != nil {
			logger.Warnf("Failed to remove temporary ZIP file %s: %v", tempZipPath, err)
		}
		
		logger.Infof("Successfully extracted %s to %s", info.SeriesUID, finalPath)
		return nil
	}
}
