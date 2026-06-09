package providers

import "strings"

// Endpoints suggests paths learned from successful requests. Paths may
// contain {id} wildcard segments, e.g. /users/{id}/posts.
type Endpoints struct {
	Paths map[string][]string // path -> methods that have succeeded
}

func (p *Endpoints) GetSuggestions(ctx Context) []Suggestion {
	if ctx.Kind != KindPath {
		return nil
	}

	arg := ctx.Arg
	if !strings.HasPrefix(arg, "/") {
		arg = "/" + arg
	}
	argSegs := strings.Split(arg[1:], "/")

	var results []Suggestion
	for path, methods := range p.Paths {
		rest, ok := templateContinuation(path, argSegs)
		if !ok {
			continue
		}

		score := 72
		for _, m := range methods {
			if m == ctx.Command {
				score = 85
			}
		}

		results = append(results, Suggestion{
			Text:        ctx.Prefix + rest,
			Display:     path,
			Description: strings.Join(methods, ", "),
			Score:       score,
		})
	}

	return results
}

// templateContinuation returns the text that extends the typed segments
// along the template, stopping right after the slash before the next {id}
// hole so the cursor lands where the ID goes.
func templateContinuation(path string, argSegs []string) (string, bool) {
	tplSegs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	n := len(argSegs)
	if len(tplSegs) < n {
		return "", false
	}

	// Fully typed segments must match the template, with {id} taking anything
	for i := 0; i < n-1; i++ {
		if tplSegs[i] != argSegs[i] && !(tplSegs[i] == "{id}" && argSegs[i] != "") {
			return "", false
		}
	}

	// The cursor is in an ID hole - the user has to type it themselves
	if tplSegs[n-1] == "{id}" {
		return "", false
	}

	last := argSegs[n-1]
	if !strings.HasPrefix(tplSegs[n-1], last) {
		return "", false
	}

	rest := tplSegs[n-1][len(last):]
	for _, seg := range tplSegs[n:] {
		if seg == "{id}" {
			rest += "/"
			break
		}
		rest += "/" + seg
	}

	if rest == "" {
		return "", false
	}

	return rest, true
}
