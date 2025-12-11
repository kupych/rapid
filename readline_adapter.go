package main

// ReadlineCompleter adapts our AutocompleteEngine to readline's interface
type ReadlineCompleter struct {
	engine    *AutocompleteEngine
	variables *map[string]interface{} // Pointer to live variables
}

func NewReadlineCompleter(variables *map[string]interface{}) *ReadlineCompleter {
	return &ReadlineCompleter{
		variables: variables,
	}
}

// Do implements readline.AutoCompleter interface
// line = current buffer, pos = cursor position
// Returns: suggestions as [][]rune, and length to replace
func (c *ReadlineCompleter) Do(line []rune, pos int) ([][]rune, int) {
	// Recreate engine with current variables
	c.engine = NewAutocompleteEngine(*c.variables)

	input := string(line)
	suggestions := c.engine.GetSuggestions(input, pos)

	if len(suggestions) == 0 {
		return nil, 0
	}

	// Get the prefix being completed
	prefix := GetPrefix(input, pos)

	// Convert suggestions to readline format
	var results [][]rune
	for _, sug := range suggestions {
		// readline wants just the SUFFIX to add after removing prefix
		// If suggestion is "get(" and prefix is "g", return "et("
		if len(sug.Text) > len(prefix) {
			suffix := sug.Text[len(prefix):]
			results = append(results, []rune(suffix))
		}
	}

	// Return suggestions and length of prefix to replace
	return results, len(prefix)
}
