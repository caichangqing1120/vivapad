package syntax

import (
	_ "embed"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
)

// Word groups from Notepad++ v8.9.8.1 langs.model.xml, in reference order.
//
//go:embed baanc_words.txt
var baanWordData string

type baanWordGroup struct {
	words    map[string]struct{}
	patterns []baanPattern
	kind     chroma.TokenType
}

type baanPattern struct{ prefix, suffix string }

var baanWordGroups = func() []baanWordGroup {
	kinds := [...]chroma.TokenType{chroma.Keyword, chroma.NameBuiltin, chroma.NameBuiltin,
		chroma.NameLabel, chroma.KeywordNamespace, chroma.NameVariable, chroma.NameProperty,
		chroma.NameConstant, chroma.NameBuiltin}
	groups := make([]baanWordGroup, len(kinds))
	for i, line := range strings.SplitN(strings.TrimSpace(baanWordData), "\n", len(kinds)) {
		group := baanWordGroup{kind: kinds[i], words: make(map[string]struct{})}
		for _, word := range strings.Fields(line) {
			word = strings.ToLower(word)
			switch {
			case strings.HasPrefix(word, "^"):
				group.patterns = append(group.patterns, baanPattern{prefix: word[1:]})
			case strings.Contains(word, "~"):
				prefix, suffix, _ := strings.Cut(word, "~")
				group.patterns = append(group.patterns, baanPattern{prefix, suffix})
			default:
				group.words[word] = struct{}{}
			}
		}
		groups[i] = group
	}
	return groups
}()

func (group baanWordGroup) contains(word string) bool {
	if _, ok := group.words[word]; ok {
		return true
	}
	for _, pattern := range group.patterns {
		if len(word) >= len(pattern.prefix)+len(pattern.suffix) &&
			strings.HasPrefix(word, pattern.prefix) && strings.HasSuffix(word, pattern.suffix) {
			return true
		}
	}
	return false
}

type baanState uint8

const (
	baanCode baanState = iota
	baanUsage
	baanDirective
	baanString
)

type baanLexer struct{ config chroma.Config }

var _ = lexers.Register(&baanLexer{config: chroma.Config{Name: "BaanC", Aliases: []string{"baanc"}}})

func (l *baanLexer) Config() *chroma.Config                         { return &l.config }
func (l *baanLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *baanLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*baanLexer) AnalyseText(string) float32                       { return 0 }

func (*baanLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanBaanLine), nil
}

type BaanCHighlighter struct{ *lineHighlighter[baanState] }

func NewBaanCHighlighter() *BaanCHighlighter {
	h := newLineHighlighter(scanBaanLine)
	h.color = baanTokenColor
	return &BaanCHighlighter{h}
}

func baanTokenColor(kind chroma.TokenType) (gfx.Color, bool) {
	switch kind {
	case chroma.NameAttribute, chroma.NameLabel, chroma.NameBuiltin, chroma.NameClass:
		return tokenColor(chroma.NameFunction)
	case chroma.NameVariable, chroma.NameProperty, chroma.NameConstant:
		return tokenColor(chroma.LiteralNumber)
	case chroma.CommentPreproc:
		return tokenColor(chroma.Keyword)
	default:
		return tokenColor(kind)
	}
}

func scanBaanLine(line string, state baanState, emit func(int, int, chroma.TokenType)) baanState {
	line = strings.TrimSuffix(line, "\r")
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && end > start && kind != chroma.Text {
			emit(start, end, kind)
		}
	}
	i := 0
	if state == baanUsage {
		i = baanUsageEnd(line)
		if i == 0 {
			style(0, len(line), chroma.CommentMultiline)
			return baanUsage
		}
		style(0, i, chroma.CommentMultiline)
	} else if strings.HasPrefix(line, "^") {
		switch state {
		case baanDirective:
			style(0, len(line), chroma.CommentPreproc)
			return baanDirective
		case baanString:
			end := strings.IndexByte(line, '"')
			if end < 0 {
				style(0, len(line), chroma.LiteralStringDouble)
				return baanString
			}
			i = end + 1
			style(0, i, chroma.LiteralStringDouble)
		}
	}

	first := i == 0
	var declaration chroma.TokenType
	for i < len(line) {
		start := i
		ch := line[i]
		i++
		if ch == ' ' || ch == '\t' {
			continue
		}
		switch {
		case ch == '|':
			style(start, len(line), chroma.CommentSingle)
			return baanCode
		case ch == '"':
			end := strings.IndexByte(line[i:], '"')
			if end < 0 {
				style(start, len(line), chroma.LiteralStringDouble)
				return baanString
			}
			i += end + 1
			style(start, i, chroma.LiteralStringDouble)
		case ch == '#' && first:
			for i < len(line) && (baanLetter(line[i]) || baanDigit(line[i]) || line[i] == '_') {
				i++
			}
			switch strings.ToLower(line[start:i]) {
			case "#context_off", "#context_on", "#define", "#elif", "#else", "#endif", "#ident", "#if", "#ifdef", "#ifndef", "#include", "#pragma", "#undef":
				style(start, len(line), chroma.CommentPreproc)
				return baanDirective
			}
		case baanStartsNumber(line[start:]):
			end := baanNumberEnd(line[start:]) + start
			if end > start && (end == len(line) || !baanWordByte(line[end])) {
				i = end
				style(start, end, chroma.LiteralNumber)
			} else {
				i = max(i, end)
				for i < len(line) && baanWordByte(line[i]) {
					i++
				}
			}
		case baanLetter(ch) || ch == '_':
			for i < len(line) && baanWordByte(line[i]) {
				i++
			}
			if i-start > 999 {
				break
			}
			word := strings.ToLower(line[start:i])
			if word == "dllusage" || word == "functionusage" {
				style(start, len(line), chroma.CommentMultiline)
				return baanUsage
			}
			if baanWordGroups[0].contains(word) {
				style(start, i, chroma.Keyword)
				switch word {
				case "domain":
					declaration = chroma.NameClass
				case "function":
					declaration = chroma.NameFunction
				}
			} else if declaration != 0 {
				style(start, i, declaration)
				declaration = 0
			} else if emit != nil {
				var following byte
				if i < len(line) {
					following = line[i]
				}
				style(start, i, baanIdentifierKind(word, following))
			}
		case strings.ContainsRune("#^&*()-+=|{}[]:;<>,/?!~\\", rune(ch)):
			style(start, i, chroma.Operator)
		}
		first = false
	}
	return baanCode
}

func baanUsageEnd(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i]|0x20 != 'e' {
			continue
		}
		for _, closer := range [...]string{"enddllusage", "endfunctionusage"} {
			end := i + len(closer)
			if end <= len(line) && strings.EqualFold(line[i:end], closer) {
				return end
			}
		}
	}
	return 0
}

func baanIdentifierKind(word string, following byte) chroma.TokenType {
	for i, group := range baanWordGroups[1:] {
		candidate := word
		switch i + 1 {
		case 2:
			if following != '(' {
				continue
			}
		case 3, 4:
			if following != ':' {
				continue
			}
			candidate += ":"
		}
		if group.contains(candidate) {
			return group.kind
		}
	}
	base, field, qualified := strings.Cut(word, ".")
	if baanTableName(base) {
		index, indexed := strings.CutPrefix(field, "_index")
		if !qualified || len(base) == 8 && (field == "_compnr" || indexed && (len(index) == 1 || len(index) == 2) && baanDigits(index)) {
			return chroma.NameClass
		}
		if len(base) == 8 && (field == "" || len(field) >= 4 && len(field) <= 6) {
			return chroma.NameAttribute
		}
	}
	if table, ok := strings.CutPrefix(word, "rcd.t"); ok && baanTableName(table) {
		return chroma.NameClass
	}
	if qualified && len(base) == 5 && baanLetters(base) && len(field) > 8 &&
		strings.HasPrefix(field, "dll") && field[7] == '.' && baanDigits(field[3:7]) {
		return chroma.NameFunction
	}
	if qualified && len(base) == 5 && baanLetters(base[:2]) && base[2:] == "int" && len(field) > 10 &&
		strings.HasPrefix(field, "dll") && baanLetters(field[3:8]) && field[8] == '.' {
		return chroma.NameFunction
	}
	if qualified && len(base) == 10 && base[0] == 'i' && baanLetters(base[1:6]) && baanDigits(base[6:]) {
		return chroma.NameFunction
	}
	return chroma.Text
}

func baanTableName(word string) bool {
	if len(word) == 9 && word[0] == 't' {
		word = word[1:]
	}
	return len(word) == 8 && baanLetters(word[:5]) && baanDigits(word[5:])
}

func baanLetters(word string) bool {
	for i := range len(word) {
		if !baanLetter(word[i]) {
			return false
		}
	}
	return true
}

func baanDigits(word string) bool {
	for i := range len(word) {
		if !baanDigit(word[i]) {
			return false
		}
	}
	return true
}

func baanDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func baanLetter(ch byte) bool { ch |= 0x20; return ch >= 'a' && ch <= 'z' }
func baanWordByte(ch byte) bool {
	return baanLetter(ch) || baanDigit(ch) || ch == '.' || ch == '_' || ch == '$'
}

func baanStartsNumber(line string) bool {
	if line[0] == '-' || line[0] == '+' {
		line = line[1:]
	}
	return len(line) != 0 && (baanDigit(line[0]) || len(line) > 1 && line[0] == '.' && baanDigit(line[1]))
}

func baanNumberEnd(line string) int {
	i := 0
	if line[0] == '+' || line[0] == '-' {
		i++
	}
	if len(line)-i > 2 && line[i] == '0' && line[i+1]|0x20 == 'x' {
		i += 2
		hexStart := i
		for i < len(line) && (baanDigit(line[i]) || line[i]|0x20 >= 'a' && line[i]|0x20 <= 'f') {
			i++
		}
		if i == hexStart {
			return 0
		}
		return i
	}
	for i < len(line) && baanDigit(line[i]) {
		i++
	}
	if i < len(line) && line[i] == '.' {
		i++
		for i < len(line) && baanDigit(line[i]) {
			i++
		}
	}
	if i < len(line) && line[i]|0x20 == 'e' {
		end := i + 1
		if end < len(line) && (line[end] == '+' || line[end] == '-') {
			end++
		}
		if end < len(line) && baanDigit(line[end]) {
			i = end + 1
			for i < len(line) && baanDigit(line[i]) {
				i++
			}
		}
	}
	return i
}
