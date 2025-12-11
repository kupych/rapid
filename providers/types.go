package providers

type Context int

const (
	ContextStart Context = iota
	ContextAfterEquals
	ContextInsideParen
	ContextAfterMethod
	ContextAfterDollar
	ContextInsideVariable
	ContextUnknown
)

type Suggestion struct {
	Text        string
	Display     string
	Description string
	Score       int
}

type SuggestionProvider interface {
	GetSuggestions(prefix string, ctx Context) []Suggestion
}
