package tui

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

var (
	diffSyntaxFormatter = formatters.Get("terminal16m")
	diffSyntaxStyle     = chromastyles.Get("github-dark")
)

func highlightDiffCode(filePath, content string) (string, bool) {
	if content == "" || diffSyntaxFormatter == nil || diffSyntaxStyle == nil {
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

	var buf bytes.Buffer
	if err := diffSyntaxFormatter.Format(&buf, diffSyntaxStyle, iterator); err != nil {
		return content, false
	}

	highlighted := strings.TrimRight(buf.String(), "\r\n")
	if highlighted == "" {
		return content, false
	}
	return highlighted, true
}
