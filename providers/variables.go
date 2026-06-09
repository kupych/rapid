package providers

import (
	"fmt"
	"strings"
)

type Variables struct {
	Vars map[string]interface{}
}

func (p *Variables) GetSuggestions(ctx Context) []Suggestion {
	var results []Suggestion

	switch ctx.Kind {
	case KindVariableRef:
		// Completing inside an open ${...}: insert the name and close the brace
		for name, value := range p.Vars {
			if strings.HasPrefix(name, ctx.Prefix) {
				results = append(results, Suggestion{
					Text:        name + "}",
					Display:     "${" + name + "}",
					Description: valuePreview(value),
					Score:       75,
				})
			}
		}
	case KindPath, KindArgument:
		// Inside parens: offer full ${name} interpolations
		for name, value := range p.Vars {
			full := "${" + name + "}"
			if strings.HasPrefix(full, ctx.Prefix) {
				results = append(results, Suggestion{
					Text:        full,
					Display:     full,
					Description: valuePreview(value),
					Score:       75,
				})
			}
		}
	case KindCommand:
		// Bare names at the start of a line echo the variable's value
		for name, value := range p.Vars {
			if ctx.Prefix != "" && strings.HasPrefix(name, ctx.Prefix) {
				results = append(results, Suggestion{
					Text:        name,
					Display:     name,
					Description: valuePreview(value),
					Score:       60,
				})
			}
		}
	}

	return results
}

func valuePreview(value interface{}) string {
	s := fmt.Sprint(value)
	if r := []rune(s); len(r) > 30 {
		s = string(r[:27]) + "..."
	}
	return s
}
