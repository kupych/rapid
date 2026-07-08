package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestSession(baseURL string) *Session {
	return NewSession(baseURL, make(map[string]interface{}), make(map[string]string), NewEndpointStore(filepath.Join(os.TempDir(), "rapid-test-endpoints")), false)
}

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".rapidvars")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHostMatches(t *testing.T) {
	tests := []struct {
		section, baseURL string
		want             bool
	}{
		{"api.example.com", "https://api.example.com", true},
		{"api.example.com", "http://api.example.com/v2", true},
		{"https://api.example.com/path", "http://api.example.com", true},
		{"API.example.com", "https://api.example.com", true},
		{"localhost:4000", "http://localhost:4000", true},
		{"localhost:4000", "http://localhost:5000", false},
		{"api.example.com", "https://staging.example.com", false},
	}
	for _, tt := range tests {
		if got := hostMatches(tt.section, tt.baseURL); got != tt.want {
			t.Errorf("hostMatches(%q, %q) = %v, want %v", tt.section, tt.baseURL, got, tt.want)
		}
	}
}

func TestRunStartupScriptLegacyJSON(t *testing.T) {
	s := newTestSession("https://api.example.com")
	path := writeScript(t, `{"token": "abc", "$$header:X-Test": "yes"}`)

	runStartupScript(s, path)

	if s.variables["token"] != "abc" {
		t.Errorf("token = %v, want abc", s.variables["token"])
	}
	if s.headers["x-test"] != "yes" {
		t.Errorf("x-test header = %v, want yes", s.headers["x-test"])
	}
}

func TestRunStartupScriptSections(t *testing.T) {
	s := newTestSession("https://api.example.com")
	path := writeScript(t, `
# global lines run for every session
env = global

@api.example.com
token = matched
?h x-scoped: on

@other.example.com
token = wrong
env = wrong
`)

	runStartupScript(s, path)

	if s.variables["env"] != "global" {
		t.Errorf("env = %v, want global", s.variables["env"])
	}
	if s.variables["token"] != "matched" {
		t.Errorf("token = %v, want matched", s.variables["token"])
	}
	if s.headers["x-scoped"] != "on" {
		t.Errorf("x-scoped header = %v, want on", s.headers["x-scoped"])
	}
}

func TestRunStartupScriptMalformedJSON(t *testing.T) {
	s := newTestSession("https://api.example.com")
	path := writeScript(t, `{"token": oops}`)

	runStartupScript(s, path)

	if len(s.variables) != 0 || len(s.headers) != 0 {
		t.Errorf("malformed JSON should load nothing, got vars=%v headers=%v", s.variables, s.headers)
	}
}

func TestRunStartupScriptAuthFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/login" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token": "secret-token-123"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	s := newTestSession(server.URL)
	path := writeScript(t, `
@`+server.URL+`
token = post(auth/login {user:stefan, pass:hunter2}).access_token
?h authorization: Bearer ${token}
`)

	runStartupScript(s, path)

	if s.variables["token"] != "secret-token-123" {
		t.Errorf("token = %v, want secret-token-123", s.variables["token"])
	}
	if s.headers["authorization"] != "Bearer secret-token-123" {
		t.Errorf("authorization header = %v, want Bearer secret-token-123", s.headers["authorization"])
	}
}

func TestRerunStartupScript(t *testing.T) {
	s := newTestSession("https://api.example.com")
	path := writeScript(t, "token = first\n")

	runStartupScript(s, path)
	if s.variables["token"] != "first" {
		t.Fatalf("token = %v, want first", s.variables["token"])
	}

	// ?r re-reads the file, picking up changes
	if err := os.WriteFile(path, []byte("token = second\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !s.Execute("?r") {
		t.Fatal("Execute(?r) should return true")
	}
	if s.variables["token"] != "second" {
		t.Errorf("token after ?r = %v, want second", s.variables["token"])
	}
}

func TestRerunInsideScriptDoesNotRecurse(t *testing.T) {
	s := newTestSession("https://api.example.com")
	path := writeScript(t, "?r\ntoken = ok\n")

	done := make(chan struct{})
	go func() {
		runStartupScript(s, path)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("?r inside the script recursed")
	}
	if s.variables["token"] != "ok" {
		t.Errorf("token = %v, want ok", s.variables["token"])
	}
}

func TestResolvePath(t *testing.T) {
	variables := map[string]interface{}{
		"addUser": "users/new",
		"byId":    "users/${id}",
		"id":      "42",
		"user":    map[string]interface{}{"name": "amy"},
	}

	tests := []struct {
		input       string
		want        string
		wantAliased bool
	}{
		{"addUser", "users/new", true},
		{"addUser?limit=10", "users/new?limit=10", true},
		{"byId", "users/42", true},
		{"users/${id}", "users/42", false},
		{"users/new", "users/new", false},
		{"user", "user", false},         // non-string variables never substitute
		{"/addUser", "/addUser", false}, // leading slash forces a literal path
	}
	for _, tt := range tests {
		got, aliased := resolvePath(tt.input, variables)
		if got != tt.want || aliased != tt.wantAliased {
			t.Errorf("resolvePath(%q) = %q, %v; want %q, %v", tt.input, got, aliased, tt.want, tt.wantAliased)
		}
	}
}

func TestPathAliasInRequest(t *testing.T) {
	variables := map[string]interface{}{"addUser": "users/new"}

	req, err := NewRequest("post(addUser {name:amy})", "https://api.example.com", variables, nil, false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if req.Url != "https://api.example.com/users/new" {
		t.Errorf("Url = %q, want https://api.example.com/users/new", req.Url)
	}

	// Endpoint learning records the alias's value, templating ${var} refs
	variables["byId"] = "users/${id}"
	if got := extractEndpointPath("g(byId)", variables); got != "/users/${id}" {
		t.Errorf("extractEndpointPath alias = %q, want /users/${id}", got)
	}
}

func TestExecuteQuitCommands(t *testing.T) {
	s := newTestSession("https://api.example.com")
	for _, cmd := range []string{"exit", "quit", "q", "x"} {
		if s.Execute(cmd) {
			t.Errorf("Execute(%q) should return false", cmd)
		}
	}
	if !s.Execute("token = abc") {
		t.Error("Execute(assignment) should return true")
	}
	if s.variables["token"] != "abc" {
		t.Errorf("token = %v, want abc", s.variables["token"])
	}
}
