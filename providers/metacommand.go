package providers

import "strings"

type MetaCommand struct{}

func (p *MetaCommand) GetSuggestions(ctx Context) []Suggestion {
	if ctx.Kind != KindMeta {
		return nil
	}

	commands := []Suggestion{
		{Text: "?v", Display: "?v", Description: "Show variables", Score: 90},
		{Text: "?h", Display: "?h", Description: "Show headers", Score: 90},
		{Text: "?s", Display: "?s", Description: "Session info", Score: 86},
		{Text: "?vc", Display: "?vc", Description: "Clear variables", Score: 85},
		{Text: "?hc", Display: "?hc", Description: "Clear headers", Score: 85},
		{Text: "?e", Display: "?e", Description: "Show learned endpoints", Score: 84},
		{Text: "?ec", Display: "?ec", Description: "Forget learned endpoints", Score: 83},
		{Text: "?d", Display: "?d", Description: "Toggle debug", Score: 81},
		{Text: "??", Display: "??", Description: "Preview autocomplete", Score: 81},
		{Text: "?", Display: "?", Description: "Show help", Score: 80},
	}

	var results []Suggestion
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Text, ctx.Prefix) {
			results = append(results, cmd)
		}
	}

	return results
}
