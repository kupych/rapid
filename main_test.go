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
			name:     "simple object with inferred number",
			input:    "{name:john,age:30}",
			expected: `{"age":30,"name":"john"}`,
		},
		{
			name:     "single field",
			input:    "{username:admin}",
			expected: `{"username":"admin"}`,
		},
		{
			name:     "with spaces",
			input:    "{ name : john , age : 30 }",
			expected: `{"age":30,"name":"john"}`,
		},
		{
			name:     "bools null and float",
			input:    "{active:true,deleted:false,note:null,ratio:1.5}",
			expected: `{"active":true,"deleted":false,"note":null,"ratio":1.5}`,
		},
		{
			name:     "quoted forces string",
			input:    `{zip:"007",flag:"true"}`,
			expected: `{"flag":"true","zip":"007"}`,
		},
		{
			name:     "single quotes force string",
			input:    "{a:'2',b:'true'}",
			expected: `{"a":"2","b":"true"}`,
		},
		{
			name:     "apostrophe in bare word stays literal",
			input:    "{name:O'Brien}",
			expected: `{"name":"O'Brien"}`,
		},
		{
			name:     "long id stays string",
			input:    "{id:123456789012345678901234}",
			expected: `{"id":"123456789012345678901234"}`,
		},
		{
			name:     "scalar array with inference",
			input:    "{tags[a,b,c],nums[1,2,3]}",
			expected: `{"nums":[1,2,3],"tags":["a","b","c"]}`,
		},
		{
			name:     "array with colon",
			input:    "{tags:[a,b]}",
			expected: `{"tags":["a","b"]}`,
		},
		{
			name:     "array of objects",
			input:    "{users[{name:amy},{name:bob}]}",
			expected: `{"users":[{"name":"amy"},{"name":"bob"}]}`,
		},
		{
			name:     "quoted string with comma",
			input:    `{msg:"hello, world"}`,
			expected: `{"msg":"hello, world"}`,
		},
		{
			name:     "nested without colons",
			input:    "{a{b{c:d}}}",
			expected: `{"a":{"b":{"c":"d"}}}`,
		},
		{
			name:     "nested with colons",
			input:    "{a:{b:{c:d}}}",
			expected: `{"a":{"b":{"c":"d"}}}`,
		},
		{
			name:     "mixed scalar and nested",
			input:    "{name:amy,address{city:nyc,zip:10001}}",
			expected: `{"address":{"city":"nyc","zip":10001},"name":"amy"}`,
		},
		{
			name:     "unclosed nested input",
			input:    "{a{b{c:d",
			expected: `{"a":{"b":{"c":"d"}}}`,
		},
		{
			name:     "quoted keys pass through",
			input:    `{"name": "amy", "age": 30}`,
			expected: `{"age":30,"name":"amy"}`,
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

func TestParseBodyVariables(t *testing.T) {
	variables := map[string]interface{}{
		"body": "{a{b{c:d}}}",
		"user": map[string]interface{}{"name": "amy", "age": float64(30)},
	}

	tests := []struct {
		name        string
		input       string
		wantBody    string
		wantContent string
	}{
		{
			name:        "bare variable holding CJSON string",
			input:       "body",
			wantBody:    `{"a":{"b":{"c":"d"}}}`,
			wantContent: "application/json",
		},
		{
			name:        "interpolated CJSON string",
			input:       "${body}",
			wantBody:    `{"a":{"b":{"c":"d"}}}`,
			wantContent: "application/json",
		},
		{
			name:        "bare variable holding object",
			input:       "user",
			wantBody:    `{"age":30,"name":"amy"}`,
			wantContent: "application/json",
		},
		{
			name:        "object interpolated inside CJSON",
			input:       "{profile: ${user}, active: true}",
			wantBody:    `{"active":true,"profile":{"age":30,"name":"amy"}}`,
			wantContent: "application/json",
		},
		{
			name:        "bare variable with path",
			input:       "user.name",
			wantBody:    "amy",
			wantContent: "text/plain",
		},
		{
			name:        "unknown bare word stays empty",
			input:       "nosuchvar",
			wantBody:    "",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := parseBody(tt.input, variables)
			if body != tt.wantBody || contentType != tt.wantContent {
				t.Errorf("parseBody(%q) = %q, %q; want %q, %q", tt.input, body, contentType, tt.wantBody, tt.wantContent)
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

func TestLookupVariable(t *testing.T) {
	variables := map[string]interface{}{
		"token": "abc123",
		"count": float64(7),
		"user": map[string]interface{}{
			"name":    "amy",
			"address": map[string]interface{}{"city": "berlin"},
		},
		"$$auth": "secret",
	}

	// Scalars come out bare
	if out, ok := lookupVariable("token", variables); !ok || out != "abc123" {
		t.Errorf("token: got %q ok=%v", out, ok)
	}
	if out, ok := lookupVariable("count", variables); !ok || out != "7" {
		t.Errorf("count: got %q ok=%v", out, ok)
	}

	// Objects pretty-print
	out, ok := lookupVariable("user", variables)
	if !ok || !strings.Contains(out, "\n") || !strings.Contains(out, "\"name\": \"amy\"") {
		t.Errorf("user: expected pretty JSON, got %q", out)
	}

	// Path access drills in
	if out, ok := lookupVariable("user.address.city", variables); !ok || out != "berlin" {
		t.Errorf("user.address.city: got %q ok=%v", out, ok)
	}

	// Special variables are reachable too
	if out, ok := lookupVariable("$$auth", variables); !ok || out != "secret" {
		t.Errorf("$$auth: got %q ok=%v", out, ok)
	}

	// Misses report not-ok
	if _, ok := lookupVariable("nope", variables); ok {
		t.Error("expected unknown variable to be not-ok")
	}
	if _, ok := lookupVariable("user.nope", variables); ok {
		t.Error("expected bad path to be not-ok")
	}
	if _, ok := lookupVariable("token.x", variables); ok {
		t.Error("expected path into scalar to be not-ok")
	}
}

func TestPreviewValue(t *testing.T) {
	if got := previewValue("short"); got != "short" {
		t.Errorf("short value: got %q", got)
	}

	long := strings.Repeat("x", 100)
	got := previewValue(long)
	if len([]rune(got)) != 60 || !strings.HasSuffix(got, "...") {
		t.Errorf("long value: got %d chars %q", len([]rune(got)), got)
	}

	obj := map[string]interface{}{"a": float64(1)}
	if got := previewValue(obj); got != `{"a":1}` {
		t.Errorf("object: got %q", got)
	}
}
