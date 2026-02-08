package spec

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LoadSpec loads an OpenAPI spec from a file or URL, using cache when possible
func LoadSpec(source string) (*Spec, error) {
	var rawData []byte
	var err error

	// Determine if source is a URL or file path
	if isURL(source) {
		rawData, err = loadFromURL(source)
		if err != nil {
			return nil, fmt.Errorf("failed to load spec from URL: %w", err)
		}
	} else {
		rawData, err = loadFromFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed to load spec from file: %w", err)
		}
	}

	// Compute hash of the raw data
	hash := ComputeHash(rawData)

	// Try to load from cache first
	if cachedSpec, err := LoadFromCache(hash); err == nil && cachedSpec != nil {
		// Update metadata
		cachedSpec.Source = source
		cachedSpec.LoadedAt = time.Now()
		return cachedSpec, nil
	}

	// Cache miss - parse the spec
	spec, err := ParseSpec(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	// Update metadata
	spec.Source = source
	spec.Hash = hash

	// Save to cache for future use
	if err := SaveToCache(spec); err != nil {
		// Cache save failure is not critical, just log it
		fmt.Fprintf(os.Stderr, "Warning: failed to cache spec: %v\n", err)
	}

	return spec, nil
}

// loadFromFile reads a file from the local filesystem
func loadFromFile(path string) ([]byte, error) {
	// Expand ~ to home directory if needed
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = homeDir + path[1:]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return data, nil
}

// loadFromURL fetches a spec from a URL with timeout
func loadFromURL(url string) ([]byte, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// isURL checks if a string is a URL (http:// or https://)
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
