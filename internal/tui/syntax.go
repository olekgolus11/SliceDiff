package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

var diffSyntaxStyle = chromastyles.Get("github-dark")

func highlightDiffCode(filePath, content string, base lipgloss.Style) (string, bool) {
	if content == "" || diffSyntaxStyle == nil {
		return content, false
	}

	lexer := lexers.Match(filePath)
	if lexer == nil {
		return content, false
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content, false
	}

	var highlighted strings.Builder
	for token := iterator(); token != chroma.EOF; token = iterator() {
		style := base
		entry := diffSyntaxStyle.Get(token.Type)
		if entry.Colour.IsSet() {
			style = style.Foreground(lipgloss.Color(entry.Colour.String()))
		}
		if entry.Bold == chroma.Yes {
			style = style.Bold(true)
		}
		if entry.Italic == chroma.Yes {
			style = style.Italic(true)
		}
		if entry.Underline == chroma.Yes {
			style = style.Underline(true)
		}
		highlighted.WriteString(style.Render(token.Value))
	}

	result := strings.TrimRight(highlighted.String(), "\r\n")
	if result == "" {
		return content, false
	}
	return result, true
}
