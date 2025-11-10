package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// downloadS5cmdManifest is now obsolete. The download is handled by the worker pool.
func (info *FileInfo) downloadS5cmdManifest(output string, options *Options) error {
	// This function is no longer used.
	return nil
}

func decodeS5cmd(filePath string) ([]*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open s5cmd manifest: %w", err)
	}
	defer file.Close()

	var files []*FileInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		// s5cmd manifests can have 'cp' or other commands. We are interested in lines like:
		// cp s3://bucket/key local/path
		// or just the s3 uri
		var s3uri string
		if len(parts) >= 2 && parts[0] == "cp" {
			s3uri = parts[1]
		} else if len(parts) == 1 && strings.HasPrefix(parts[0], "s3://") {
			s3uri = parts[0]
		} else {
			continue
		}

		files = append(files, &FileInfo{
			DownloadURL: s3uri,
			// Using the base of the S3 path as a temporary SeriesUID for progress tracking
			SeriesUID: filepath.Base(s3uri),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd manifest: %w", err)
	}

	logger.Infof("Found %d files to download from s5cmd manifest", len(files))
	return files, nil
}
