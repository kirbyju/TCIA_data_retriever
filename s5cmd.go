package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

	var seriesToDownload []*FileInfo
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

		// Create a temporary directory for this series
		tempDirName := "s5cmd-" + filepath.Base(originalURI)
		tempDirName = strings.ReplaceAll(tempDirName, "*", "") // Clean up for valid directory name
		tempDirPath := filepath.Join(outputDir, tempDirName)

		if err := os.MkdirAll(tempDirPath, 0755); err != nil {
			logger.Warnf("Could not create temp directory for %s: %v", originalURI, err)
			continue
		}

		// Create a single FileInfo object for the entire series download
		seriesToDownload = append(seriesToDownload, &FileInfo{
			DownloadURL:      originalURI,
			SeriesUID:        filepath.Base(originalURI), // Use URI base as a temporary ID for progress tracking
			OriginalS5cmdURI: originalURI,
			S5cmdManifestPath: tempDirPath, // This field now holds the target directory for the series download
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd manifest: %w", err)
	}

	// Note: The total number of items for the progress bar will be the number of series, not files.
	// This is a change in behavior but is necessary for the performance improvement.
	logger.Infof("Found %d series to download from s5cmd manifest", len(seriesToDownload))
	return seriesToDownload, nil
}
