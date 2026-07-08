package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// EndpointStore remembers paths that returned a successful response,
// keyed by base URL, as a stopgap until OpenAPI spec parsing exists.
type EndpointStore struct {
	file string
	data map[string]map[string][]string // baseURL -> path -> methods
}

func NewEndpointStore(file string) *EndpointStore {
	s := &EndpointStore{
		file: file,
		data: make(map[string]map[string][]string),
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		return s
	}
	json.Unmarshal(raw, &s.data)
	if s.data == nil {
		s.data = make(map[string]map[string][]string)
	}

	return s
}

func LoadEndpointStore() *EndpointStore {
	home, err := os.UserHomeDir()
	if err != nil {
		return NewEndpointStore("")
	}
	return NewEndpointStore(filepath.Join(home, ".rapid", "endpoints.json"))
}

var (
	digitsSegRe = regexp.MustCompile(`^[0-9]+$`)
	uuidSegRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexSegRe    = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	varSegRe    = regexp.MustCompile(`^\$\{[^}]+\}$`)
)

func isIDSegment(seg string) bool {
	return digitsSegRe.MatchString(seg) ||
		uuidSegRe.MatchString(seg) ||
		hexSegRe.MatchString(seg) ||
		varSegRe.MatchString(seg)
}

// generalizePath collapses ID-like segments into {id} so that
// /users/42/posts and /users/${userId}/posts learn the same template
func generalizePath(path string) string {
	if len(path) < 2 {
		return path
	}

	segs := strings.Split(path[1:], "/")
	for i, seg := range segs {
		if isIDSegment(seg) {
			segs[i] = "{id}"
		}
	}

	return "/" + strings.Join(segs, "/")
}

// Add records a successful method+path combination.
// Returns true if the store changed.
func (s *EndpointStore) Add(baseURL, method, path string) bool {
	if path == "" || method == "" {
		return false
	}
	path = generalizePath(path)

	base := strings.TrimSuffix(baseURL, "/")
	if s.data[base] == nil {
		s.data[base] = make(map[string][]string)
	}

	methods := s.data[base][path]
	for _, m := range methods {
		if m == method {
			return false
		}
	}

	methods = append(methods, method)
	sort.Strings(methods)
	s.data[base][path] = methods
	return true
}

func (s *EndpointStore) ForBase(baseURL string) map[string][]string {
	return s.data[strings.TrimSuffix(baseURL, "/")]
}

func (s *EndpointStore) Clear(baseURL string) {
	delete(s.data, strings.TrimSuffix(baseURL, "/"))
}

func (s *EndpointStore) Save() {
	if s.file == "" {
		return
	}

	data, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		return
	}

	// Best effort - learned endpoints are not worth interrupting the REPL over
	os.MkdirAll(filepath.Dir(s.file), 0755)
	os.WriteFile(s.file, data, 0644)
}

// extractEndpointPath pulls the path out of a request as typed, so
// `g(users/${id}?limit=10 {a:b})` yields the template "/users/${id}".
// A path that is an alias variable (addUser = users/new) records the
// alias's value, keeping any ${var} refs raw so they template as {id}.
func extractEndpointPath(input string, variables map[string]interface{}) string {
	open := strings.Index(input, "(")
	close := strings.LastIndex(input, ")")
	if open == -1 || close <= open {
		return ""
	}

	fields := strings.Fields(input[open+1 : close])
	if len(fields) == 0 {
		return ""
	}

	path := fields[0]
	if i := strings.Index(path, "?"); i != -1 {
		path = path[:i]
	}
	if v, ok := variables[path]; ok {
		if s, isStr := v.(string); isStr {
			path = s
			if i := strings.Index(path, "?"); i != -1 {
				path = path[:i]
			}
		}
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if path == "" || strings.HasPrefix(path, "//") {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}
