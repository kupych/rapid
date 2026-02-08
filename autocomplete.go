package main

import (
	"rapid/providers"
	"rapid/spec"
	"sort"
	"strings"
)

type AutocompleteEngine struct {
	providers []providers.SuggestionProvider
}

func DetectContext(input string, cursorPos int) providers.Context {
	textBeforeCursor := input[:cursorPos]

	if strings.HasPrefix(textBeforeCursor, "$") && !strings.HasPrefix(textBeforeCursor, "$$") {
		return providers.ContextAfterDollar
	}

	// Check for variable assignment
	if strings.Contains(textBeforeCursor, " = ") {
		afterEquals := strings.Split(textBeforeCursor, " = ")[1]

		// Check for $ after equals
		if strings.HasPrefix(afterEquals, "$") && !strings.HasPrefix(afterEquals, "$$") {
			return providers.ContextAfterDollar
		}

		// Inside a request?
		if strings.Contains(afterEquals, "(") && !strings.Contains(afterEquals, ")") {
			return providers.ContextInsideParen
		}

		// After a completed request?
		if strings.HasSuffix(strings.TrimSpace(afterEquals), ")") {
			return providers.ContextAfterMethod
		}

		// Just after equals
		return providers.ContextAfterEquals
	}

	if (strings.HasPrefix(textBeforeCursor, "$") && !strings.HasPrefix(textBeforeCursor, "$$")) || strings.Contains(textBeforeCursor, " $") {
		return providers.ContextAfterDollar
	}

	lastOpenParen := strings.LastIndex(textBeforeCursor, "(")
	lastCloseParen := strings.LastIndex(textBeforeCursor, ")")
	if lastOpenParen > lastCloseParen {
		return providers.ContextInsideParen
	}

	if strings.TrimSpace(textBeforeCursor) == textBeforeCursor && !strings.Contains(textBeforeCursor, " ") {
		return providers.ContextStart
	}

	return providers.ContextUnknown
}

func GetPrefix(input string, cursorPos int) string {
	if cursorPos == 0 {
		return ""
	}

	textBefore := input[:cursorPos]

	// If last char is a delimiter, we're starting fresh
	lastChar := textBefore[len(textBefore)-1]
	if lastChar == ' ' || lastChar == '(' || lastChar == ')' ||
		lastChar == '=' || lastChar == '.' || lastChar == ',' {
		return ""
	}

	words := strings.FieldsFunc(textBefore, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == '=' || r == '.' || r == ','
	})

	if len(words) > 0 {
		return words[len(words)-1]
	}

	return ""
}

func (e *AutocompleteEngine) GetSuggestions(input string, cursorPos int) []providers.Suggestion {
	ctx := DetectContext(input, cursorPos)
	prefix := GetPrefix(input, cursorPos)

	var all []providers.Suggestion

	for _, provider := range e.providers {
		suggestions := provider.GetSuggestions(prefix, ctx)
		all = append(all, suggestions...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})

	return all
}

func NewAutocompleteEngine(variables map[string]interface{}, openAPISpec *spec.Spec) *AutocompleteEngine {
	providerList := []providers.SuggestionProvider{
		&providers.Command{},
	}

	// Add OpenAPI provider if spec is loaded
	if openAPISpec != nil {
		providerList = append(providerList, &providers.OpenAPI{Spec: openAPISpec})
	}

	// Add remaining providers
	providerList = append(providerList,
		&providers.MetaCommand{},
		&providers.Variables{Vars: variables},
	)

	return &AutocompleteEngine{
		providers: providerList,
	}
}
