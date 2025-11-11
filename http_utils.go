package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// BaseUrl is the base URL for the NBIA API
	BaseUrl = "https://nbia.cancerimagingarchive.net/nbia-api/services/v4"
	// ImageUrl is the URL for downloading images
	ImageUrl = BaseUrl + "/getImage"
	// MetaUrl is the URL for fetching metadata
	MetaUrl = BaseUrl + "/getSeries"
	// SeriesMetadataUrl is the URL for fetching series metadata
	SeriesMetadataUrl = BaseUrl + "/getSeriesMetadata"
)

// makeURL constructs a URL with query parameters
func makeURL(base string, params map[string]interface{}) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// doRequest performs an HTTP request and returns the response
func doRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// FetchSeriesMetadataCSV fetches series metadata as a CSV file
func FetchSeriesMetadataCSV(seriesUIDs []string, client *http.Client) ([]byte, error) {
	// Prepare the request body
	data := url.Values{}
	data.Set("list", strings.Join(seriesUIDs, ","))
	data.Set("format", "csv")

	req, err := http.NewRequest("POST", SeriesMetadataUrl, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %w", err)
	}

	// Set headers
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "text/csv")

	// Perform the request
	resp, err := doRequest(client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform metadata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata request failed with status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata response: %w", err)
	}

	return body, nil
}
