package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadS5cmdSeriesMap loads the mapping file from the output directory.
func loadS5cmdSeriesMap(outputDir string) (map[string]string, error) {
	mapFilePath := filepath.Join(outputDir, "metadata", "s5cmd_series_map.json")
	if _, err := os.Stat(mapFilePath); os.IsNotExist(err) {
		return make(map[string]string), nil // Return empty map if file doesn't exist
	}

	data, err := os.ReadFile(mapFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read s5cmd series map: %w", err)
	}

	var seriesMap map[string]string
	if err := json.Unmarshal(data, &seriesMap); err != nil {
		return nil, fmt.Errorf("could not parse s5cmd series map: %w", err)
	}
	return seriesMap, nil
}

// saveS5cmdSeriesMap saves the mapping file to the output directory.
func saveS5cmdSeriesMap(outputDir string, seriesMap map[string]string) error {
	mapFilePath := filepath.Join(outputDir, "metadata", "s5cmd_series_map.json")
	data, err := json.MarshalIndent(seriesMap, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal s5cmd series map: %w", err)
	}

	return os.WriteFile(mapFilePath, data, 0644)
}

func decodeS5cmd(filePath string, outputDir string, processedSeries map[string]string) ([]*FileInfo, int) {
	file, err := os.Open(filePath)
	if err != nil {
		logger.Fatalf("could not open s5cmd manifest: %v", err)
	}
	defer file.Close()

	var jobsToProcess []*FileInfo
	var newJobs int
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

		if seriesUID, ok := processedSeries[originalURI]; ok {
			// This is a sync job for an existing series
			logger.Infof("Queueing sync job for existing series: %s", originalURI)
			finalDirPath := filepath.Join(outputDir, seriesUID)
			jobsToProcess = append(jobsToProcess, &FileInfo{
				DownloadURL:      originalURI,
				SeriesUID:        seriesUID, // We already know the final UID
				OriginalS5cmdURI: originalURI,
				S5cmdManifestPath: finalDirPath, // The final directory is the target for sync
				IsSyncJob:        true,
			})
		} else {
			// This is a new copy job
			newJobs++
			logger.Infof("Queueing new copy job for series: %s", originalURI)
			cleanURI := strings.TrimSuffix(originalURI, "/*")
			seriesGUID := filepath.Base(cleanURI)
			tempDirName := "s5cmd-tmp-" + seriesGUID
			tempDirPath := filepath.Join(outputDir, tempDirName)

			if err := os.MkdirAll(tempDirPath, 0755); err != nil {
				logger.Warnf("Could not create temp directory for %s: %v", originalURI, err)
				continue
			}

			jobsToProcess = append(jobsToProcess, &FileInfo{
				DownloadURL:      originalURI,
				SeriesUID:        filepath.Base(originalURI), // Temporary ID for progress
				OriginalS5cmdURI: originalURI,
				S5cmdManifestPath: tempDirPath, // The temporary directory is the target for copy
				IsSyncJob:        false,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Fatalf("error reading s5cmd manifest: %v", err)
	}

	logger.Infof("Found %d s5cmd jobs to process (%d new, %d existing)", len(jobsToProcess), newJobs, len(jobsToProcess)-newJobs)
	return jobsToProcess, newJobs
}
