package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GetCachePath returns the cache file path for a given spec hash
func GetCachePath(hash string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".rapid", "cache", "specs", hash+".json")
}

// SaveToCache saves a parsed spec to the disk cache
func SaveToCache(spec *Spec) error {
	if spec == nil || spec.Hash == "" {
		return fmt.Errorf("spec or hash is empty")
	}

	if err := EnsureCacheDir(); err != nil {
		return err
	}

	cachePath := GetCachePath(spec.Hash)

	// Marshal spec to JSON
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal spec for cache: %w", err)
	}

	// Write to file
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// LoadFromCache loads a parsed spec from the disk cache
// Returns nil if cache doesn't exist
func LoadFromCache(hash string) (*Spec, error) {
	if hash == "" {
		return nil, nil
	}

	cachePath := GetCachePath(hash)

	// Check if file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, nil // Cache miss is not an error
	}

	// Read file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	// Unmarshal JSON
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached spec: %w", err)
	}

	return &spec, nil
}

// InvalidateCache deletes a cache entry
func InvalidateCache(hash string) error {
	if hash == "" {
		return nil
	}

	cachePath := GetCachePath(hash)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	return nil
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func EnsureCacheDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".rapid", "cache", "specs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	return nil
}
