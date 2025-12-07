package main

import (
	"sort"
	"strings"
)

type Context int

const (
	ContextStart Context = iota
	ContextAfterEquals
	ContextInsideParen
	ContextAfterMethod
	ContextAfterDollar
	ContextInsideVariable
	ContextUnknown
)

type SuggestionProvider interface {
	GetSuggestions(prefix string, ctx Context) []Suggestion
}

type AutocompleteEngine struct {
	providers []SuggestionProvider
}

type CommandProvider struct{}
type MetaCommandProvider struct{}

type Suggestion struct {
	Text        string
	Display     string
	Description string
	Score       int
}

func DetectContext(input string, cursorPos int) Context {
	textBeforeCursor := input[:cursorPos]

	if strings.HasPrefix(textBeforeCursor, "$") && !strings.HasPrefix(textBeforeCursor, "$$") {
		return ContextAfterDollar
	}

	// Check for variable assignment
	if strings.Contains(textBeforeCursor, " = ") {
		afterEquals := strings.Split(textBeforeCursor, " = ")[1]

		// Check for $ after equals
		if strings.HasPrefix(afterEquals, "$") && !strings.HasPrefix(afterEquals, "$$") {
			return ContextAfterDollar
		}

		// Inside a request?
		if strings.Contains(afterEquals, "(") && !strings.Contains(afterEquals, ")") {
			return ContextInsideParen
		}

		// After a completed request?
		if strings.HasSuffix(strings.TrimSpace(afterEquals), ")") {
			return ContextAfterMethod
		}

		// Just after equals
		return ContextAfterEquals
	}

	if (strings.HasPrefix(textBeforeCursor, "$") && !strings.HasPrefix(textBeforeCursor, "$$")) || strings.Contains(textBeforeCursor, " $") {
		return ContextAfterDollar
	}

	lastOpenParen := strings.LastIndex(textBeforeCursor, "(")
	lastCloseParen := strings.LastIndex(textBeforeCursor, ")")
	if lastOpenParen > lastCloseParen {
		return ContextInsideParen
	}

	if strings.TrimSpace(textBeforeCursor) == textBeforeCursor && !strings.Contains(textBeforeCursor, " ") {
		return ContextStart
	}

	return ContextUnknown
}

func GetPrefix(input string, cursorPos int) string {
	if cursorPos == 0 {
		return ""
	}

	textBefore := input[:cursorPos]

	words := strings.FieldsFunc(textBefore, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == '=' || r == '.' || r == ','
	})

	if len(words) > 0 {
		return words[len(words)-1]
	}

	return ""
}

func (e *AutocompleteEngine) GetSuggestions(input string, cursorPos int) []Suggestion {
	ctx := DetectContext(input, cursorPos)
	prefix := GetPrefix(input, cursorPos)

	var all []Suggestion

	for _, provider := range e.providers {
		suggestions := provider.GetSuggestions(prefix, ctx)
		all = append(all, suggestions...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})

	return all
}

func (p *CommandProvider) GetSuggestions(prefix string, ctx Context) []Suggestion {
	if ctx != ContextStart && ctx != ContextAfterEquals {
		return nil
	}

	commands := []Suggestion{
		{Text: "get(", Display: "get()", Description: "GET Request", Score: 100},
		{Text: "post(", Display: "post()", Description: "POST Request", Score: 100},
		{Text: "delete(", Display: "delete()", Description: "DELETE Request", Score: 100},
		{Text: "put(", Display: "put()", Description: "PUT Request", Score: 99},
		{Text: "patch(", Display: "patch()", Description: "PATCH Request", Score: 99},
	}

	var results []Suggestion
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Text, prefix) {
			results = append(results, cmd)
		}
	}

	return results
}

func (p *MetaCommandProvider) GetSuggestions(prefix string, ctx Context) []Suggestion {
	if !strings.HasPrefix(prefix, "?") {
		return nil
	}

	commands := []Suggestion{
		{Text: "?v", Display: "?v", Description: "Show variables", Score: 90},
		{Text: "?h", Display: "?h", Description: "Show headers", Score: 90},
		{Text: "?vc", Display: "?vc", Description: "Clear variables", Score: 85},
		{Text: "?hc", Display: "?hc", Description: "Clear headers", Score: 85},
		{Text: "?d", Display: "?d", Description: "Toggle debug", Score: 81},
		{Text: "?", Display: "?", Description: "Show help", Score: 80},
	}

	var results []Suggestion
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Text, prefix) {
			results = append(results, cmd)
		}
	}

	return results
}

func NewAutocompleteEngine() *AutocompleteEngine {
	return &AutocompleteEngine{
		providers: []SuggestionProvider{
			&CommandProvider{},
			&MetaCommandProvider{},
		},
	}
}
