package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFromGen3(t *testing.T) {
	// Set up logger
	setLogger(true, "")

	// Create a mock Gen3 server
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/drs/dg.4DFC/510a380c-3a25-5214-9bfe-0a487f497e04" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"access_url": "%s/download/510a380c-3a25-5214-9bfe-0a487f497e04"}`, server.URL)
		} else if r.URL.Path == "/download/510a380c-3a25-5214-9bfe-0a487f497e04" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "file content")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create a temporary output directory
	outputDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create a mock auth file
	authFile, err := os.CreateTemp("", "auth")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(authFile.Name())
	authFile.WriteString("test-api-key")
	authFile.Close()

	// Create a FileInfo object with a DRS URI
	fileInfo := &FileInfo{
		DRSURI:    fmt.Sprintf("drs://%s/dg.4DFC/510a380c-3a25-5214-9bfe-0a487f497e04", server.Listener.Addr().String()),
		SeriesUID: "510a380c-3a25-5214-9bfe-0a487f497e04",
	}

	// Create an Options object
	options := &Options{
		Auth: authFile.Name(),
	}

	// Create an HTTP client
	httpClient := server.Client()

	// Call the downloadFromGen3 function
	err = fileInfo.downloadFromGen3(outputDir, httpClient, options)
	if err != nil {
		t.Fatalf("downloadFromGen3 failed: %v", err)
	}

	// Verify that the file was downloaded
	downloadedFilePath := filepath.Join(outputDir, fileInfo.SeriesUID)
	content, err := os.ReadFile(downloadedFilePath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != "file content\n" {
		t.Errorf("unexpected file content: got %q, want %q", string(content), "file content\n")
	}
}
