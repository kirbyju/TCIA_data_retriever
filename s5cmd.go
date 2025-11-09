package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// downloadS5cmdManifest downloads files from an S3 manifest using the s5cmd command-line tool.
func (info *FileInfo) downloadS5cmdManifest(output string, options *Options) error {
	logger.Debugf("Downloading from S3 using s5cmd manifest: %s", info.S5cmdManifestPath)

	// Construct the s5cmd command
	// s5cmd --no-sign-request --endpoint-url https://s3.amazonaws.com run <manifest-path>
	cmd := exec.Command("s5cmd",
		"--no-sign-request",
		"--endpoint-url", "https://s3.amazonaws.com",
		"run",
		info.S5cmdManifestPath,
	)
	cmd.Dir = output // Run the command in the output directory

	// Execute the command
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("s5cmd command failed: %s\nOutput: %s", err, string(stdout))
	}

	logger.Infof("s5cmd output:\n%s", string(stdout))

	return nil
}

func decodeS5cmd(filePath string) ([]*FileInfo, error) {
	// For s5cmd, we don't need to parse the file here.
	// We just need to pass the manifest path to the download function.
	// We'll create a single FileInfo to represent the entire manifest.
	return []*FileInfo{
		{
			S5cmdManifestPath: filePath,
			SeriesUID:         fmt.Sprintf("s5cmd-manifest-%s", filepath.Base(filePath)),
		},
	}, nil
}
