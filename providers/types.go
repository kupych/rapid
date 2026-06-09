package providers

type ContextKind int

const (
	KindUnknown     ContextKind = iota
	KindCommand                 // first word of the line, or right side of `=`: request commands
	KindMeta                    // ?-prefixed metacommand
	KindPath                    // inside request parens, still in the path token
	KindArgument                // inside request parens, past the path: bodies, variables
	KindVariableRef             // inside an unclosed ${...}
	KindDollarPath              // $N.path response history accessor
	KindHeaderName              // inside an unclosed <...>, before the colon
)

type Context struct {
	Kind         ContextKind
	Prefix       string // the text being completed; Suggestion.Text replaces it
	HistoryIndex int    // KindDollarPath: which response ($ = 0, $1 = 1, ...)
	Path         string // KindDollarPath: parent path already typed, "" = root
	Arg          string // KindPath: the full path token typed so far
	Command      string // KindPath: HTTP method of the surrounding request
}

type Suggestion struct {
	Text        string // full replacement for Context.Prefix
	Display     string
	Description string
	Score       int
}

type SuggestionProvider interface {
	GetSuggestions(ctx Context) []Suggestion
}
