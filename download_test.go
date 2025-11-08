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
		downloadPath := "/download/510a380c-3a25-5214-bfe-0a487f497e04"
		presignedURLPath := "/user/data/download/dg.4DFC%2F510a380c-3a25-5214-bfe-0a487f497e04"

		switch {
		case r.Method == "GET" && r.URL.RawPath == presignedURLPath:
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				t.Errorf("Incorrect Authorization header: got %q", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"url": "%s%s"}`, server.URL, downloadPath)
		case r.Method == "GET" && r.URL.Path == downloadPath:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "file content")
		default:
			t.Logf("Unexpected request: %s %s (RawPath: %s)", r.Method, r.URL.Path, r.URL.RawPath)
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
	if _, err := authFile.WriteString("{\"access_token\":\"test_token\"}"); err != nil {
		t.Fatalf("Failed to write to auth file: %v", err)
	}
	authFile.Close()

	// Create a FileInfo object with a DRS URI
	fileInfo := &FileInfo{
		DRSURI:    fmt.Sprintf("drs://%s/dg.4DFC/510a380c-3a25-5214-bfe-0a487f497e04", server.Listener.Addr().String()),
		SeriesUID: "510a380c-3a25-5214-bfe-0a487f497e04",
		FileName:  "custom_filename.txt",
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
	downloadedFilePath := filepath.Join(outputDir, fileInfo.FileName)
	content, err := os.ReadFile(downloadedFilePath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != "file content\n" {
		t.Errorf("unexpected file content: got %q, want %q", string(content), "file content\n")
	}
}

func TestDownloadFromGen3_NoFileName(t *testing.T) {
	// Set up logger
	setLogger(true, "")

	// Create a mock Gen3 server
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadPath := "/download/510a380c-3a25-5214-bfe-0a487f497e04"
		presignedURLPath := "/user/data/download/dg.4DFC%2F510a380c-3a25-5214-bfe-0a487f497e04"

		switch {
		case r.Method == "GET" && r.URL.RawPath == presignedURLPath:
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				t.Errorf("Incorrect Authorization header: got %q", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"url": "%s%s"}`, server.URL, downloadPath)
		case r.Method == "GET" && r.URL.Path == downloadPath:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "file content")
		default:
			t.Logf("Unexpected request: %s %s (RawPath: %s)", r.Method, r.URL.Path, r.URL.RawPath)
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
	if _, err := authFile.WriteString("{\"access_token\":\"test_token\"}"); err != nil {
		t.Fatalf("Failed to write to auth file: %v", err)
	}
	authFile.Close()

	// Create a FileInfo object with a DRS URI but no FileName
	fileInfo := &FileInfo{
		DRSURI:    fmt.Sprintf("drs://%s/dg.4DFC/510a380c-3a25-5214-bfe-0a487f497e04", server.Listener.Addr().String()),
		SeriesUID: "510a380c-3a25-5214-bfe-0a487f497e04",
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

	// Verify that the file was downloaded with the SeriesUID as the name
	downloadedFilePath := filepath.Join(outputDir, fileInfo.SeriesUID)
	content, err := os.ReadFile(downloadedFilePath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != "file content\n" {
		t.Errorf("unexpected file content: got %q, want %q", string(content), "file content\n")
	}
}
