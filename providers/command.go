package providers

import "strings"

type Command struct{}

func (p *Command) GetSuggestions(prefix string, ctx Context) []Suggestion {
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
