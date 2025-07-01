package main

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
)

func newClient(proxy string, maxConnsPerHost int) *http.Client {
	logger.Debugf("initializing http request client with max %d connections per host", maxConnsPerHost)
	if proxy != "" {
		logger.Debugf("using proxy %s", proxy)
	}

	// Configure transport for parallel downloads (server-friendly settings)
	transport := &http.Transport{
		MaxIdleConns:        maxConnsPerHost * 2,  // Server-friendly: reduced multiplier
		MaxIdleConnsPerHost: maxConnsPerHost,
		MaxConnsPerHost:     maxConnsPerHost,
		IdleConnTimeout:     30 * time.Second,     // Server-friendly: reduced from 90s
		TLSHandshakeTimeout: 20 * time.Second,     // Server-friendly: increased timeout
		DisableKeepAlives:   false,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	// Add proxy if configured
	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			logger.Fatal("failed to parse proxy string: %v", err)
		}
		transport.Proxy = http.ProxyURL(p)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute, // Global timeout for requests
	}
	
	return client
}
