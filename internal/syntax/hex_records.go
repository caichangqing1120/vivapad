package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type hexRecordLexer struct {
	config chroma.Config
	marker byte
}

func (lexer *hexRecordLexer) Config() *chroma.Config                         { return &lexer.config }
func (lexer *hexRecordLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return lexer }
func (lexer *hexRecordLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return lexer }
func (*hexRecordLexer) AnalyseText(string) float32                           { return 0 }

func init() {
	for _, lexer := range []*hexRecordLexer{
		{config: chroma.Config{Name: "Intel HEX", Aliases: []string{"ihex"}}, marker: ':'},
		{config: chroma.Config{Name: "Motorola S-Record", Aliases: []string{"srec"}}, marker: 'S'},
		{config: chroma.Config{Name: "Tektronix extended HEX", Aliases: []string{"tehex"}}, marker: '%'},
	} {
		lexers.Register(lexer)
	}
}

func (lexer *hexRecordLexer) Tokenise(options *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	if options != nil && options.EnsureLF {
		value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	}
	var tokens []chroma.Token
	for value != "" {
		end := strings.IndexAny(value, "\r\n")
		if end < 0 {
			return chroma.Literator(lexer.line(tokens, value)...), nil
		}
		tokens = lexer.line(tokens, value[:end])
		newline := end + 1
		if value[end] == '\r' && newline < len(value) && value[newline] == '\n' {
			newline++
		}
		tokens = append(tokens, chroma.Token{Type: chroma.Text, Value: value[end:newline]})
		value = value[newline:]
	}
	return chroma.Literator(tokens...), nil
}

func (lexer *hexRecordLexer) line(tokens []chroma.Token, line string) []chroma.Token {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" || trimmed[0] != lexer.marker && !(lexer.marker == 'S' && trimmed[0] == 's') {
		if line != "" {
			tokens = append(tokens, chroma.Token{Type: chroma.Text, Value: line})
		}
		return tokens
	}
	if leading := len(line) - len(trimmed); leading != 0 {
		tokens = append(tokens, chroma.Token{Type: chroma.Text, Value: line[:leading]})
	}
	line = trimmed
	field := func(start, end int, kind chroma.TokenType, valid bool) {
		end = min(end, len(line))
		// Record fields use ASCII widths. Keep a malformed multibyte
		// character whole in the field where it starts.
		start = hexFieldBoundary(line, start)
		end = hexFieldBoundary(line, end)
		if start >= end {
			return
		}
		if !valid {
			kind = chroma.Error
		}
		tokens = append(tokens, chroma.Token{Type: kind, Value: line[start:end]})
	}
	data := func(start, end int) {
		end = min(end, len(line))
		for at := start; at < end; at += 2 {
			last := min(at+2, end)
			kind := chroma.LiteralString
			if (at-start)%4 == 2 {
				kind = chroma.LiteralNumberHex
			}
			field(at, last, kind, hexDigits(line, at, last))
		}
	}
	field(0, 1, chroma.Keyword, true)
	// Use the declared boundary even when the record has extra characters.
	// For partial input, field clips each token to the bytes actually present.
	switch lexer.marker {
	case ':':
		count, countOK := hexByte(line, 1)
		kind, typeOK := hexByte(line, 7)
		switch kind {
		case 1:
			countOK = countOK && count == 0
		case 2, 4:
			countOK = countOK && count == 2
		case 3, 5:
			countOK = countOK && count == 4
		}
		checksum := 9 + 2*count
		end := checksum + 2
		field(1, 3, chroma.LiteralNumber, countOK && len(line) == end)
		field(3, 7, chroma.NameFunction, hexDigits(line, 3, 7))
		field(7, 9, chroma.NameTag, typeOK && kind <= 5)
		data(9, checksum)
		field(checksum, end, chroma.Comment, hexChecksum(line, end, 0))
		field(end, len(line), chroma.Error, false)
	case 'S':
		addrBytes := 0
		var kind byte
		if len(line) > 1 {
			kind = line[1]
			switch kind {
			case '0', '1', '5', '9':
				addrBytes = 2
			case '2', '6', '8':
				addrBytes = 3
			case '3', '7':
				addrBytes = 4
			}
		}
		count, countOK := hexByte(line, 2)
		if kind >= '5' && kind <= '9' {
			countOK = countOK && count == addrBytes+1
		}
		addressEnd := 4 + 2*addrBytes
		checksum := max(addressEnd, 2+2*count)
		end := checksum + 2
		field(1, 2, chroma.NameTag, addrBytes != 0)
		field(2, 4, chroma.LiteralNumber, countOK && count > addrBytes && len(line) == end)
		field(4, addressEnd, chroma.NameFunction, hexDigits(line, 4, addressEnd))
		data(addressEnd, checksum)
		field(checksum, end, chroma.Comment, hexChecksum(line[1:], end-1, 255))
		field(end, len(line), chroma.Error, false)
	case '%':
		count, countOK := hexByte(line, 1)
		const addressEnd = 15
		end := max(addressEnd, count+1)
		field(1, 3, chroma.LiteralNumber, countOK && count >= addressEnd-1 && len(line) == end)
		field(3, 4, chroma.NameTag, len(line) > 3 && (line[3] == '6' || line[3] == '8'))
		field(4, 6, chroma.Comment, tekChecksum(line, end))
		field(6, addressEnd, chroma.NameFunction, hexDigits(line, 6, addressEnd))
		data(addressEnd, end)
		field(end, len(line), chroma.Error, false)
	}
	return tokens
}

func hexFieldBoundary(line string, at int) int {
	for at < len(line) && !utf8.RuneStart(line[at]) {
		at++
	}
	return at
}

func hexNibble(ch byte) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10
	default:
		return -1
	}
}

func hexDigits(value string, start, end int) bool {
	if end > len(value) {
		return false
	}
	for i := start; i < end; i++ {
		if hexNibble(value[i]) < 0 {
			return false
		}
	}
	return true
}

func hexByte(value string, at int) (int, bool) {
	if !hexDigits(value, at, at+2) {
		return 0, false
	}
	return 16*hexNibble(value[at]) + hexNibble(value[at+1]), true
}

func hexChecksum(value string, end, residue int) bool {
	sum := 0
	for i := 1; i < end; i += 2 {
		val, ok := hexByte(value, i)
		if !ok {
			return false
		}
		sum += val
	}
	return sum&255 == residue
}

func tekChecksum(value string, end int) bool {
	want, ok := hexByte(value, 4)
	if !ok || end > len(value) {
		return false
	}
	sum := 0
	for i := 1; i < end; i++ {
		if i == 4 || i == 5 {
			continue
		}
		part := hexNibble(value[i])
		if part < 0 {
			return false
		}
		sum += part
	}
	return sum&255 == want
}
