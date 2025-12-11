package providers

import "strings"

type Variables struct {
	Vars map[string]interface{}
}

func (p *Variables) GetSuggestions(prefix string, ctx Context) []Suggestion {
	// Only suggest inside parens or after equals
	if ctx != ContextInsideParen && ctx != ContextAfterEquals {
		return nil
	}

	var results []Suggestion
	for name := range p.Vars {
		// Suggest as ${name}
		varText := "${" + name + "}"

		// Match either the prefix or the variable name
		if strings.HasPrefix(varText, prefix) || strings.HasPrefix(name, prefix) {
			results = append(results, Suggestion{
				Text:        varText,
				Display:     varText,
				Description: "variable",
				Score:       75,
			})
		}
	}

	return results
}
