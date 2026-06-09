package providers

import "strings"

type Command struct{}

func (p *Command) GetSuggestions(ctx Context) []Suggestion {
	if ctx.Kind != KindCommand {
		return nil
	}

	commands := []Suggestion{
		{Text: "get(", Display: "get()", Description: "GET request", Score: 100},
		{Text: "post(", Display: "post()", Description: "POST request", Score: 100},
		{Text: "delete(", Display: "delete()", Description: "DELETE request", Score: 100},
		{Text: "put(", Display: "put()", Description: "PUT request", Score: 99},
		{Text: "patch(", Display: "patch()", Description: "PATCH request", Score: 99},
	}

	var results []Suggestion
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Text, ctx.Prefix) {
			results = append(results, cmd)
		}
	}

	return results
}
