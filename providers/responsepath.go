package providers

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ResponsePath completes JSON keys from response history, so typing
// `$.da<Tab>` offers the actual fields of the last response.
type ResponsePath struct {
	History *[]string
}

func (p *ResponsePath) GetSuggestions(ctx Context) []Suggestion {
	if ctx.Kind != KindDollarPath || p.History == nil {
		return nil
	}

	history := *p.History
	idx := len(history) - 1 - ctx.HistoryIndex
	if idx < 0 || idx >= len(history) {
		return nil
	}

	node := gjson.Parse(history[idx])
	if ctx.Path != "" {
		node = gjson.Get(history[idx], ctx.Path)
	}
	if !node.Exists() {
		return nil
	}

	var results []Suggestion
	switch {
	case node.IsObject():
		node.ForEach(func(key, value gjson.Result) bool {
			text := escapeKey(key.String())
			if strings.HasPrefix(text, ctx.Prefix) {
				results = append(results, Suggestion{
					Text:        text,
					Display:     text,
					Description: describeValue(value),
					Score:       80,
				})
			}
			return true
		})
	case node.IsArray():
		count := len(node.Array())
		candidates := []Suggestion{
			{Text: "0", Display: "0", Description: "first element", Score: 80},
			{Text: "#", Display: "#", Description: fmt.Sprintf("count (%d)", count), Score: 79},
		}
		for _, c := range candidates {
			if strings.HasPrefix(c.Text, ctx.Prefix) {
				results = append(results, c)
			}
		}
	}

	return results
}

// escapeKey escapes characters that are path syntax in gjson
func escapeKey(key string) string {
	r := strings.NewReplacer(".", `\.`, "*", `\*`, "?", `\?`, "#", `\#`)
	return r.Replace(key)
}

func describeValue(value gjson.Result) string {
	switch {
	case value.IsObject():
		count := 0
		value.ForEach(func(_, _ gjson.Result) bool { count++; return true })
		return fmt.Sprintf("object {%d}", count)
	case value.IsArray():
		return fmt.Sprintf("array [%d]", len(value.Array()))
	default:
		preview := value.String()
		if len(preview) > 40 {
			preview = preview[:37] + "..."
		}
		return preview
	}
}
