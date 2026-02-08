package providers

import (
	"rapid/spec"
	"strings"
)

// OpenAPI provides autocomplete suggestions based on an OpenAPI specification
type OpenAPI struct {
	Spec       *spec.Spec
	basePrefix string // Common path prefix to strip (e.g., "/v1")
}

// GetSuggestions returns endpoint suggestions from the OpenAPI spec
// prefix can be like "get(", "post(", "/users", etc.
func (o *OpenAPI) GetSuggestions(prefix string, ctx Context) []Suggestion {
	// Only suggest in appropriate contexts
	if ctx != ContextStart && ctx != ContextAfterEquals && ctx != ContextInsideParen {
		return nil
	}

	// If no spec is loaded, return no suggestions
	if o.Spec == nil {
		return nil
	}

	var suggestions []Suggestion
	prefixLower := strings.ToLower(prefix)

	// Determine if we're inside parentheses (need to suggest just paths)
	insideParen := ctx == ContextInsideParen

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

			// Strip common base prefix (e.g., /v1/) from path
			displayPath := o.stripBasePrefix(path)

			var suggestionText, displayText string

			if insideParen {
				// Inside parentheses: suggest just the path (strip leading / to match RAPID's syntax)
				pathWithoutLeadingSlash := strings.TrimPrefix(displayPath, "/")
				suggestionText = pathWithoutLeadingSlash
				displayText = pathWithoutLeadingSlash
			} else {
				// At start or after equals: suggest full command
				suggestionText = method + "(" + displayPath + ")"
				displayText = suggestionText
			}

			// Check if it matches the prefix
			if prefixLower != "" {
				if insideParen {
					// Inside paren: match against path without leading /
					pathWithoutSlash := strings.TrimPrefix(displayPath, "/")
					pathLower := strings.ToLower(pathWithoutSlash)

					// Only match if path STARTS with prefix (no fuzzy matching for readline)
					if !strings.HasPrefix(pathLower, prefixLower) {
						continue
					}
				} else {
					// Outside paren: match against full command
					if !strings.HasPrefix(strings.ToLower(suggestionText), prefixLower) {
						// Also try matching just the method or path
						if !strings.HasPrefix(method, prefixLower) && !strings.Contains(strings.ToLower(displayPath), prefixLower) {
							continue
						}
					}
				}
			}

			// Use operation summary as description, fallback to description or path
			description := operation.Summary
			if description == "" {
				description = operation.Description
			}
			if description == "" {
				description = method + " " + path
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

			// Limit to 10 suggestions to keep UI clean
			if len(suggestions) >= 10 {
				return suggestions
			}
		}
	}

	return suggestions
}

// DetectBasePrefix finds the common path prefix (e.g., "/v1") from all paths in the spec
// Returns prefix if 80%+ of paths share it (doesn't need to be 100%)
func (o *OpenAPI) DetectBasePrefix() string {
	if o.Spec == nil || len(o.Spec.Paths) == 0 {
		return ""
	}

	totalPaths := len(o.Spec.Paths)

	// Common prefixes to check (most specific first)
	commonPrefixes := []string{"/v3/", "/v2/", "/v1/", "/api/v3/", "/api/v2/", "/api/v1/", "/api/"}

	for _, prefix := range commonPrefixes {
		// Count how many paths have this prefix
		matchCount := 0
		for path := range o.Spec.Paths {
			if strings.HasPrefix(path, prefix) {
				matchCount++
			}
		}

		// If 80% or more paths have this prefix, use it
		percentage := float64(matchCount) / float64(totalPaths)
		if percentage >= 0.8 {
			return prefix
		}
	}

	return ""
}

// stripBasePrefix removes the common base prefix from a path
func (o *OpenAPI) stripBasePrefix(path string) string {
	if o.basePrefix == "" {
		// Lazily detect base prefix
		o.basePrefix = o.DetectBasePrefix()
		if o.basePrefix == "" {
			o.basePrefix = "NONE" // Mark as checked to avoid re-detection
		}
	}

	if o.basePrefix != "NONE" && strings.HasPrefix(path, o.basePrefix) {
		stripped := strings.TrimPrefix(path, o.basePrefix)
		// Ensure it starts with / if not empty
		if stripped == "" {
			return "/"
		}
		if stripped[0] != '/' {
			return "/" + stripped
		}
		return stripped
	}

	return path
}
