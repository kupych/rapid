package main

import (
	"fmt"

	"github.com/chzyer/readline"
	"github.com/chzyer/readline/runes"
)

// GhostPainter renders the top autocomplete suggestion as dim inline text
// after the cursor, fish-style. Right arrow (or Ctrl+F) accepts it.
type GhostPainter struct {
	completer   *ReadlineCompleter
	promptWidth int
	ghost       []rune // suggestion currently on screen, nil if none
}

func NewGhostPainter(completer *ReadlineCompleter, prompt string) *GhostPainter {
	return &GhostPainter{
		completer:   completer,
		promptWidth: runes.WidthAll([]rune(prompt)),
	}
}

// Ghost returns the suggestion currently rendered, for the accept key
func (p *GhostPainter) Ghost() []rune {
	return p.ghost
}

// Paint implements readline.Painter. The returned runes are display-only;
// readline positions the cursor from the real buffer, so after the dim
// suggestion we emit a cursor-left escape to land back at the input's end.
func (p *GhostPainter) Paint(line []rune, pos int) []rune {
	p.ghost = nil

	if pos != len(line) || len(line) == 0 {
		return line
	}

	suffixes, _ := p.completer.Do(line, pos)
	if len(suffixes) == 0 {
		return line
	}
	ghost := suffixes[0]

	// The cursor-left escape cannot cross rows, so skip the ghost when it
	// would make the line wrap
	if w := readline.GetScreenWidth(); w > 0 &&
		p.promptWidth+runes.WidthAll(line)+runes.WidthAll(ghost) >= w {
		return line
	}

	p.ghost = ghost

	out := make([]rune, 0, len(line)+len(ghost)+16)
	out = append(out, line...)
	out = append(out, []rune(CGray)...)
	out = append(out, ghost...)
	out = append(out, []rune(CReset)...)
	out = append(out, []rune(fmt.Sprintf("\033[%dD", runes.WidthAll(ghost)))...)
	return out
}
