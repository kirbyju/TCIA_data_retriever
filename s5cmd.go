package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

		// Check if the URI contains a wildcard
		if strings.Contains(s3uri, "*") {
			logger.Infof("Expanding wildcard URI: %s", s3uri)
			// Use s5cmd to list files
			cmd := exec.Command("s5cmd", "--no-sign-request", "ls", s3uri)
			var out bytes.Buffer
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				logger.Warnf("Failed to expand wildcard URI %s: %v", s3uri, err)
				continue
			}

			// Process the output of s5cmd ls
			lsScanner := bufio.NewScanner(&out)
			for lsScanner.Scan() {
				lsLine := lsScanner.Text()
				// Find the s3:// prefix to correctly handle filenames with spaces
				s3Index := strings.Index(lsLine, "s3://")
				if s3Index == -1 {
					continue
				}
				expandedURI := lsLine[s3Index:]
				files = append(files, &FileInfo{
					DownloadURL: expandedURI,
					// Using the base of the S3 path as a temporary SeriesUID for progress tracking
					SeriesUID: filepath.Base(expandedURI),
				})
			}
			if err := lsScanner.Err(); err != nil {
				logger.Warnf("Error reading s5cmd ls output for %s: %v", s3uri, err)
			}
		} else {
			// No wildcard, add directly
			files = append(files, &FileInfo{
				DownloadURL: s3uri,
				// Using the base of the S3 path as a temporary SeriesUID for progress tracking
				SeriesUID: filepath.Base(s3uri),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd manifest: %w", err)
	}

	logger.Infof("Found %d files to download from s5cmd manifest", len(files))
	return files, nil
}
