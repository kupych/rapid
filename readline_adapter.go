package main

import "strings"

// ReadlineCompleter adapts our AutocompleteEngine to readline's interface
type ReadlineCompleter struct {
	variables *map[string]interface{} // Pointers to live session state
	headers   *map[string]string
	history   *[]string
	endpoints *EndpointStore
	baseURL   string
}

func NewReadlineCompleter(variables *map[string]interface{}, headers *map[string]string, history *[]string, endpoints *EndpointStore, baseURL string) *ReadlineCompleter {
	return &ReadlineCompleter{
		variables: variables,
		headers:   headers,
		history:   history,
		endpoints: endpoints,
		baseURL:   baseURL,
	}
}

// Do implements readline.AutoCompleter interface
// line = current buffer, pos = cursor position (in runes)
// Returns: candidate suffixes as [][]rune, and prefix length to replace
func (c *ReadlineCompleter) Do(line []rune, pos int) ([][]rune, int) {
	// Recreate engine with current session state
	engine := NewAutocompleteEngine(*c.variables, *c.headers, c.history, c.endpoints.ForBase(c.baseURL))

	input := string(line)
	bytePos := len(string(line[:pos]))
	suggestions, ctx := engine.Complete(input, bytePos)

	prefixLen := len([]rune(ctx.Prefix))

	// readline appends the returned runes after the cursor, so only
	// suggestions that actually extend the typed prefix are completable
	var results [][]rune
	for _, sug := range suggestions {
		runes := []rune(sug.Text)
		if len(runes) > prefixLen && strings.EqualFold(string(runes[:prefixLen]), ctx.Prefix) {
			results = append(results, runes[prefixLen:])
		}
	}

	return results, prefixLen
}
