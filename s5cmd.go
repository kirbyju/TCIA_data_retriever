package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// downloadFromS5cmd downloads a file from S3 using the s5cmd command-line tool.
func (info *FileInfo) downloadFromS5cmd(output string, options *Options) error {
	logger.Debugf("Downloading from S3 using s5cmd: %s", info.S5cmdURI)

	// Construct the s5cmd command
	// s5cmd --no-sign-request --endpoint-url https://s3.amazonaws.com cp <s3-uri> <output-dir>
	cmd := exec.Command("s5cmd",
		"--no-sign-request",
		"--endpoint-url", "https://s3.amazonaws.com",
		"cp",
		info.S5cmdURI,
		output,
	)

	// Execute the command
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("s5cmd command failed: %s\nOutput: %s", err, string(stdout))
	}

	logger.Infof("s5cmd output:\n%s", string(stdout))

	return nil
}

func decodeS5cmd(filePath string) ([]*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open s5cmd file: %w", err)
	}
	defer file.Close()

	var fileInfos []*FileInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Ignore comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Look for 'cp' commands
		if strings.HasPrefix(line, "cp ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				s3URI := parts[1]
				// a simple validation for s3 uri
				if strings.HasPrefix(s3URI, "s3://") {
					fileInfos = append(fileInfos, &FileInfo{
						S5cmdURI:  s3URI,
						SeriesUID: s3URI, // Use URI as a unique identifier
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading s5cmd file: %w", err)
	}

	return fileInfos, nil
}
