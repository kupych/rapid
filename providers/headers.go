package providers

import "strings"

// Headers completes header names inside an open <...> in a request.
type Headers struct {
	Session map[string]string
}

var commonHeaders = []Suggestion{
	{Text: "content-type: ", Display: "content-type:", Description: "common header", Score: 60},
	{Text: "accept: ", Display: "accept:", Description: "common header", Score: 60},
	{Text: "authorization: ", Display: "authorization:", Description: "common header", Score: 60},
	{Text: "x-api-key: ", Display: "x-api-key:", Description: "common header", Score: 60},
	{Text: "user-agent: ", Display: "user-agent:", Description: "common header", Score: 60},
	{Text: "cache-control: ", Display: "cache-control:", Description: "common header", Score: 60},
	{Text: "if-none-match: ", Display: "if-none-match:", Description: "common header", Score: 60},
	{Text: "cookie: ", Display: "cookie:", Description: "common header", Score: 60},
}

func (p *Headers) GetSuggestions(ctx Context) []Suggestion {
	if ctx.Kind != KindHeaderName {
		return nil
	}

	prefix := strings.ToLower(ctx.Prefix)

	var results []Suggestion
	for name := range p.Session {
		text := strings.ToLower(name) + ": "
		if strings.HasPrefix(text, prefix) {
			results = append(results, Suggestion{
				Text:        text,
				Display:     strings.ToLower(name) + ":",
				Description: "session header",
				Score:       70,
			})
		}
	}

	for _, h := range commonHeaders {
		if strings.HasPrefix(h.Text, prefix) {
			results = append(results, h)
		}
	}

	return results
}
