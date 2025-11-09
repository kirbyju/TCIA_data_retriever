package main

import (
	"bufio"
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
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		parts := strings.Fields(line)
		var s3uri string
		if len(parts) >= 2 && parts[0] == "cp" {
			s3uri = parts[1]
		} else if len(parts) >= 1 && strings.HasPrefix(parts[0], "s3://") {
			s3uri = parts[0]
		} else {
			logger.Warnf("Skipping unrecognized line in s5cmd manifest: %s", line)
			continue
		}

		// If the URI contains a wildcard, expand it
		if strings.Contains(s3uri, "*") {
			logger.Infof("Found wildcard in s5cmd manifest: %s. Expanding...", s3uri)
			cmd := exec.Command("s5cmd",
				"--no-sign-request",
				"--endpoint-url", "https://s3.amazonaws.com",
				"ls",
				s3uri,
			)
			stdout, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("s5cmd ls command failed for %s: %s\nOutput: %s", s3uri, err, string(stdout))
			}

			// Process the output of the ls command
			lsScanner := bufio.NewScanner(strings.NewReader(string(stdout)))
			for lsScanner.Scan() {
				lsLine := strings.TrimSpace(lsScanner.Text())
				// Find the start of the s3:// URI
				s3Index := strings.Index(lsLine, "s3://")
				if s3Index != -1 {
					expandedURI := lsLine[s3Index:]
					files = append(files, &FileInfo{
						DownloadURL: expandedURI,
						SeriesUID:   filepath.Base(expandedURI), // Temporary UID for progress
					})
				}
			}
			if err := lsScanner.Err(); err != nil {
				return nil, fmt.Errorf("error reading s5cmd ls output for %s: %w", s3uri, err)
			}
		} else {
			// No wildcard, add the file directly
			files = append(files, &FileInfo{
				DownloadURL: s3uri,
				SeriesUID:   filepath.Base(s3uri), // Temporary UID for progress
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd manifest: %w", err)
	}

	logger.Infof("Found %d total files to download from s5cmd manifest after expansion", len(files))
	return files, nil
}
