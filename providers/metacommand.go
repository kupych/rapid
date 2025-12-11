package providers

import "strings"

type MetaCommand struct{}

func (p *MetaCommand) GetSuggestions(prefix string, ctx Context) []Suggestion {
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
