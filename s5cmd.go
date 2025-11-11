package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// s5cmdSeriesMap stores the mapping from original S3 URI to SeriesInstanceUID
var s5cmdSeriesMap = make(map[string]string)
var s5cmdSeriesMapMutex sync.Mutex

// loadS5cmdSeriesMap loads the mapping file from the output directory
func loadS5cmdSeriesMap(outputDir string) (map[string]string, error) {
	mapFilePath := filepath.Join(outputDir, ".s5cmd_series_map.json")
	s5cmdSeriesMapMutex.Lock()
	defer s5cmdSeriesMapMutex.Unlock()

	if _, err := os.Stat(mapFilePath); os.IsNotExist(err) {
		return make(map[string]string), nil // Return empty map if file doesn't exist
	}

	data, err := os.ReadFile(mapFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read s5cmd series map: %w", err)
	}

	if err := json.Unmarshal(data, &s5cmdSeriesMap); err != nil {
		return nil, fmt.Errorf("could not parse s5cmd series map: %w", err)
	}
	return s5cmdSeriesMap, nil
}

// saveS5cmdSeriesMap saves the mapping file to the output directory
func saveS5cmdSeriesMap(outputDir string) error {
	mapFilePath := filepath.Join(outputDir, ".s5cmd_series_map.json")
	s5cmdSeriesMapMutex.Lock()
	defer s5cmdSeriesMapMutex.Unlock()

	data, err := json.MarshalIndent(s5cmdSeriesMap, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal s5cmd series map: %w", err)
	}

	return os.WriteFile(mapFilePath, data, 0644)
}

// updateS5cmdSeriesMap adds a new entry to the map
func updateS5cmdSeriesMap(originalURI, seriesUID string) {
	s5cmdSeriesMapMutex.Lock()
	defer s5cmdSeriesMapMutex.Unlock()
	s5cmdSeriesMap[originalURI] = seriesUID
}

// expandS5cmdURI expands a wildcard URI using "s5cmd ls"
func expandS5cmdURI(s3uri string) ([]string, error) {
	if !strings.Contains(s3uri, "*") {
		return []string{s3uri}, nil
	}

	// Extract bucket name to reconstruct the full URI later
	uriParts := strings.SplitN(strings.TrimPrefix(s3uri, "s3://"), "/", 2)
	if len(uriParts) < 1 {
		return nil, fmt.Errorf("invalid s3 uri for bucket extraction: %s", s3uri)
	}
	bucket := uriParts[0]

	cmd := exec.Command("s5cmd", "--no-sign-request", "--endpoint-url", "https://s3.amazonaws.com", "ls", s3uri)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("s5cmd ls failed for %s: %v", s3uri, err)
	}

	var expandedFiles []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 4 { // s5cmd ls output has at least 4 fields
			// The object key starts from the 4th field to the end
			objectKey := strings.Join(parts[3:], " ")
			// Reconstruct the full s3:// URI
			fullURI := fmt.Sprintf("s3://%s/%s", bucket, objectKey)
			expandedFiles = append(expandedFiles, fullURI)
		}
	}
	return expandedFiles, nil
}

func decodeS5cmd(filePath string, outputDir string) ([]*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open s5cmd manifest: %w", err)
	}
	defer file.Close()

	// Load the series map to skip already processed URIs
	processedURIs, err := loadS5cmdSeriesMap(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load s5cmd series map: %w", err)
	}

	var filesToDownload []*FileInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		var originalURI string
		if len(parts) >= 2 && parts[0] == "cp" {
			originalURI = parts[1]
		} else if len(parts) == 1 && strings.HasPrefix(parts[0], "s3://") {
			originalURI = parts[0]
		} else {
			continue // Skip comments and invalid lines
		}

		// Check if this URI has already been processed
		if _, ok := processedURIs[originalURI]; ok {
			logger.Infof("Skipping already processed series: %s", originalURI)
			continue
		}

		// Expand the URI if it contains a wildcard
		expandedURIs, err := expandS5cmdURI(originalURI)
		if err != nil {
			logger.Warnf("Could not expand URI %s: %v", originalURI, err)
			continue
		}

		// Create a temporary directory for this series
		tempDirName := "s5cmd-" + filepath.Base(originalURI)
		// Clean up wildcards for valid directory names
		tempDirName = strings.ReplaceAll(tempDirName, "*", "")
		tempDirPath := filepath.Join(outputDir, tempDirName)

		if err := os.MkdirAll(tempDirPath, 0755); err != nil {
			logger.Warnf("Could not create temp directory for %s: %v", originalURI, err)
			continue
		}

		for _, expandedURI := range expandedURIs {
			filesToDownload = append(filesToDownload, &FileInfo{
				DownloadURL:      expandedURI,
				SeriesUID:        filepath.Base(expandedURI), // Temporary UID for progress
				OriginalS5cmdURI: originalURI,
				// Store the temporary directory path in S5cmdManifestPath for the worker to use
				S5cmdManifestPath: tempDirPath,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd manifest: %w", err)
	}

	logger.Infof("Found %d files to download from s5cmd manifest", len(filesToDownload))
	return filesToDownload, nil
}
