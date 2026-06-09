package main

import (
	"rapid/providers"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type AutocompleteEngine struct {
	providers []providers.SuggestionProvider
}

func NewAutocompleteEngine(variables map[string]interface{}, headers map[string]string, history *[]string, endpoints map[string][]string) *AutocompleteEngine {
	return &AutocompleteEngine{
		providers: []providers.SuggestionProvider{
			&providers.Command{},
			&providers.MetaCommand{},
			&providers.Variables{Vars: variables},
			&providers.Headers{Session: headers},
			&providers.ResponsePath{History: history},
			&providers.Endpoints{Paths: endpoints},
		},
	}
}

// matches a $N.path response accessor ending at the cursor
var dollarPathRe = regexp.MustCompile(`\$(\d*)(\.[^\s]*)?$`)

// Analyze determines what kind of token is being completed at the cursor
func Analyze(input string, cursorPos int) providers.Context {
	if cursorPos > len(input) {
		cursorPos = len(input)
	}
	text := input[:cursorPos]

	// Metacommand: line starts with ? and we're still in the first word
	if strings.HasPrefix(text, "?") && !strings.Contains(text, " ") {
		return providers.Context{Kind: providers.KindMeta, Prefix: text}
	}

	// Unclosed ${ -> variable reference
	if idx := strings.LastIndex(text, "${"); idx != -1 && !strings.Contains(text[idx:], "}") {
		return providers.Context{Kind: providers.KindVariableRef, Prefix: text[idx+2:]}
	}

	// $N.path response accessor
	if ctx, ok := dollarContext(text); ok {
		return ctx
	}

	// Unclosed <...> -> inline header
	if lt := strings.LastIndex(text, "<"); lt > strings.LastIndex(text, ">") {
		seg := text[lt+1:]
		if !strings.Contains(seg, ":") {
			return providers.Context{Kind: providers.KindHeaderName, Prefix: seg}
		}
		// Completing a header value - nothing to suggest
		return providers.Context{Kind: providers.KindUnknown}
	}

	// Inside request parens
	if lastOpen := strings.LastIndex(text, "("); lastOpen > strings.LastIndex(text, ")") {
		seg := text[lastOpen+1:]
		// No whitespace yet means the cursor is still in the path token
		if !strings.ContainsAny(seg, " \t") {
			return providers.Context{
				Kind:    providers.KindPath,
				Prefix:  currentWord(text),
				Arg:     seg,
				Command: methodBefore(text[:lastOpen]),
			}
		}
		return providers.Context{Kind: providers.KindArgument, Prefix: currentWord(text)}
	}

	// Right side of an assignment: `id = g...`
	if i := strings.Index(text, " = "); i != -1 {
		after := text[i+3:]
		if !strings.Contains(after, " ") {
			return providers.Context{Kind: providers.KindCommand, Prefix: after}
		}
		return providers.Context{Kind: providers.KindUnknown}
	}

	// First word of the line
	if !strings.Contains(text, " ") {
		return providers.Context{Kind: providers.KindCommand, Prefix: text}
	}

	return providers.Context{Kind: providers.KindUnknown}
}

// dollarContext parses a trailing $N.path expression like `$.data.us` or `$2.items.`
func dollarContext(text string) (providers.Context, bool) {
	loc := dollarPathRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return providers.Context{}, false
	}

	// The $ must start the expression: preceded by nothing, space, = or (
	start := loc[0]
	if start > 0 {
		prev := text[start-1]
		if prev != ' ' && prev != '=' && prev != '(' {
			return providers.Context{}, false
		}
	}

	digits := text[loc[2]:loc[3]]
	historyIndex := 0
	if digits != "" {
		historyIndex, _ = strconv.Atoi(digits)
	}

	// Without a dot the user is still picking the history index - nothing to suggest
	if loc[4] == -1 {
		return providers.Context{}, false
	}

	path := text[loc[4]+1 : loc[5]] // skip the leading dot
	parent, prefix := "", path
	if i := strings.LastIndex(path, "."); i != -1 {
		parent, prefix = path[:i], path[i+1:]
	}

	return providers.Context{
		Kind:         providers.KindDollarPath,
		Prefix:       prefix,
		HistoryIndex: historyIndex,
		Path:         parent,
	}, true
}

var methodNames = map[string]string{
	"get": "GET", "g": "GET", "ge": "GET",
	"post": "POST", "p": "POST", "po": "POST", "pos": "POST",
	"put": "PUT", "pu": "PUT",
	"patch": "PATCH", "pa": "PATCH", "pat": "PATCH",
	"delete": "DELETE", "d": "DELETE", "del": "DELETE",
}

// methodBefore resolves the request command ending at text, e.g. "id = get" -> GET
func methodBefore(text string) string {
	i := len(text)
	for i > 0 {
		c := text[i-1]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			break
		}
		i--
	}
	return methodNames[strings.ToLower(text[i:])]
}

// currentWord returns the token being typed at the end of text
func currentWord(text string) string {
	if text == "" {
		return ""
	}

	isDelim := func(r rune) bool {
		return strings.ContainsRune(" ()=.,/{}:&?<>", r)
	}

	if isDelim(rune(text[len(text)-1])) {
		return ""
	}

	words := strings.FieldsFunc(text, isDelim)
	if len(words) > 0 {
		return words[len(words)-1]
	}

	return ""
}

// Complete returns ranked suggestions plus the context they apply to
func (e *AutocompleteEngine) Complete(input string, cursorPos int) ([]providers.Suggestion, providers.Context) {
	ctx := Analyze(input, cursorPos)

	var all []providers.Suggestion
	for _, provider := range e.providers {
		all = append(all, provider.GetSuggestions(ctx)...)
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Text < all[j].Text
	})

	// Dedupe by text, keeping the highest-scored entry
	seen := make(map[string]bool)
	results := all[:0]
	for _, s := range all {
		if !seen[s.Text] {
			seen[s.Text] = true
			results = append(results, s)
		}
	}

	return results, ctx
}

func (e *AutocompleteEngine) GetSuggestions(input string, cursorPos int) []providers.Suggestion {
	suggestions, _ := e.Complete(input, cursorPos)
	return suggestions
}
