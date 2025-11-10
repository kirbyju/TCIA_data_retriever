package main

import (
	"net/http"
	"strings"
)

// doRequest performs an HTTP request
func doRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

// makeURL creates a URL with query parameters
func makeURL(baseURL string, params map[string]interface{}) (string, error) {
	if len(params) == 0 {
		return baseURL, nil
	}

	var sb strings.Builder
	sb.WriteString(baseURL)
	sb.WriteRune('?')

	first := true
	for key, value := range params {
		if !first {
			sb.WriteRune('&')
		}
		first = false
		sb.WriteString(key)
		sb.WriteRune('=')
		sb.WriteString(value.(string))
	}
	return sb.String(), nil
}
