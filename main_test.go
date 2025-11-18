package main

import (
	"strings"
	"testing"
)

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			baseURL:  "https://api.com",
			path:     "/users",
			expected: "https://api.com/users",
		},
		{
			name:     "base URL with trailing slash",
			baseURL:  "https://api.com/",
			path:     "/users",
			expected: "https://api.com/users",
		},
		{
			name:     "path without leading slash",
			baseURL:  "https://api.com",
			path:     "users",
			expected: "https://api.com/users",
		},
		{
			name:     "override with //",
			baseURL:  "https://api.com",
			path:     "//other.com/path",
			expected: "https://other.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildURL(tt.baseURL, tt.path)
			if result != tt.expected {
				t.Errorf("buildURL(%q, %q) = %q, want %q", tt.baseURL, tt.path, result, tt.expected)
			}
		})
	}
}

func TestInterpolateVars(t *testing.T) {
	vars := map[string]interface{}{
		"id":   123,
		"name": "john",
		"flag": true,
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single variable",
			input:    "/users/${id}",
			expected: "/users/123",
		},
		{
			name:     "multiple variables",
			input:    "/users/${id}/posts/${name}",
			expected: "/users/123/posts/john",
		},
		{
			name:     "no variables",
			input:    "/users/all",
			expected: "/users/all",
		},
		{
			name:     "boolean variable",
			input:    "/flag/${flag}",
			expected: "/flag/true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolateVars(tt.input, vars)
			if result != tt.expected {
				t.Errorf("interpolateVars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple object",
			input:    "{name:john,age:30}",
			expected: `{"age":"30","name":"john"}`,
		},
		{
			name:     "single field",
			input:    "{username:admin}",
			expected: `{"username":"admin"}`,
		},
		{
			name:     "with spaces",
			input:    "{ name : john , age : 30 }",
			expected: `{"age":"30","name":"john"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCJSON(tt.input)
			if result != tt.expected {
				t.Errorf("parseCJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseVarMappings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "simple mapping",
			input: "{id, name}",
			expected: map[string]string{
				"id":   "id",
				"name": "name",
			},
		},
		{
			name:  "custom mapping",
			input: "{userId: data.id, userName: data.name}",
			expected: map[string]string{
				"userId":   "data.id",
				"userName": "data.name",
			},
		},
		{
			name:  "mixed mapping",
			input: "{id, userName: user.name}",
			expected: map[string]string{
				"id":       "id",
				"userName": "user.name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVarMappings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseVarMappings(%q) returned %d items, want %d", tt.input, len(result), len(tt.expected))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("parseVarMappings(%q)[%q] = %q, want %q", tt.input, k, result[k], v)
				}
			}
		})
	}
}

func TestIsRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "get long form", input: "get(users)", expected: true},
		{name: "get short form", input: "g(users)", expected: true},
		{name: "post long form", input: "post(users {id:1})", expected: true},
		{name: "post short form", input: "p(users {id:1})", expected: true},
		{name: "put long form", input: "put(users/1 {name:john})", expected: true},
		{name: "put short form", input: "pu(users/1 {name:john})", expected: true},
		{name: "patch long form", input: "patch(users/1 {name:john})", expected: true},
		{name: "patch short form", input: "pa(users/1 {name:john})", expected: true},
		{name: "delete long form", input: "delete(users/1)", expected: true},
		{name: "delete short form", input: "d(users/1)", expected: true},
		{name: "not a request", input: "?v", expected: false},
		{name: "variable assignment", input: "id = 123", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRequest(tt.input)
			if result != tt.expected {
				t.Errorf("isRequest(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInlineHeaders(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedHeaders map[string]string
		expectedClean   string
	}{
		{
			name:  "single header",
			input: "get(users) <X-API-Key: abc123>",
			expectedHeaders: map[string]string{
				"x-api-key": "abc123",
			},
			expectedClean: "get(users)",
		},
		{
			name:  "multiple headers",
			input: "get(users) <X-API-Key: abc123><Content-Type: application/json>",
			expectedHeaders: map[string]string{
				"x-api-key":    "abc123",
				"content-type": "application/json",
			},
			expectedClean: "get(users)",
		},
		{
			name:            "no headers",
			input:           "get(users)",
			expectedHeaders: map[string]string{},
			expectedClean:   "get(users)",
		},
		{
			name:  "header with spaces",
			input: "get(users) < X-API-Key : abc123 >",
			expectedHeaders: map[string]string{
				"x-api-key": "abc123",
			},
			expectedClean: "get(users)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, clean := parseInlineHeaders(tt.input)

			if len(headers) != len(tt.expectedHeaders) {
				t.Errorf("parseInlineHeaders(%q) returned %d headers, want %d", tt.input, len(headers), len(tt.expectedHeaders))
			}

			for k, v := range tt.expectedHeaders {
				if headers[k] != v {
					t.Errorf("parseInlineHeaders(%q) header[%q] = %q, want %q", tt.input, k, headers[k], v)
				}
			}

			if clean != tt.expectedClean {
				t.Errorf("parseInlineHeaders(%q) clean = %q, want %q", tt.input, clean, tt.expectedClean)
			}
		})
	}
}

func TestDollarPrefixMatching(t *testing.T) {
	// Test that $ and $$ are handled correctly
	tests := []struct {
		name              string
		input             string
		shouldMatchDollar bool // Should match the "strings.HasPrefix(input, "$") && !strings.HasPrefix(input, "$$")" case
	}{
		{name: "single dollar", input: "$", shouldMatchDollar: true},
		{name: "dollar with number", input: "$1", shouldMatchDollar: true},
		{name: "dollar with path", input: "$.data", shouldMatchDollar: true},
		{name: "dollar with index and path", input: "$1.0.id", shouldMatchDollar: true},
		{name: "double dollar auth", input: "$$auth", shouldMatchDollar: false},
		{name: "double dollar header", input: "$$header:X-Key", shouldMatchDollar: false},
		{name: "double dollar in assignment", input: "$$auth = token123", shouldMatchDollar: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.HasPrefix(tt.input, "$") && !strings.HasPrefix(tt.input, "$$")
			if result != tt.shouldMatchDollar {
				t.Errorf("Pattern match for %q = %v, want %v", tt.input, result, tt.shouldMatchDollar)
			}
		})
	}
}
