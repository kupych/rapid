package main

import (
	"rapid/providers"
	"testing"
)

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		cursor int
		kind   providers.ContextKind
		prefix string
	}{
		{"start of line", "g", 1, providers.KindCommand, "g"},
		{"empty line", "", 0, providers.KindCommand, ""},
		{"after equals", "id = ", 5, providers.KindCommand, ""},
		{"after equals with text", "id = ge", 7, providers.KindCommand, "ge"},
		{"inside paren", "get(users", 9, providers.KindPath, "users"},
		{"inside paren after slash", "get(/users/", 11, providers.KindPath, ""},
		{"inside paren past path", "post(/login {us", 15, providers.KindArgument, "us"},
		{"metacommand", "?v", 2, providers.KindMeta, "?v"},
		{"metacommand bare", "?", 1, providers.KindMeta, "?"},
		{"variable ref", "get(/users/${us", 15, providers.KindVariableRef, "us"},
		{"variable ref empty", "get(/users/${", 13, providers.KindVariableRef, ""},
		{"closed variable ref", "get(/users/${id}", 16, providers.KindPath, ""},
		{"dollar path root", "$.da", 4, providers.KindDollarPath, "da"},
		{"dollar path empty", "$.", 2, providers.KindDollarPath, ""},
		{"dollar path nested", "$.data.us", 9, providers.KindDollarPath, "us"},
		{"dollar path indexed", "$2.items.", 9, providers.KindDollarPath, ""},
		{"dollar in assignment", "id = $.da", 9, providers.KindDollarPath, "da"},
		{"header name", "get(/api <cont", 14, providers.KindHeaderName, "cont"},
		{"header name empty", "get(/api <", 10, providers.KindHeaderName, ""},
		{"header value", "get(/api <custom: val", 21, providers.KindUnknown, ""},
		{"closed header", "get(/api <custom: val>", 22, providers.KindArgument, ""},
		{"after completed request", "id = get(users) ", 16, providers.KindUnknown, ""},
		{"double dollar", "$$auth", 6, providers.KindCommand, "$$auth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Analyze(tt.input, tt.cursor)
			if ctx.Kind != tt.kind {
				t.Errorf("kind: got %v, want %v", ctx.Kind, tt.kind)
			}
			if ctx.Prefix != tt.prefix {
				t.Errorf("prefix: got %q, want %q", ctx.Prefix, tt.prefix)
			}
		})
	}
}

func TestAnalyzePathDetails(t *testing.T) {
	ctx := Analyze("id = post(/users/12/po", 22)
	if ctx.Kind != providers.KindPath {
		t.Fatalf("expected KindPath, got %v", ctx.Kind)
	}
	if ctx.Arg != "/users/12/po" {
		t.Errorf("arg: got %q, want %q", ctx.Arg, "/users/12/po")
	}
	if ctx.Command != "POST" {
		t.Errorf("command: got %q, want %q", ctx.Command, "POST")
	}
	if ctx.Prefix != "po" {
		t.Errorf("prefix: got %q, want %q", ctx.Prefix, "po")
	}
}

func TestAnalyzeDollarPathDetails(t *testing.T) {
	ctx := Analyze("$2.data.users.0.na", 18)
	if ctx.Kind != providers.KindDollarPath {
		t.Fatalf("expected KindDollarPath, got %v", ctx.Kind)
	}
	if ctx.HistoryIndex != 2 {
		t.Errorf("history index: got %d, want 2", ctx.HistoryIndex)
	}
	if ctx.Path != "data.users.0" {
		t.Errorf("path: got %q, want %q", ctx.Path, "data.users.0")
	}
	if ctx.Prefix != "na" {
		t.Errorf("prefix: got %q, want %q", ctx.Prefix, "na")
	}
}

func TestCommandProvider(t *testing.T) {
	provider := &providers.Command{}

	suggestions := provider.GetSuggestions(providers.Context{Kind: providers.KindCommand, Prefix: "g"})
	if len(suggestions) != 1 || suggestions[0].Text != "get(" {
		t.Errorf("Expected get( suggestion for prefix 'g'")
	}

	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindCommand, Prefix: "p"})
	if len(suggestions) != 3 {
		t.Errorf("Expected post(, patch( and put( for prefix 'p', got %d", len(suggestions))
	}

	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindPath, Prefix: "g"})
	if len(suggestions) > 0 {
		t.Errorf("Expected no suggestions inside parens")
	}
}

func TestMetaCommandProvider(t *testing.T) {
	provider := &providers.MetaCommand{}

	suggestions := provider.GetSuggestions(providers.Context{Kind: providers.KindMeta, Prefix: "?v"})
	if len(suggestions) != 2 {
		t.Errorf("Expected ?v and ?vc for prefix '?v', got %d", len(suggestions))
	}

	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindMeta, Prefix: "?"})
	if len(suggestions) != 10 {
		t.Errorf("Expected all 10 metacommands for prefix '?', got %d", len(suggestions))
	}
}

func TestVariablesProvider(t *testing.T) {
	provider := &providers.Variables{Vars: map[string]interface{}{
		"userId": 42,
		"token":  "abc",
	}}

	// Inside an open ${...} the suggestion closes the brace
	suggestions := provider.GetSuggestions(providers.Context{Kind: providers.KindVariableRef, Prefix: "us"})
	if len(suggestions) != 1 || suggestions[0].Text != "userId}" {
		t.Errorf("Expected userId} for prefix 'us', got %v", suggestions)
	}

	// Inside parens the suggestion is the full interpolation
	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindPath, Prefix: ""})
	if len(suggestions) != 2 {
		t.Errorf("Expected both variables with empty prefix, got %d", len(suggestions))
	}

	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindArgument, Prefix: "${to"})
	if len(suggestions) != 1 || suggestions[0].Text != "${token}" {
		t.Errorf("Expected ${token} for prefix '${to', got %v", suggestions)
	}

	// Bare names complete at the start of a line for value echo
	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindCommand, Prefix: "tok"})
	if len(suggestions) != 1 || suggestions[0].Text != "token" || suggestions[0].Description != "abc" {
		t.Errorf("Expected bare token with value preview, got %v", suggestions)
	}

	// But not on an empty line - commands own that space
	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindCommand, Prefix: ""})
	if len(suggestions) != 0 {
		t.Errorf("Expected no bare variables with empty prefix, got %v", suggestions)
	}
}

func TestHeadersProvider(t *testing.T) {
	provider := &providers.Headers{Session: map[string]string{
		"x-tenant-id": "abc",
	}}

	suggestions := provider.GetSuggestions(providers.Context{Kind: providers.KindHeaderName, Prefix: "x-"})
	foundSession := false
	for _, s := range suggestions {
		if s.Text == "x-tenant-id: " && s.Description == "session header" {
			foundSession = true
		}
	}
	if !foundSession {
		t.Errorf("Expected session header x-tenant-id in suggestions, got %v", suggestions)
	}

	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindHeaderName, Prefix: "cont"})
	if len(suggestions) != 1 || suggestions[0].Text != "content-type: " {
		t.Errorf("Expected content-type for prefix 'cont', got %v", suggestions)
	}

	// Case-insensitive match
	suggestions = provider.GetSuggestions(providers.Context{Kind: providers.KindHeaderName, Prefix: "Cont"})
	if len(suggestions) != 1 {
		t.Errorf("Expected case-insensitive match for 'Cont', got %v", suggestions)
	}
}

func TestResponsePathProvider(t *testing.T) {
	history := []string{
		`{"old": true}`,
		`{"data": {"users": [{"name": "amy", "email": "amy@x.io"}], "total": 1}, "ok": true}`,
	}
	provider := &providers.ResponsePath{History: &history}

	get := func(prefix string, historyIndex int, path string) []providers.Suggestion {
		return provider.GetSuggestions(providers.Context{
			Kind:         providers.KindDollarPath,
			Prefix:       prefix,
			HistoryIndex: historyIndex,
			Path:         path,
		})
	}

	// Root keys of the latest response
	suggestions := get("", 0, "")
	if len(suggestions) != 2 {
		t.Fatalf("Expected 2 root keys, got %v", suggestions)
	}

	// Prefix filter
	suggestions = get("da", 0, "")
	if len(suggestions) != 1 || suggestions[0].Text != "data" {
		t.Errorf("Expected data for prefix 'da', got %v", suggestions)
	}

	// Nested object keys
	suggestions = get("", 0, "data")
	if len(suggestions) != 2 {
		t.Errorf("Expected users and total under data, got %v", suggestions)
	}

	// Array suggests index and count
	suggestions = get("", 0, "data.users")
	if len(suggestions) != 2 || suggestions[0].Text != "0" || suggestions[1].Text != "#" {
		t.Errorf("Expected 0 and # for array, got %v", suggestions)
	}

	// Keys inside an array element
	suggestions = get("em", 0, "data.users.0")
	if len(suggestions) != 1 || suggestions[0].Text != "email" {
		t.Errorf("Expected email, got %v", suggestions)
	}

	// Older response via history index
	suggestions = get("", 1, "")
	if len(suggestions) != 1 || suggestions[0].Text != "old" {
		t.Errorf("Expected old key from previous response, got %v", suggestions)
	}

	// Out-of-range history index
	suggestions = get("", 9, "")
	if len(suggestions) != 0 {
		t.Errorf("Expected no suggestions for missing history entry, got %v", suggestions)
	}

	// Nonexistent path
	suggestions = get("", 0, "nope.nada")
	if len(suggestions) != 0 {
		t.Errorf("Expected no suggestions for bad path, got %v", suggestions)
	}
}

func TestEndpointsProvider(t *testing.T) {
	provider := &providers.Endpoints{Paths: map[string][]string{
		"/users":            {"GET", "POST"},
		"/users/{id}":       {"GET"},
		"/users/{id}/posts": {"GET"},
		"/health":           {"GET"},
	}}

	get := func(arg, prefix, command string) map[string]bool {
		texts := make(map[string]bool)
		for _, s := range provider.GetSuggestions(providers.Context{
			Kind:    providers.KindPath,
			Prefix:  prefix,
			Arg:     arg,
			Command: command,
		}) {
			texts[s.Text] = true
		}
		return texts
	}

	// Open paren: literals complete fully, templates stop at the ID hole
	texts := get("", "", "GET")
	for _, want := range []string{"users", "users/", "health"} {
		if !texts[want] {
			t.Errorf("Expected %q in suggestions for empty arg, got %v", want, texts)
		}
	}

	// Partial path narrows
	texts = get("/us", "us", "GET")
	if !texts["users"] || !texts["users/"] || texts["health"] {
		t.Errorf("Expected users and users/ for /us, got %v", texts)
	}

	// Cursor in the ID hole: nothing to insert
	texts = get("/users/", "", "GET")
	if len(texts) != 0 {
		t.Errorf("Expected no suggestions in ID hole, got %v", texts)
	}

	// A typed ID matches the wildcard and the path continues past it
	texts = get("/users/42/", "", "GET")
	if len(texts) != 1 || !texts["posts"] {
		t.Errorf("Expected posts after /users/42/, got %v", texts)
	}

	texts = get("/users/42/po", "po", "GET")
	if len(texts) != 1 || !texts["posts"] {
		t.Errorf("Expected posts for /users/42/po, got %v", texts)
	}

	// A typed ${var} segment matches the wildcard too
	texts = get("/users/${userId}/", "", "GET")
	if len(texts) != 1 || !texts["posts"] {
		t.Errorf("Expected posts after ${userId} segment, got %v", texts)
	}

	// Missing leading slash still matches
	texts = get("hea", "hea", "GET")
	if len(texts) != 1 || !texts["health"] {
		t.Errorf("Expected health for arg without slash, got %v", texts)
	}

	// Method match scores higher
	for _, s := range provider.GetSuggestions(providers.Context{
		Kind: providers.KindPath, Arg: "/", Command: "POST",
	}) {
		if s.Display == "/users" && s.Score != 85 {
			t.Errorf("Expected POST-capable /users to score 85, got %d", s.Score)
		}
		if s.Display != "/users" && s.Score != 72 {
			t.Errorf("Expected non-POST endpoint to score 72, got %d for %s", s.Score, s.Display)
		}
	}

	// Nothing outside the path token
	if got := provider.GetSuggestions(providers.Context{Kind: providers.KindArgument}); len(got) != 0 {
		t.Errorf("Expected no endpoint suggestions in body position, got %v", got)
	}
}

func TestGeneralizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/42", "/users/{id}"},
		{"/users/42/posts/7", "/users/{id}/posts/{id}"},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "/users/{id}"},
		{"/docs/507f1f77bcf86cd799439011", "/docs/{id}"},
		{"/users/${userId}", "/users/{id}"},
		{"/users/${userId}/posts", "/users/{id}/posts"},
		{"/v2/users", "/v2/users"},
		{"/users/1.json", "/users/1.json"},
		{"/users", "/users"},
		{"/", "/"},
	}

	for _, tt := range tests {
		if got := generalizePath(tt.input); got != tt.expected {
			t.Errorf("generalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEngineComplete(t *testing.T) {
	history := []string{`{"id": 7, "name": "amy"}`}
	vars := map[string]interface{}{"userId": 7}
	headers := map[string]string{}
	engine := NewAutocompleteEngine(vars, headers, &history, nil)

	// End-to-end: response path completion
	suggestions, ctx := engine.Complete("$.n", 3)
	if ctx.Prefix != "n" {
		t.Errorf("Expected prefix 'n', got %q", ctx.Prefix)
	}
	if len(suggestions) != 1 || suggestions[0].Text != "name" {
		t.Errorf("Expected name, got %v", suggestions)
	}

	// Sorted by score descending
	suggestions, _ = engine.Complete("p", 1)
	if len(suggestions) != 3 || suggestions[0].Text != "post(" {
		t.Errorf("Expected post( first, got %v", suggestions)
	}

	// Variables offered inside parens
	suggestions, _ = engine.Complete("get(/users/", 11)
	if len(suggestions) != 1 || suggestions[0].Text != "${userId}" {
		t.Errorf("Expected ${userId}, got %v", suggestions)
	}
}

func TestReadlineCompleterDo(t *testing.T) {
	vars := map[string]interface{}{}
	headers := map[string]string{}
	history := []string{`{"data": [1, 2, 3]}`}
	store := NewEndpointStore("")
	store.Add("http://x.io", "GET", "/items")
	completer := NewReadlineCompleter(&vars, &headers, &history, store, "http://x.io")

	// "ge" should complete to "get(" -> suffix "t("
	line := []rune("ge")
	results, length := completer.Do(line, 2)
	if length != 2 {
		t.Errorf("Expected prefix length 2, got %d", length)
	}
	if len(results) != 1 || string(results[0]) != "t(" {
		t.Errorf("Expected suffix 't(', got %v", results)
	}

	// "$.da" should complete to "data" -> suffix "ta"
	line = []rune("$.da")
	results, length = completer.Do(line, 4)
	if length != 2 {
		t.Errorf("Expected prefix length 2, got %d", length)
	}
	if len(results) != 1 || string(results[0]) != "ta" {
		t.Errorf("Expected suffix 'ta', got %v", results)
	}

	// Live state: adding a variable later is picked up
	vars["token"] = "abc"
	line = []rune("get(/x/${t")
	results, _ = completer.Do(line, 10)
	if len(results) != 1 || string(results[0]) != "oken}" {
		t.Errorf("Expected suffix 'oken}', got %v", results)
	}

	// Learned endpoints complete in the path position
	line = []rune("get(/it")
	results, _ = completer.Do(line, 7)
	if len(results) != 1 || string(results[0]) != "ems" {
		t.Errorf("Expected suffix 'ems' from learned endpoint, got %v", results)
	}
}

func TestGhostPainter(t *testing.T) {
	vars := map[string]interface{}{}
	headers := map[string]string{}
	history := []string{}
	store := NewEndpointStore("")
	store.Add("http://x.io", "GET", "/users/42")
	completer := NewReadlineCompleter(&vars, &headers, &history, store, "http://x.io")
	painter := NewGhostPainter(completer, "> ")

	// Top suggestion's remainder is rendered dim after the line
	line := []rune("ge")
	out := painter.Paint(line, 2)
	if string(painter.Ghost()) != "t(" {
		t.Errorf("Expected ghost 't(', got %q", string(painter.Ghost()))
	}
	expected := "ge" + CGray + "t(" + CReset + "\033[2D"
	if string(out) != expected {
		t.Errorf("Paint output = %q, want %q", string(out), expected)
	}

	// Learned endpoints ghost in the path position, stopping at the ID hole
	out = painter.Paint([]rune("get(/u"), 6)
	if string(painter.Ghost()) != "sers/" {
		t.Errorf("Expected ghost 'sers/', got %q", string(painter.Ghost()))
	}

	// No ghost mid-line
	out = painter.Paint([]rune("get("), 2)
	if painter.Ghost() != nil || string(out) != "get(" {
		t.Errorf("Expected untouched line mid-cursor, got %q ghost %q", string(out), string(painter.Ghost()))
	}

	// No ghost on empty line
	out = painter.Paint([]rune(""), 0)
	if painter.Ghost() != nil || string(out) != "" {
		t.Errorf("Expected no ghost on empty line")
	}

	// No ghost without matches, line passes through untouched
	out = painter.Paint([]rune("zzz"), 3)
	if painter.Ghost() != nil || string(out) != "zzz" {
		t.Errorf("Expected untouched line without matches, got %q", string(out))
	}

	// An open CJSON body ghosts the closing braces and the function paren
	line = []rune("post(/x {a{b")
	painter.Paint(line, len(line))
	if string(painter.Ghost()) != "}})" {
		t.Errorf("Expected ghost '}})', got %q", string(painter.Ghost()))
	}
}

func TestClosingSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"post(/x {a{b", "}})"},
		{"post(/x {a:{b:{c", "}}})"},
		{"post(/x {tags[a,b", "]})"},
		{"post(/x {a[{b:{c", "}}]})"},
		{"get(/u", ")"},
		{"post(/x {a:1}", ")"},
		{"get(/x)", ""},
		{"plain text", ""},
	}
	for _, tt := range tests {
		if got := closingSuffix(tt.input); got != tt.expected {
			t.Errorf("closingSuffix(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEndpointStore(t *testing.T) {
	file := t.TempDir() + "/endpoints.json"
	store := NewEndpointStore(file)

	if !store.Add("http://x.io/", "GET", "/users") {
		t.Error("Expected first Add to report a change")
	}
	if store.Add("http://x.io", "GET", "/users") {
		t.Error("Expected duplicate Add to report no change")
	}
	if !store.Add("http://x.io", "POST", "/users") {
		t.Error("Expected new method to report a change")
	}
	if store.Add("http://x.io", "GET", "") {
		t.Error("Expected empty path to be rejected")
	}

	// Trailing slash on base URL is normalized
	known := store.ForBase("http://x.io/")
	if len(known) != 1 || len(known["/users"]) != 2 {
		t.Fatalf("Expected /users with 2 methods, got %v", known)
	}

	// Other base URLs are isolated
	if store.ForBase("http://other.io") != nil {
		t.Error("Expected no endpoints for unknown base URL")
	}

	// Persistence roundtrip
	store.Save()
	reloaded := NewEndpointStore(file)
	known = reloaded.ForBase("http://x.io")
	if len(known["/users"]) != 2 {
		t.Errorf("Expected reloaded store to keep /users methods, got %v", known)
	}

	reloaded.Clear("http://x.io")
	if reloaded.ForBase("http://x.io") != nil {
		t.Error("Expected Clear to remove endpoints")
	}
}

func TestExtractEndpointPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"get(/users)", "/users"},
		{"get(users)", "/users"},
		{"get(/users?limit=10&offset=20)", "/users"},
		{"post(/login {username:admin})", "/login"},
		{"post(/webhook \"message\")", "/webhook"},
		{"get(/users/${id})", "/users/${id}"},
		{"get(/users/) ", "/users"},
		{"get(/api <custom: value>)", "/api"},
		{"get(/)", "/"},
		{"get()", ""},
		{"get(//cdn.example.com/x)", ""},
		{"nonsense", ""},
	}

	for _, tt := range tests {
		if got := extractEndpointPath(tt.input); got != tt.expected {
			t.Errorf("extractEndpointPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
