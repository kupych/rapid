package providers

import (
	"rapid/spec"
	"strings"
)

// OpenAPI provides autocomplete suggestions based on an OpenAPI specification
type OpenAPI struct {
	Spec *spec.Spec
}

// GetSuggestions returns endpoint suggestions from the OpenAPI spec
func (o *OpenAPI) GetSuggestions(prefix string, ctx Context) []Suggestion {
	// Only suggest in appropriate contexts
	if ctx != ContextStart && ctx != ContextAfterEquals {
		return nil
	}

	// If no spec is loaded, return no suggestions
	if o.Spec == nil {
		return nil
	}

	var suggestions []Suggestion
	prefix = strings.ToLower(prefix)

	// Iterate through all paths and operations
	for path, pathItem := range o.Spec.Paths {
		// Check each HTTP method
		operations := map[string]*spec.Operation{
			"get":    pathItem.Get,
			"post":   pathItem.Post,
			"put":    pathItem.Put,
			"patch":  pathItem.Patch,
			"delete": pathItem.Delete,
		}

		for method, operation := range operations {
			if operation == nil {
				continue
			}

			// Create suggestion text: "method(path)"
			suggestionText := method + "(" + path + ")"
			displayText := suggestionText

			// Check if it matches the prefix
			if prefix != "" && !strings.HasPrefix(strings.ToLower(suggestionText), prefix) {
				// Also try matching just the method or path
				if !strings.HasPrefix(method, prefix) && !strings.Contains(strings.ToLower(path), prefix) {
					continue
				}
			}

			// Use operation summary as description, fallback to description or path
			description := operation.Summary
			if description == "" {
				description = operation.Description
			}
			if description == "" {
				description = path
			}

			// Truncate long descriptions
			if len(description) > 80 {
				description = description[:77] + "..."
			}

			suggestions = append(suggestions, Suggestion{
				Text:        suggestionText,
				Display:     displayText,
				Description: description,
				Score:       95, // High priority for spec-based suggestions
			})

			// Limit to 50 suggestions to avoid overwhelming the user
			if len(suggestions) >= 50 {
				return suggestions
			}
		}
	}

	return suggestions
}
