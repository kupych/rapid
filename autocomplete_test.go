package main

import (
	"rapid/providers"
	"testing"
)

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cursor   int
		expected providers.Context
	}{
		{"start of line", "g", 1, providers.ContextStart},
		{"after equals", "id = ", 5, providers.ContextAfterEquals},
		{"after equals with text", "id = g", 5, providers.ContextAfterEquals},
		{"inside paren", "id = get(users", 9, providers.ContextInsideParen},
		{"after method", "id = get(users)", 15, providers.ContextAfterMethod},
		{"after dollar", "id = $", 6, providers.ContextAfterDollar},
		{"inside dollar path", "id = $.dat", 10, providers.ContextAfterDollar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectContext(tt.input, tt.cursor)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommandProvider(t *testing.T) {
	provider := &providers.Command{}

	suggestions := provider.GetSuggestions("g", providers.ContextStart)
	if len(suggestions) != 1 || suggestions[0].Text != "get(" {
		t.Errorf("Expected get( suggestion for prefix 'g'")
	}

	suggestions = provider.GetSuggestions("p", providers.ContextAfterEquals)
	if len(suggestions) < 2 {
		t.Errorf("Expected post(, patch( and put( for prefix 'p'")
	}

	suggestions = provider.GetSuggestions("g", providers.ContextInsideParen)
	if len(suggestions) > 0 {
		t.Errorf("Expected no suggestions")
	}
}
