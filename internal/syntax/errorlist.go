package syntax

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Formats follow the plain-text cases in Lexilla's LexErrorList.cxx.
var (
	errorListColonLocation = regexp.MustCompile(`^(.+?):-?[0-9]+:`)
	errorListMicrosoft     = regexp.MustCompile(`(?i)^.+\([1-9][0-9]* *(?:, *[0-9]+ *)?\)(?: *: *| +)(?:error|warning|fatal|catastrophic|note|remark)\b`)
	errorListBash          = regexp.MustCompile(`: line [0-9]+:`)
	errorListGCCExcerpt    = regexp.MustCompile(`^[ +0-9]* \|[ +]`)
	errorListCTags         = regexp.MustCompile(`^[^ \t]+\t[^\t]+\t(?:[0-9]+(?:$|[;\t])|/\^)`)
)

type errorListLexer struct{ config chroma.Config }

var _ = lexers.Register(&errorListLexer{config: chroma.Config{Name: "ErrorList"}})

func (l *errorListLexer) Config() *chroma.Config                         { return &l.config }
func (l *errorListLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *errorListLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*errorListLexer) AnalyseText(string) float32                       { return 0 }

func (*errorListLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanErrorListLine), nil
}

func scanErrorListLine(line string, _ struct{}, emit func(int, int, chroma.TokenType)) struct{} {
	if kind := classifyErrorListLine(line); kind != chroma.Text {
		emit(0, len(line), kind)
	}
	return struct{}{}
}

func classifyErrorListLine(line string) chroma.TokenType {
	if line == "" {
		return chroma.Text
	}
	switch line[0] {
	case '>':
		return chroma.GenericPrompt
	case '<':
		return chroma.GenericDeleted
	case '!':
		return chroma.GenericEmph
	case '+', '-':
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			return chroma.GenericHeading
		}
		if line[0] == '+' {
			return chroma.GenericInserted
		}
		return chroma.GenericDeleted
	}

	switch {
	case strings.HasPrefix(line, "In file included from "), strings.HasPrefix(line, "                 from "),
		errorListGCCExcerpt.MatchString(line), errorListCTags.MatchString(line):
		return chroma.GenericEmph
	case errorListOrdered(line, `File "`, ", line "), errorListOrdered(line, " in ", " on line "),
		strings.HasPrefix(line, "   at ") && strings.Contains(line, ":line "),
		strings.HasPrefix(line, "\tat ") && errorListOrdered(line, "(", ".java:"),
		strings.Contains(line, "at line ") && strings.Contains(line, "file "), errorListOrdered(line, " at ", " line "):
		return chroma.GenericTraceback
	case strings.HasPrefix(line, "cf90-"), strings.HasPrefix(line, "fortcom:"),
		strings.HasPrefix(line, "Error "), strings.HasPrefix(line, "Warning "),
		strings.HasPrefix(line, "Line ") && strings.Contains(line, ", file "),
		strings.HasPrefix(line, "line ") && strings.Contains(line, " column "),
		strings.HasPrefix(line, "NMAKE : fatal error"), strings.Contains(line, ": warning C"),
		strings.Contains(line, "warning LNK"), strings.Contains(line, "error LNK"),
		errorListBash.MatchString(line), errorListMicrosoft.MatchString(line):
		return chroma.GenericError
	}
	if location := errorListColonLocation.FindStringSubmatchIndex(line); location != nil {
		if line[0] == '\t' || strings.Contains(line[:location[3]], ": ") {
			return chroma.GenericTraceback
		}
		return chroma.GenericError
	}
	return chroma.Text
}

func errorListOrdered(line, left, right string) bool {
	_, tail, ok := strings.Cut(line, left)
	return ok && strings.Contains(tail, right)
}
